package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"aigw-cli/internal/client/claude"
	codexclient "aigw-cli/internal/client/codex"
	"aigw-cli/internal/client/hermes"
	"aigw-cli/tools/release/construction"
)

var fixtureHermesInstaller = []byte("#!/bin/sh\n# official installer fixture\n")

// testInstallerRecord names the file in which the supply interruption test's
// fake installers record their process IDs. A test process started with it
// serves the tool its invocation names.
const testInstallerRecord = "AIGW_CI_TEST_INSTALLER_RECORD"

// serveTestTool runs this test binary as the `ci` command or as a fake
// installer. The fake `mise` starts a fake `npm`, as the real one does; both
// ignore termination requests and block, as a stalled download can, so only
// their owner's forced cleanup stops them.
func serveTestTool(name, record string) int {
	switch name {
	case "ci":
		if err := run(os.Args[1:], os.Stdout, systemRunner); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "mise", "npm":
		signal.Ignore(os.Interrupt, syscall.SIGTERM)
		if name == "mise" {
			if err := exec.Command(filepath.Join(filepath.Dir(os.Args[0]), "npm")).Start(); err != nil {
				return 2
			}
		}
		file, err := os.OpenFile(record, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return 2
		}
		if _, err := fmt.Fprintln(file, os.Getpid()); err != nil || file.Close() != nil {
			return 2
		}
		time.Sleep(time.Minute)
		return 3
	}
	return 2
}

func fixtureRelease() clientRelease {
	return clientRelease{codex: "0.163.0", claude: "2.1.302", hermesCommit: strings.Repeat("b", 40),
		hermesInstaller: fmt.Sprintf("%x", sha256.Sum256(fixtureHermesInstaller))}
}

func fixtureReleaseFor(host string) clientRelease {
	release := fixtureRelease()
	platform, _, _ := strings.Cut(host, "/")
	if !slices.Contains(construction.NativeAcceptanceClients(platform), hermes.ID) {
		release.hermesCommit, release.hermesInstaller = "", ""
	}
	return release
}

