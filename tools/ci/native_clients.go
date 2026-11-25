package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	claudeclient "aigw-cli/internal/adapter/claude"
	codexclient "aigw-cli/internal/adapter/codex"
	hermesclient "aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/platform"
	"aigw-cli/tools/release/construction"

	"github.com/rogpeppe/go-internal/robustio"
)

// Official native client distributions that real-client acceptance runs. The
// pipeline model pins their releases once; this owner knows where each host's
// official packages place their executables, and the release owner declares
// which clients each host's journey runs.
const (
	clientSupplyPrefix = "aigw-native-clients-"
	hermesInstallerURL = "https://raw.githubusercontent.com/NousResearch/hermes-agent/%s/scripts/install.sh"
)

// clientLayout names one host's official npm platform packages: their platform
// suffix and the Codex vendor target.
type clientLayout struct{ npm, target string }

var (
	clientLayouts = map[string]clientLayout{
		"darwin/arm64": {npm: "darwin-arm64", target: "aarch64-apple-darwin"},
		"linux/amd64":  {npm: "linux-x64", target: "x86_64-unknown-linux-musl"},
	}
	exactSHA256  = regexp.MustCompile(`^[0-9a-f]{64}$`)
	exactCommit  = regexp.MustCompile(`^[0-9a-f]{40}$`)
	exactRelease = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	// installerHostSettings are the only host settings that the official
	// Hermes installer receives: where the host's tools are and how the host
	// reaches the network.
	installerHostSettings = []string{
		"PATH", "HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy", "NO_PROXY", "no_proxy", "SSL_CERT_FILE", "SSL_CERT_DIR",
	}
)

func (layout clientLayout) codexRoot() string {
	return "node_modules/@openai/codex-" + layout.npm + "/vendor/" + layout.target
}

func (layout clientLayout) claudeExecutable() string {
	return "node_modules/@anthropic-ai/claude-code-" + layout.npm + "/claude"
}

// clientRelease pins the official client releases that one supply installs.
type clientRelease struct {
	codex, claude                 string
	hermesCommit, hermesInstaller string
}

// packages names the official npm packages that carry this host's native
// Codex and Claude executables.
func (release clientRelease) packages(layout clientLayout) []string {
	return []string{"@openai/codex@" + release.codex, "@anthropic-ai/claude-code-" + layout.npm + "@" + release.claude}
}

// validate admits exactly the pins that clients need.
func (release clientRelease) validate(clients []string) error {
	if !exactRelease.MatchString(release.codex) || !exactRelease.MatchString(release.claude) {
		return errors.New("native client supply requires exact Codex and Claude releases")
	}
	for _, client := range clients {
		if client != claudeclient.ID && client != codexclient.ID && client != hermesclient.ID {
			return fmt.Errorf("no official supply for native client %s", client)
		}
	}
	pinned := release.hermesCommit != "" || release.hermesInstaller != ""
	switch hermes := slices.Contains(clients, hermesclient.ID); {
	case hermes && (!exactCommit.MatchString(release.hermesCommit) || !exactSHA256.MatchString(release.hermesInstaller)):
		return errors.New("native Hermes supply requires its exact release commit and installer checksum")
	case !hermes && pinned:
		return errors.New("Hermes is not part of this host's native client journeys")
	}
	return nil
}

// clientEnvironment names the official clients under supply and the
// companion tools they run, and returns the acceptance environment.
func clientEnvironment(supply string, layout clientLayout, clients []string) ([]string, error) {
	node, err := exec.LookPath("node")
	if err != nil {
		return nil, fmt.Errorf("locate Node.js: %w", err)
	}
	git, err := exec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("locate Git: %w", err)
	}
	codex := filepath.Join(supply, filepath.FromSlash(layout.codexRoot()))
	environment := []string{
		"AIGW_ACCEPTANCE_CODEX=" + filepath.Join(codex, "bin", "codex"),
		"AIGW_ACCEPTANCE_CLAUDE=" + filepath.Join(supply, filepath.FromSlash(layout.claudeExecutable())),
	}
	if slices.Contains(clients, hermesclient.ID) {
		environment = append(environment,
			"AIGW_ACCEPTANCE_HERMES="+hermesLauncher(supply),
			"AIGW_ACCEPTANCE_HERMES_HOME="+filepath.Join(supply, "hermes"))
	}
	var search []string
	for _, directory := range []string{filepath.Join(codex, "codex-path"), filepath.Dir(node), filepath.Dir(git), "/usr/bin", "/bin"} {
		if !slices.Contains(search, directory) {
			search = append(search, directory)
		}
	}
	return append(environment, "AIGW_ACCEPTANCE_CLIENT_PATH="+strings.Join(search, string(os.PathListSeparator))), nil
}

