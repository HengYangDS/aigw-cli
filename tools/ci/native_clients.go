package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/rogpeppe/go-internal/robustio"
)

// Official Windows client packages that native acceptance runs. The pipeline
// model selects their versions where it installs or packages them.
const (
	windowsCodexRoot          = "node_modules/@openai/codex-win32-x64/vendor/x86_64-pc-windows-msvc"
	windowsClaudeExecutable   = "node_modules/@anthropic-ai/claude-code-win32-x64/claude.exe"
	windowsClientSupplyPrefix = "aigw-native-clients-"
)

var (
	exactSHA256 = regexp.MustCompile(`^[0-9a-f]{64}$`)
	exactCommit = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

// supplyWindowsInput downloads the selected public input package into the
// job-owned directory, verifies its exact checksum and extracts the reviewed
// official Windows clients. Release acceptance rechecks the same archive.
func supplyWindowsInput(getenv func(string) string, runner commandRunner) ([]string, error) {
	if getenv("AIGW_NATIVE_PLATFORM") != "windows" {
		return nil, errors.New("native public input requires the Windows platform")
	}
	if !exactSHA256.MatchString(getenv("AIGW_NATIVE_INPUT_SHA256")) || !exactCommit.MatchString(getenv("AIGW_CANDIDATE_SOURCE")) {
		return nil, errors.New("native public input requires exact source and checksum")
	}
	if !regularFile(getenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE")) || !regularFile(getenv("AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE")) ||
		strings.TrimSpace(getenv("AIGW_RELEASE_ARTIFACT_SIGNER")) == "" {
		return nil, errors.New("native public input requires configured source and artifact trust")
	}
	supplier := getenv("AIGW_NATIVE_CLIENT_SUPPLIER")
	if supplier == "" {
		return nil, errors.New("native public input requires its declared client supplier")
	}
	archive, extracted := windowsJobPath(getenv, "native-input.tar"), windowsJobPath(getenv, "native-input")
	download := command{
		Name: "mise",
		Args: []string{"exec", "--locked", "--", "glab", "packages", "download", "--repo", getenv("CI_PROJECT_URL"), "--name", getenv("AIGW_NATIVE_INPUT_PACKAGE"),
			"--version", getenv("AIGW_CANDIDATE_SOURCE"), "--filename", "public-inputs.tar", "--path", archive},
		Env: []string{"GLAB_ENABLE_CI_AUTOLOGIN=true", "GLAB_CONFIG_DIR=" + windowsJobPath(getenv, "glab")},
	}
	if err := runner(download); err != nil {
		return nil, fmt.Errorf("download native public input: %w", err)
	}
	if digest, err := fileSHA256(archive); err != nil || digest != getenv("AIGW_NATIVE_INPUT_SHA256") {
		return nil, errors.Join(errors.New("native public input checksum mismatch"), err)
	}
	if err := os.Mkdir(extracted, 0o700); err != nil {
		return nil, err
	}
	for _, arguments := range [][]string{
		{"-xf", archive, "-C", extracted, supplier},
		{"-xf", filepath.Join(extracted, filepath.FromSlash(supplier)), "-C", extracted},
	} {
		if err := runner(command{Name: "tar", Args: arguments}); err != nil {
			return nil, fmt.Errorf("extract official Windows clients: %w", err)
		}
	}
	return windowsClientEnvironment(filepath.Join(extracted, "clients"), "native Claude requires Git Bash")
}

// windowsClientEnvironment locates the official clients under root and the Git
// Bash that Claude requires, and returns the acceptance environment.
func windowsClientEnvironment(root, missingBash string) ([]string, error) {
	git, err := exec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("locate Git: %w", err)
	}
	node, err := exec.LookPath("node")
	if err != nil {
		return nil, fmt.Errorf("locate Node.js: %w", err)
	}
	gitBin := filepath.Dir(git)
	bash := filepath.Join(filepath.Dir(gitBin), "bin", "bash.exe")
	if !regularFile(bash) {
		return nil, errors.New(missingBash)
	}
	codex := filepath.Join(root, filepath.FromSlash(windowsCodexRoot))
	search := []string{filepath.Join(codex, "codex-path"), filepath.Dir(node), gitBin, filepath.Dir(bash), filepath.Join(os.Getenv("SystemRoot"), "System32")}
	return []string{
		"AIGW_ACCEPTANCE_CODEX=" + filepath.Join(codex, "bin", "codex.exe"),
		"AIGW_ACCEPTANCE_CLAUDE=" + filepath.Join(root, filepath.FromSlash(windowsClaudeExecutable)),
		"AIGW_ACCEPTANCE_CLIENT_PATH=" + strings.Join(search, ";"),
		"CLAUDE_CODE_GIT_BASH_PATH=" + bash,
	}, nil
}

// runNativeClients supplies or retires the official Windows clients that the
// GitHub native job installs for real-client acceptance.
func runNativeClients(arguments []string, stdout io.Writer, runner commandRunner) error {
	const usage = "usage: ci native-clients supply --root <dir> --github-env <file> --package <spec>... | ci native-clients retire"
	if slices.Equal(arguments, []string{"retire"}) {
		return retireWindowsClients(os.Getenv("AIGW_NATIVE_CLIENT_SUPPLY"))
	}
	if len(arguments) == 0 || arguments[0] != "supply" {
		return errors.New(usage)
	}
	flags := flag.NewFlagSet("ci native-clients supply", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	root := flags.String("root", "", "directory that holds the private client supply")
	githubEnvironment := flags.String("github-env", "", "GitHub environment file")
	var packages repeatedFlag
	flags.Var(&packages, "package", "official client package specification")
	if err := flags.Parse(arguments[1:]); err != nil || flags.NArg() != 0 || *root == "" || *githubEnvironment == "" || len(packages) == 0 {
		return errors.New(usage)
	}
	return supplyWindowsClients(*root, *githubEnvironment, packages, stdout, runner)
}

// supplyWindowsClients installs the selected official Windows clients into a
// private directory under root and records their acceptance environment. The
// directory is recorded first, so the cleanup step retires it after any later
// failure.
func supplyWindowsClients(root, githubEnvironment string, packages []string, stdout io.Writer, runner commandRunner) (err error) {
	clients, err := os.MkdirTemp(root, windowsClientSupplyPrefix)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, robustio.RemoveAll(clients))
		}
	}()
	if err := appendGitHubEnvironment(githubEnvironment, "AIGW_NATIVE_CLIENT_SUPPLY="+clients); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(clients, "package.json"), []byte("{\"private\":true}\n"), 0o600); err != nil {
		return err
	}
	gitConfig := filepath.Join(clients, "gitconfig")
	isolated := []string{"GIT_CONFIG_GLOBAL=" + gitConfig}
	if err := runCommands([]command{
		{Name: "git", Args: []string{"config", "--file", gitConfig, "core.autocrlf", "false"}},
		{Name: "mise", Args: append([]string{"exec", "--locked", "--", "npm", "install", "--prefix", clients, "--ignore-scripts", "--save-exact", "--no-audit", "--no-fund"}, packages...), Env: isolated},
		{Name: "mise", Args: []string{"exec", "--locked", "--", "npm", "audit", "signatures", "--prefix", clients}, Env: isolated},
	}, stdout, runner); err != nil {
		return err
	}
	environment, err := windowsClientEnvironment(clients, "Claude requires native Git Bash")
	if err != nil {
		return err
	}
	for _, entry := range environment[:2] {
		_, executable, _ := strings.Cut(entry, "=")
		digest, err := fileSHA256(executable)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(stdout, "Official Windows client %s sha256=%s\n", executable, digest); err != nil {
			return err
		}
	}
	return appendGitHubEnvironment(githubEnvironment, environment...)
}

// retireWindowsClients removes the client directory that supply recorded and
// refuses any path it did not create.
func retireWindowsClients(owned string) error {
	if owned == "" {
		return nil
	}
	if !filepath.IsAbs(owned) || !strings.HasPrefix(filepath.Base(owned), windowsClientSupplyPrefix) {
		return fmt.Errorf("refuse to remove an unowned Windows client supply: %s", owned)
	}
	if err := robustio.RemoveAll(owned); err != nil {
		return fmt.Errorf("remove the Windows client supply: %w", err)
	}
	if _, err := os.Lstat(owned); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("Windows native supply remains after cleanup: %s", owned)
	}
	return nil
}

func appendGitHubEnvironment(path string, entries ...string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open GitHub environment: %w", err)
	}
	_, err = io.WriteString(file, strings.Join(entries, "\n")+"\n")
	return errors.Join(err, file.Close())
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	digest := sha256.New()
	_, err = io.Copy(digest, file)
	return hex.EncodeToString(digest.Sum(nil)), errors.Join(err, file.Close())
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return path != "" && err == nil && info.Mode().IsRegular()
}