func TestPosixClientEnvironmentNamesTheMissingTool(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PATH", root)
	layout := clientLayouts["linux/amd64"]
	if _, err := posixClientEnvironment(root, layout, nil); err == nil || !strings.Contains(err.Error(), "locate Node.js") {
		t.Fatalf("POSIX clients without Node.js = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "node"+executableSuffix()), []byte("inert fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := posixClientEnvironment(root, layout, nil); err == nil || !strings.Contains(err.Error(), "locate Git") {
		t.Fatalf("POSIX clients without Git = %v", err)
	}
}

func TestClientSupplyFollowsTheReleaseOwnersJourneys(t *testing.T) {
	for host, layout := range clientLayouts {
		t.Run(host, func(t *testing.T) {
			root := t.TempDir()
			gitBin, bash, nodeDirectory := windowsClientTools(t, root)
			githubEnvironment := filepath.Join(root, "github-env")
			var calls []command
			var stdout bytes.Buffer
			release := fixtureReleaseFor(host)
			err := supplyClients(host, root, githubEnvironment, release, &stdout, recordingSupply(&calls, layout), nil)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(string(readFile(t, githubEnvironment))), "\n")
			supply, found := strings.CutPrefix(lines[0], "AIGW_NATIVE_CLIENT_SUPPLY=")
			if !found || filepath.Dir(supply) != root || !strings.HasPrefix(filepath.Base(supply), clientSupplyPrefix) {
				t.Fatalf("client supply is not recorded before installation: %q", lines)
			}
			platform, _, _ := strings.Cut(host, "/")
			var want []string
			if platform == "windows" {
				want = expectedWindowsClientEnvironment(supply, gitBin, bash, nodeDirectory)
			} else {
				want = expectedPosixClientEnvironment(supply, layout, gitBin, nodeDirectory)
			}
			if !slices.Equal(lines[1:], want) {
				t.Fatalf("%s client environment = %q, want %q", host, lines[1:], want)
			}
			var supplied []string
			for _, entry := range lines[1:] {
				key, _, _ := strings.Cut(entry, "=")
				if client, found := strings.CutPrefix(key, "AIGW_ACCEPTANCE_"); found && client != "CLIENT_PATH" && client != "HERMES_HOME" {
					supplied = append(supplied, strings.ToLower(client))
				}
			}
			if declared := construction.NativeAcceptanceClients(platform); !slices.Equal(slices.Sorted(slices.Values(supplied)), slices.Sorted(slices.Values(declared))) {
				t.Fatalf("%s supplies %q, but its journey declares %q", host, supplied, declared)
			}
			checkClientIdentities(t, lines[1:])
			install := slices.IndexFunc(calls, func(call command) bool { return slices.Contains(call.Args, "install") })
			if install < 0 || !slices.Equal(calls[install].Args[len(calls[install].Args)-2:], release.packages(layout)) {
				t.Fatalf("%s installed packages = %#v, want %q", host, calls, release.packages(layout))
			}
			if digest := fmt.Sprintf("%x", sha256.Sum256([]byte("codex fixture"))); !strings.Contains(stdout.String(), digest) {
				t.Fatalf("official client digests were not reported: %s", stdout.String())
			}
		})
	}
}

func TestClientSupplyRefusesAnUnsupportedHostBeforeAnyEffect(t *testing.T) {
	for _, host := range []string{"linux/arm64", "freebsd/amd64", "windows/arm64"} {
		root := t.TempDir()
		err := supplyClients(host, root, filepath.Join(root, "github-env"), fixtureRelease(), &bytes.Buffer{}, func(call command) error {
			t.Fatalf("%s reached %s", host, call.Name)
			return nil
		}, nil)
		if err == nil || !strings.Contains(err.Error(), "no official native client distribution for "+host) {
			t.Fatalf("%s supply = %v", host, err)
		}
		if entries, _ := os.ReadDir(root); len(entries) != 0 {
			t.Fatalf("%s supply left %d entries", host, len(entries))
		}
	}
}

func TestClientSupplyRequiresExactReleasePins(t *testing.T) {
	inexact := func(change func(*clientRelease)) clientRelease {
		release := fixtureRelease()
		change(&release)
		return release
	}
	for name, entry := range map[string]struct {
		host    string
		release clientRelease
		problem string
	}{
		"missing Codex":           {"linux/amd64", inexact(func(r *clientRelease) { r.codex = "" }), "exact Codex and Claude releases"},
		"ranged Claude":           {"darwin/arm64", inexact(func(r *clientRelease) { r.claude = "^2.1.302" }), "exact Codex and Claude releases"},
		"missing Hermes commit":   {"linux/amd64", inexact(func(r *clientRelease) { r.hermesCommit = "" }), "exact release commit and installer checksum"},
		"abbreviated commit":      {"darwin/arm64", inexact(func(r *clientRelease) { r.hermesCommit = "bf0203a7" }), "exact release commit and installer checksum"},
		"uppercase checksum":      {"linux/amd64", inexact(func(r *clientRelease) { r.hermesInstaller = strings.ToUpper(r.hermesInstaller) }), "exact release commit and installer checksum"},
		"Hermes outside journeys": {"windows/amd64", fixtureRelease(), "Hermes is not part of this host's native client journeys"},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			err := supplyClients(entry.host, root, filepath.Join(root, "github-env"), entry.release, &bytes.Buffer{}, func(call command) error {
				t.Fatalf("inexact pins reached %s", call.Name)
				return nil
			}, nil)
			if err == nil || !strings.Contains(err.Error(), entry.problem) {
				t.Fatalf("supply with %s = %v, want %q", name, err, entry.problem)
			}
			if entries, _ := os.ReadDir(root); len(entries) != 0 {
				t.Fatalf("refused supply left %d entries", len(entries))
			}
		})
	}
}