// hermesLauncher is the command the official Hermes installer publishes under
// the supply's private home.
func hermesLauncher(supply string) string {
	return filepath.Join(supply, "home", ".local", "bin", "hermes")
}

// runNativeClients supplies or retires the official clients that a GitHub
// native job installs for real-client acceptance on its own host.
func runNativeClients(arguments []string, stdout io.Writer, runner commandRunner) error {
	const usage = "usage: ci native-clients supply --root <dir> --github-env <file> --codex <version> --claude <version> " +
		"[--hermes-commit <sha> --hermes-installer-sha256 <sha256>] | ci native-clients retire"
	if slices.Equal(arguments, []string{"retire"}) {
		return retireClients(os.Getenv("AIGW_NATIVE_CLIENT_SUPPLY"), systemOutputRunner)
	}
	if len(arguments) == 0 || arguments[0] != "supply" {
		return errors.New(usage)
	}
	flags := flag.NewFlagSet("ci native-clients supply", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	root := flags.String("root", "", "directory that holds the private client supply")
	githubEnvironment := flags.String("github-env", "", "GitHub environment file")
	var release clientRelease
	flags.StringVar(&release.codex, "codex", "", "official Codex release")
	flags.StringVar(&release.claude, "claude", "", "official Claude Code release")
	flags.StringVar(&release.hermesCommit, "hermes-commit", "", "official Hermes release commit")
	flags.StringVar(&release.hermesInstaller, "hermes-installer-sha256", "", "SHA-256 of that release's installer")
	if err := flags.Parse(arguments[1:]); err != nil || flags.NArg() != 0 || *root == "" || *githubEnvironment == "" {
		return errors.New(usage)
	}
	// An interrupted job still retires its partial supply: the process owner
	// stops the running installer and every process it started, and this
	// process outlives them to remove the supply.
	interrupted := make(chan os.Signal, 1)
	signal.Notify(interrupted, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(interrupted)
	return supplyClients(runtime.GOOS+"/"+runtime.GOARCH, *root, *githubEnvironment, release, stdout, runner, interrupted)
}

// supplyClients installs the official clients that host's journey runs into a
// private directory under root and records their acceptance environment. The
// directory is recorded first, so the cleanup step retires it after any later
// failure; a failed or interrupted supply also removes it before returning.
func supplyClients(host, root, githubEnvironment string, release clientRelease, stdout io.Writer, runner commandRunner, interrupted <-chan os.Signal) (err error) {
	layout, found := clientLayouts[host]
	system, _, _ := strings.Cut(host, "/")
	clients := construction.NativeAcceptanceClients(system)
	if !found || len(clients) == 0 {
		return fmt.Errorf("no official native client distribution for %s", host)
	}
	if err := release.validate(clients); err != nil {
		return err
	}
	supply, err := os.MkdirTemp(root, clientSupplyPrefix)
	if err != nil {
		return err
	}
	// A failure names a signal that stopped the running step before the
	// step's own error.
	defer func() {
		if err != nil {
			err = errors.Join(interruption(interrupted), err, robustio.RemoveAll(supply))
		}
	}()
	if err := appendGitHubEnvironment(githubEnvironment, "AIGW_NATIVE_CLIENT_SUPPLY="+supply); err != nil {
		return err
	}
	// A signal stops the supply before its next step; the final check below
	// covers a signal that arrives during the last one.
	guarded := func(call command) error {
		if err := interruption(interrupted); err != nil {
			return err
		}
		return runner(call)
	}
	if err := os.WriteFile(filepath.Join(supply, "package.json"), []byte("{\"private\":true}\n"), 0o600); err != nil {
		return err
	}
	gitConfig := filepath.Join(supply, "gitconfig")
	isolated := []string{"GIT_CONFIG_GLOBAL=" + gitConfig}
	if err := runCommands([]command{
		{Name: "git", Args: []string{"config", "--file", gitConfig, "core.autocrlf", "false"}},
		{Name: "mise", Args: append([]string{"exec", "--locked", "--", "npm", "install", "--prefix", supply, "--ignore-scripts", "--save-exact", "--no-audit", "--no-fund"}, release.packages(layout)...), Env: isolated},
		{Name: "mise", Args: []string{"exec", "--locked", "--", "npm", "audit", "signatures", "--prefix", supply}, Env: isolated},
	}, stdout, guarded); err != nil {
		return err
	}
	environment, err := clientEnvironment(supply, layout, clients)
	if err != nil {
		return err
	}
	if slices.Contains(clients, hermesclient.ID) {
		if err := installHermes(supply, release, stdout, guarded); err != nil {
			return err
		}
	}
	for _, entry := range environment {
		key, executable, _ := strings.Cut(entry, "=")
		if !slices.Contains([]string{"AIGW_ACCEPTANCE_CODEX", "AIGW_ACCEPTANCE_CLAUDE", "AIGW_ACCEPTANCE_HERMES"}, key) {
			continue
		}
		digest, err := platform.FileSHA256(executable)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(stdout, "Official %s client %s sha256=%s\n", host, executable, digest); err != nil {
			return err
		}
	}
	if err := interruption(interrupted); err != nil {
		return err
	}
	return appendGitHubEnvironment(githubEnvironment, environment...)
}

// installHermes installs the pinned official Hermes release through that
// release's own installer. Its homes, caches, temporary files and published
// launcher stay inside the supply, and the result must be the pinned checkout.
func installHermes(supply string, release clientRelease, stdout io.Writer, runner commandRunner) error {
	installer, home, hermesHome := filepath.Join(supply, "hermes-install.sh"), filepath.Join(supply, "home"), filepath.Join(supply, "hermes")
	checkout, temporary := filepath.Join(hermesHome, "hermes-agent"), filepath.Join(supply, "tmp")
	for _, directory := range []string{home, temporary} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			return err
		}
	}
	download := command{Name: "curl", Args: []string{"--fail", "--silent", "--show-error", "--location", "--proto", "=https", "--max-time", "120",
		"--output", installer, fmt.Sprintf(hermesInstallerURL, release.hermesCommit)}}
	if err := runner(download); err != nil {
		return fmt.Errorf("download the official Hermes installer: %w", err)
	}
	if digest, err := platform.FileSHA256(installer); err != nil || digest != release.hermesInstaller {
		return errors.Join(errors.New("official Hermes installer checksum mismatch"), err)
	}
	// The installer receives a declared environment rather than the job's, so
	// a job setting such as HERMES_RUNTIME_DIR cannot move its state outside
	// the supply. Its tool store is bound to the installer's own default place.
	private := []string{
		"HOME=" + home, "TMPDIR=" + temporary, "GIT_CONFIG_GLOBAL=" + filepath.Join(supply, "gitconfig"), "GIT_TERMINAL_PROMPT=0",
		"XDG_CONFIG_HOME=" + filepath.Join(home, ".config"), "XDG_CACHE_HOME=" + filepath.Join(home, ".cache"),
		"XDG_DATA_HOME=" + filepath.Join(home, ".local", "share"), "XDG_STATE_HOME=" + filepath.Join(home, ".local", "state"),
		"XDG_BIN_HOME=" + filepath.Join(home, ".local", "bin"), "HERMES_RUNTIME_DIR=" + filepath.Join(hermesHome, "tools"),
	}
	for _, name := range installerHostSettings {
		if value, found := os.LookupEnv(name); found {
			private = append(private, name+"="+value)
		}
	}
	install := command{Name: "bash", Args: []string{installer, "--commit", release.hermesCommit, "--dir", checkout, "--hermes-home", hermesHome,
		"--non-interactive", "--skip-browser", "--skip-computer-use"}, Env: private, Isolated: true}
	if err := runCommands([]command{install}, stdout, runner); err != nil {
		return fmt.Errorf("install the official Hermes release: %w", err)
	}
	// Mutual ancestry proves the checkout is the pinned commit; a clean diff
	// proves the installer left the vendor source unmodified.
	for _, arguments := range [][]string{
		{"-C", checkout, "merge-base", "--is-ancestor", "HEAD", release.hermesCommit},
		{"-C", checkout, "merge-base", "--is-ancestor", release.hermesCommit, "HEAD"},
		{"-C", checkout, "diff", "--quiet", "HEAD", "--"},
	} {
		if err := runner(command{Name: "git", Args: arguments, Env: private, Isolated: true}); err != nil {
			return fmt.Errorf("official Hermes checkout is not the unmodified pinned release: %w", err)
		}
	}
	if !regularFile(hermesLauncher(supply)) {
		return errors.New("official Hermes installer published no launcher")
	}
	_, err := fmt.Fprintf(stdout, "Official Hermes release %s installed at %s\n", release.hermesCommit, checkout)
	return err
}