func TestHermesSupplyRunsOnlyThePinnedOfficialInstaller(t *testing.T) {
	root := t.TempDir()
	windowsClientTools(t, root)
	// Job settings that the official installer would otherwise follow.
	t.Setenv("HERMES_RUNTIME_DIR", filepath.Join(root, "job", "tools"))
	t.Setenv("HERMES_REPO_URL", "https://example.invalid/fork.git")
	t.Setenv("HTTPS_PROXY", "http://proxy.invalid:3128")
	layout := clientLayouts["linux/amd64"]
	var calls []command
	githubEnvironment := filepath.Join(root, "github-env")
	if err := supplyClients("linux/amd64", root, githubEnvironment, fixtureRelease(), &bytes.Buffer{}, recordingSupply(&calls, layout), nil); err != nil {
		t.Fatal(err)
	}
	supply := strings.TrimPrefix(strings.Split(string(readFile(t, githubEnvironment)), "\n")[0], "AIGW_NATIVE_CLIENT_SUPPLY=")
	release, home := fixtureRelease(), filepath.Join(supply, "home")
	installer := filepath.Join(supply, "hermes-install.sh")
	download := slices.IndexFunc(calls, func(call command) bool { return call.Name == "curl" })
	install := slices.IndexFunc(calls, func(call command) bool { return call.Name == "bash" })
	if download < 0 || install <= download {
		t.Fatalf("Hermes installer was not downloaded before installation: %#v", calls)
	}
	if got, want := calls[download].Args[len(calls[download].Args)-1], "https://raw.githubusercontent.com/NousResearch/hermes-agent/"+release.hermesCommit+"/scripts/install.sh"; got != want {
		t.Fatalf("Hermes installer source = %q, want %q", got, want)
	}
	wantArguments := []string{installer, "--commit", release.hermesCommit, "--dir", filepath.Join(supply, "hermes", "hermes-agent"), "--hermes-home", filepath.Join(supply, "hermes"),
		"--non-interactive", "--skip-browser", "--skip-computer-use"}
	if !slices.Equal(calls[install].Args, wantArguments) {
		t.Fatalf("Hermes installation = %q, want %q", calls[install].Args, wantArguments)
	}
	for _, key := range []string{"HOME", "TMPDIR", "GIT_CONFIG_GLOBAL", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_BIN_HOME", "HERMES_RUNTIME_DIR"} {
		index := slices.IndexFunc(calls[install].Env, func(entry string) bool { return strings.HasPrefix(entry, key+"=") })
		if index < 0 || !strings.HasPrefix(strings.TrimPrefix(calls[install].Env[index], key+"="), supply+string(filepath.Separator)) {
			t.Fatalf("Hermes installation %s escapes the supply: %q", key, calls[install].Env)
		}
	}
	if !slices.Contains(calls[install].Env, "HOME="+home) || !slices.Contains(calls[install].Env, "HERMES_RUNTIME_DIR="+filepath.Join(supply, "hermes", "tools")) {
		t.Fatalf("Hermes installation home and tool store = %q", calls[install].Env)
	}
	// The installer receives exactly its private settings and the host's tool
	// path and network settings; no other job setting reaches it.
	private := []string{"HOME", "TMPDIR", "GIT_CONFIG_GLOBAL", "GIT_TERMINAL_PROMPT", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_BIN_HOME", "HERMES_RUNTIME_DIR"}
	var keys []string
	for _, entry := range calls[install].Env {
		key, _, _ := strings.Cut(entry, "=")
		if slices.Contains(keys, key) || (!slices.Contains(private, key) && !slices.Contains(installerHostSettings, key)) {
			t.Fatalf("Hermes installation receives job setting %s: %q", key, calls[install].Env)
		}
		keys = append(keys, key)
	}
	if !calls[install].Isolated || !slices.Contains(calls[install].Env, "PATH="+os.Getenv("PATH")) || !slices.Contains(calls[install].Env, "HTTPS_PROXY=http://proxy.invalid:3128") {
		t.Fatalf("Hermes installation host settings = isolated %v, %q", calls[install].Isolated, calls[install].Env)
	}
	verified := 0
	for _, item := range calls[install+1:] {
		if item.Name == "git" && slices.Contains(item.Args, filepath.Join(supply, "hermes", "hermes-agent")) && item.Isolated && slices.Equal(item.Env, calls[install].Env) {
			verified++
		}
	}
	if verified != 3 {
		t.Fatalf("Hermes checkout identity checks = %d, want 3: %#v", verified, calls[install+1:])
	}
}

func TestHermesSupplyRetiresEveryUnverifiedInstallation(t *testing.T) {
	named := func(name string, subcommand ...string) func(command) bool {
		return func(call command) bool {
			return call.Name == name && (len(subcommand) == 0 || slices.Contains(call.Args, subcommand[0]))
		}
	}
	fails := func(message string) func(command) error {
		return func(command) error { return errors.New(message) }
	}
	for name, entry := range map[string]struct {
		selects func(command) bool
		result  func(command) error
		problem string
	}{
		"tampered installer": {named("curl"), func(call command) error {
			return os.WriteFile(call.Args[slices.Index(call.Args, "--output")+1], []byte("tampered installer"), 0o600)
		}, "official Hermes installer checksum mismatch"},
		"failed download":     {named("curl"), fails("curl exited with status 22"), "download the official Hermes installer"},
		"failed installation": {named("bash"), fails("bash exited with status 1"), "install the official Hermes release"},
		"different checkout":  {named("git", "merge-base"), fails("git exited with status 1"), "not the unmodified pinned release"},
		"modified checkout":   {named("git", "diff"), fails("git exited with status 1"), "not the unmodified pinned release"},
		"missing launcher": {named("bash"), func(call command) error {
			return os.MkdirAll(call.Args[slices.Index(call.Args, "--dir")+1], 0o700)
		}, "published no launcher"},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			windowsClientTools(t, root)
			layout := clientLayouts["linux/amd64"]
			var calls []command
			supply := recordingSupply(&calls, layout)
			githubEnvironment := filepath.Join(root, "github-env")
			err := supplyClients("linux/amd64", root, githubEnvironment, fixtureRelease(), &bytes.Buffer{}, func(call command) error {
				if entry.selects(call) {
					return entry.result(call)
				}
				return supply(call)
			}, nil)
			if err == nil || !strings.Contains(err.Error(), entry.problem) {
				t.Fatalf("unverified Hermes supply = %v, want %q", err, entry.problem)
			}
			if lines := strings.Split(strings.TrimSpace(string(readFile(t, githubEnvironment))), "\n"); len(lines) != 1 {
				t.Fatalf("unverified Hermes supply exported %q", lines)
			}
			if left, _ := filepath.Glob(filepath.Join(root, clientSupplyPrefix+"*")); len(left) != 0 {
				t.Fatalf("unverified Hermes supply survived: %q", left)
			}
		})
	}
}

func TestClientSupplyRetiresAnInterruptedSupply(t *testing.T) {
	for name, during := range map[string]func(command) bool{
		"package installation": func(call command) bool { return slices.Contains(call.Args, "install") },
		"last identity check":  func(call command) bool { return call.Name == "git" && slices.Contains(call.Args, "diff") },
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			windowsClientTools(t, root)
			layout := clientLayouts["linux/amd64"]
			interrupted := make(chan os.Signal, 1)
			var calls []command
			supply := recordingSupply(&calls, layout)
			githubEnvironment := filepath.Join(root, "github-env")
			err := supplyClients("linux/amd64", root, githubEnvironment, fixtureRelease(), &bytes.Buffer{}, func(call command) error {
				if during(call) {
					interrupted <- os.Interrupt
				}
				return supply(call)
			}, interrupted)
			if err == nil || !strings.Contains(err.Error(), "native client supply was interrupted") {
				t.Fatalf("interrupted supply = %v", err)
			}
			if after := slices.IndexFunc(calls, during); after < 0 || after != len(calls)-1 {
				t.Fatalf("interrupted supply continued after the signal: %#v", calls)
			}
			if lines := strings.Split(strings.TrimSpace(string(readFile(t, githubEnvironment))), "\n"); len(lines) != 1 {
				t.Fatalf("interrupted supply exported %q", lines)
			}
			if left, _ := filepath.Glob(filepath.Join(root, clientSupplyPrefix+"*")); len(left) != 0 {
				t.Fatalf("interrupted supply survived: %q", left)
			}
		})
	}
}