// interruption reports a signal that asked the supply to stop.
func interruption(interrupted <-chan os.Signal) error {
	select {
	case received := <-interrupted:
		return fmt.Errorf("native client supply was interrupted by %v", received)
	default:
		return nil
	}
}

// retireClients removes the client directory that supply recorded. It refuses
// any path it did not create, and it keeps a supply that a live process still
// uses, so a client that outlived its journey fails the job.
func retireClients(owned string, inventory outputRunner) error {
	if owned == "" {
		return nil
	}
	if !filepath.IsAbs(owned) || !strings.HasPrefix(filepath.Base(owned), clientSupplyPrefix) {
		return fmt.Errorf("refuse to remove an unowned native client supply: %s", owned)
	}
	if err := supplyInUse(owned, inventory); err != nil {
		return err
	}
	if err := robustio.RemoveAll(owned); err != nil {
		return fmt.Errorf("remove the native client supply: %w", err)
	}
	if _, err := os.Lstat(owned); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("native client supply remains after cleanup: %s", owned)
	}
	return nil
}

// supplyInUse refuses a supply that a live process still uses: a client, or a
// process that a client started, names a path inside the supply in its command
// line.
func supplyInUse(owned string, inventory outputRunner) error {
	users, err := supplyProcesses(owned, inventory)
	if err != nil {
		return fmt.Errorf("prove that no process uses the native client supply: %w", err)
	}
	if len(users) != 0 {
		return fmt.Errorf("native client supply is still in use by process %s; retained %s", strings.Join(users, ", "), owned)
	}
	return nil
}

// supplyProcesses returns the IDs of live processes whose command line names a
// path inside the supply. Only IDs are reported; command lines stay private.
func supplyProcesses(owned string, inventory outputRunner) ([]string, error) {
	output, err := inventory(command{Name: "ps", Args: []string{"-A", "-ww", "-o", "pid=", "-o", "args="}, Env: []string{"LC_ALL=C"}, Timeout: 30 * time.Second})
	if err != nil {
		return nil, err
	}
	self := strconv.Itoa(os.Getpid())
	var users []string
	for line := range strings.Lines(string(output)) {
		pid, arguments, _ := strings.Cut(strings.TrimSpace(line), " ")
		if pid != self && strings.Contains(arguments, owned+string(filepath.Separator)) {
			users = append(users, pid)
		}
	}
	return users, nil
}

func appendGitHubEnvironment(path string, entries ...string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open GitHub environment: %w", err)
	}
	_, err = io.WriteString(file, strings.Join(entries, "\n")+"\n")
	return errors.Join(err, file.Close())
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return path != "" && err == nil && info.Mode().IsRegular()
}