func TestNativeClientsCommandSuppliesAndRetiresThisHost(t *testing.T) {
	host := runtime.GOOS + "/" + runtime.GOARCH
	layout, supported := clientLayouts[host]
	if !supported {
		t.Fatalf("native client acceptance has no official layout for test host %s", host)
	}
	root := t.TempDir()
	windowsClientTools(t, root)
	githubEnvironment := filepath.Join(root, "github-env")
	release := fixtureReleaseFor(host)
	arguments := []string{"native-clients", "supply", "--root", root, "--github-env", githubEnvironment, "--codex", release.codex, "--claude", release.claude}
	if release.hermesCommit != "" {
		arguments = append(arguments, "--hermes-commit", release.hermesCommit, "--hermes-installer-sha256", release.hermesInstaller)
	}
	var calls []command
	if err := run(arguments, &bytes.Buffer{}, recordingSupply(&calls, layout)); err != nil {
		t.Fatal(err)
	}
	clients := strings.TrimPrefix(strings.Split(string(readFile(t, githubEnvironment)), "\n")[0], "AIGW_NATIVE_CLIENT_SUPPLY=")
	t.Setenv("AIGW_NATIVE_CLIENT_SUPPLY", clients)
	if err := run([]string{"native-clients", "retire"}, &bytes.Buffer{}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(clients); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("client supply survived retirement: %v", err)
	}
}

func TestNativeClientSupplyRefusesAClientWithoutAnOfficialDistribution(t *testing.T) {
	err := fixtureRelease().validate([]string{claude.ID, codexclient.ID, hermes.ID, "cursor"})
	if err == nil || !strings.Contains(err.Error(), "no official supply for native client cursor") {
		t.Fatalf("supply of a client without an official distribution = %v", err)
	}
}

// The supply creates the private Hermes homes itself, so an entry that already
// occupies one stops the installation before any command runs.
func TestHermesSupplyRefusesAnExistingPrivateHome(t *testing.T) {
	supply := t.TempDir()
	if err := os.WriteFile(filepath.Join(supply, "home"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	var calls []command
	err := installHermes(supply, fixtureRelease(), &bytes.Buffer{}, func(call command) error {
		calls = append(calls, call)
		return nil
	})
	if !errors.Is(err, os.ErrExist) || len(calls) != 0 {
		t.Fatalf("Hermes supply over an existing home = %v after %d commands", err, len(calls))
	}
}

func recordingSupply(calls *[]command, layout clientLayout) commandRunner {
	return func(call command) error {
		*calls = append(*calls, call)
		switch {
		case call.Name == "mise" && slices.Contains(call.Args, "install"):
			prefix := call.Args[slices.Index(call.Args, "--prefix")+1]
			for path, data := range map[string]string{layout.codexRoot() + "/bin/codex" + layout.suffix: "codex fixture", layout.claudeExecutable(): "claude fixture"} {
				if err := writeFixture(filepath.Join(prefix, filepath.FromSlash(path)), data); err != nil {
					return err
				}
			}
		case call.Name == "curl":
			return os.WriteFile(call.Args[slices.Index(call.Args, "--output")+1], fixtureHermesInstaller, 0o600)
		case call.Name == "bash":
			checkout := call.Args[slices.Index(call.Args, "--dir")+1]
			if err := os.MkdirAll(checkout, 0o700); err != nil {
				return err
			}
			home := strings.TrimPrefix(call.Env[slices.IndexFunc(call.Env, func(entry string) bool { return strings.HasPrefix(entry, "HOME=") })], "HOME=")
			return writeFixture(filepath.Join(home, ".local", "bin", "hermes"), "hermes fixture")
		}
		return nil
	}
}

func writeFixture(path, data string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(data), 0o700)
}

func expectedPosixClientEnvironment(supply string, layout clientLayout, gitBin, nodeDirectory string) []string {
	codex := filepath.Join(supply, filepath.FromSlash(layout.codexRoot()))
	return []string{
		"AIGW_ACCEPTANCE_CODEX=" + filepath.Join(codex, "bin", "codex"),
		"AIGW_ACCEPTANCE_CLAUDE=" + filepath.Join(supply, filepath.FromSlash(layout.claudeExecutable())),
		"AIGW_ACCEPTANCE_HERMES=" + filepath.Join(supply, "home", ".local", "bin", "hermes"),
		"AIGW_ACCEPTANCE_HERMES_HOME=" + filepath.Join(supply, "hermes"),
		"AIGW_ACCEPTANCE_CLIENT_PATH=" + strings.Join([]string{filepath.Join(codex, "codex-path"), nodeDirectory, gitBin, "/usr/bin", "/bin"}, string(os.PathListSeparator)),
	}
}
