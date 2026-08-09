package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestSourceRunsThePortableGateSequence(t *testing.T) {
	want := [][]string{
		{"go", "run", "./tools/cicontract", "toolchain", "."},
		{"go", "run", "./tools/releasekit", "validate-toolchain", "go.mod"},
		{"go", "run", "./tools/releasekit", "validate-release-sources"},
		{"go", "run", "./tools/architecture", "--root", "."},
		{"go", "test", "./tools/architecture"},
		{"go", "run", "./tools/coveragegate", "--race"},
		{"go", "vet", "./..."},
		{"go", "tool", "staticcheck", "-checks=all,-ST1000,-ST1005", "./..."},
		{"go", "tool", "errcheck", "./..."},
		{"go", "run", "./tools/repositorycheck", "--root", ".", "go-format"},
		{"go", "run", "./tools/repositorycheck", "--root", ".", "product-surface"},
		{"go", "run", "./tools/repositorycheck", "--root", ".", "english-text"},
		{"go", "run", "./tools/repositorycheck", "--root", ".", "credentials"},
		{"go", "test", "./tools/repositorycheck"},
		{"go", "run", "./tools/repositorycheck", "--root", ".", "governance"},
		{"go", "test", "./internal/selfupdate", "./tools/releasekit"},
		{"go", "run", "./tools/cicontract", "pipeline", "."},
		{"go", "run", "./tools/cicontract", "github-verify", "."},
		{"go", "run", "./tools/cicontract", "github-release", "."},
		{"go", "test", "./tools/forge"},
	}
	var got [][]string
	runner := func(call command) error {
		got = append(got, append([]string{call.Name}, call.Args...))
		return nil
	}
	if err := run([]string{"source"}, &bytes.Buffer{}, runner); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("commands = %#v, want %#v", got, want)
	}
}

func TestSourceStopsAtTheFirstFailedGate(t *testing.T) {
	want := errors.New("failed")
	calls := 0
	err := run([]string{"source"}, &bytes.Buffer{}, func(command) error {
		calls++
		if calls == 3 {
			return want
		}
		return nil
	})
	if !errors.Is(err, want) || calls != 3 {
		t.Fatalf("error=%v calls=%d", err, calls)
	}
}

func TestSourceIncludesForgeSpecificProvenanceWhenConfigured(t *testing.T) {
	t.Setenv("AIGW_FORGE_PROVIDER", "github")
	t.Setenv("AIGW_RELEASE_AUTHOR_EMAIL", "maintainer@example.com")
	t.Setenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE", "trust/allowed-signers")
	var got []command
	if err := run([]string{"source"}, &bytes.Buffer{}, func(call command) error {
		got = append(got, call)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	want := command{Name: "go", Args: []string{"run", "./tools/forge", "commits", "--provider", "github", "--email", "maintainer@example.com", "--allowed-signers", "trust/allowed-signers"}}
	if !slices.ContainsFunc(got, func(call command) bool { return reflect.DeepEqual(call, want) }) {
		t.Fatalf("missing provenance command: %#v", got)
	}
}

func TestSourceConfigurationRejectsIncompleteForgeProvenance(t *testing.T) {
	t.Setenv("AIGW_FORGE_PROVIDER", "unsupported")
	if _, err := configuredSourceCommands(); err == nil || !strings.Contains(err.Error(), "must be gitlab or github") {
		t.Fatalf("invalid provider error = %v", err)
	}

	for _, missing := range []string{"email", "signers"} {
		t.Run(missing, func(t *testing.T) {
			t.Setenv("AIGW_FORGE_PROVIDER", "gitlab")
			t.Setenv("AIGW_RELEASE_AUTHOR_EMAIL", "maintainer@example.com")
			t.Setenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE", "trust/allowed-signers")
			if missing == "email" {
				t.Setenv("AIGW_RELEASE_AUTHOR_EMAIL", "")
			} else {
				t.Setenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE", "")
			}
			if _, err := configuredSourceCommands(); err == nil || !strings.Contains(err.Error(), "requires author email") {
				t.Fatalf("missing %s error = %v", missing, err)
			}
		})
	}
}

func TestRunRejectsInvalidCommandShapes(t *testing.T) {
	for _, args := range [][]string{
		{"source", "extra"},
		{"native", "--platform", "linux", "extra"},
		{"trust-input"},
		{"trust-input", "--output", "out", "--github-env", "env", "extra"},
		{"fetch-tags", "extra"},
	} {
		if err := run(args, &bytes.Buffer{}, func(command) error { return nil }); err == nil {
			t.Fatalf("accepted %#v", args)
		}
	}
}

func TestTrustInputRejectsMissingOrUnwritableDestinations(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "allowed-signers")
	environment := filepath.Join(root, "github-env")
	if err := writeTrustInput(output, environment); err == nil || !strings.Contains(err.Error(), "is required") {
		t.Fatalf("missing trust error = %v", err)
	}

	t.Setenv("AIGW_RELEASE_ALLOWED_SIGNERS", "release ssh-ed25519 fixture")
	if err := writeTrustInput(filepath.Join(root, "missing", "allowed-signers"), environment); err == nil || !strings.Contains(err.Error(), "write trust input") {
		t.Fatalf("write trust error = %v", err)
	}
	if err := writeTrustInput(output, filepath.Join(root, "missing", "github-env")); err == nil || !strings.Contains(err.Error(), "open GitHub environment") {
		t.Fatalf("environment open error = %v", err)
	}
}

func TestSystemRunnerPropagatesSetupAndCommandFailures(t *testing.T) {
	root := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	if err := os.WriteFile("build", []byte("blocks directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := systemRunner(command{Name: "go", Args: []string{"version"}}); err == nil {
		t.Fatal("systemRunner accepted an unavailable build directory")
	}
	if err := os.Remove("build"); err != nil {
		t.Fatal(err)
	}
	if err := systemRunner(command{Name: "definitely-not-an-aigw-command"}); err == nil {
		t.Fatal("systemRunner hid command failure")
	}
}

func TestNativeAcceptanceUsesPortablePaths(t *testing.T) {
	for _, platform := range []string{"darwin", "linux", "windows"} {
		t.Run(platform, func(t *testing.T) {
			calls := nativeCommands(platform)
			if len(calls) != 7 {
				t.Fatalf("%s commands = %d, want 7", platform, len(calls))
			}
			for _, call := range calls {
				joined := strings.Join(append([]string{call.Name}, call.Args...), " ")
				for _, forbidden := range []string{"/tmp/", "/Users/", `C:\\Users\\`, "sh -c", "pwsh"} {
					if strings.Contains(joined, forbidden) {
						t.Fatalf("non-portable command %q", joined)
					}
				}
			}
			installed := filepath.Join("build", "acceptance", "installed", "aigw")
			if platform == "windows" {
				installed += ".exe"
			}
			if got := calls[5]; got.Name != installed || !reflect.DeepEqual(got.Args, []string{"--version"}) {
				t.Fatalf("installed smoke = %#v", got)
			}
			if got := calls[6]; len(got.Args) != 3 || got.Args[0] != "uninstall" || got.Args[2] != installed {
				t.Fatalf("portable uninstall = %#v", got)
			}
		})
	}
}

func TestNativeAcceptanceRequiresTheRealHostPlatform(t *testing.T) {
	var calls int
	if err := run([]string{"native", "--platform", runtime.GOOS}, &bytes.Buffer{}, func(command) error {
		calls++
		return nil
	}); err != nil || calls != 7 {
		t.Fatalf("native host error=%v calls=%d", err, calls)
	}
	other := "linux"
	if runtime.GOOS == other {
		other = "darwin"
	}
	if err := run([]string{"native", "--platform", other}, &bytes.Buffer{}, func(command) error { return nil }); err == nil || !strings.Contains(err.Error(), "requires "+other+" host") {
		t.Fatalf("cross-host native acceptance error = %v", err)
	}
}

func TestRejectsUnknownCommandsAndPlatforms(t *testing.T) {
	for _, args := range [][]string{nil, {"unknown"}, {"native"}, {"native", "--platform", "plan9"}} {
		if err := run(args, &bytes.Buffer{}, func(command) error { return nil }); err == nil {
			t.Fatalf("accepted %#v", args)
		}
	}
}

func TestTrustInputWritesPrivateFileAndGitHubEnvironment(t *testing.T) {
	t.Setenv("AIGW_RELEASE_ALLOWED_SIGNERS", "release ssh-ed25519 fixture")
	root := t.TempDir()
	output := filepath.Join(root, "allowed-signers")
	environment := filepath.Join(root, "github-env")
	if err := run([]string{"trust-input", "--output", output, "--github-env", environment}, &bytes.Buffer{}, func(command) error { return nil }); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output)
	if err != nil || string(data) != "release ssh-ed25519 fixture\n" {
		t.Fatalf("trust input=%q error=%v", data, err)
	}
	info, err := os.Stat(output)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("trust mode=%v error=%v", info, err)
	}
	envData, err := os.ReadFile(environment)
	if err != nil || string(envData) != "AIGW_GITHUB_ALLOWED_SIGNERS="+output+"\n" {
		t.Fatalf("GitHub environment=%q error=%v", envData, err)
	}
}

func TestFetchTagsRetriesBoundedly(t *testing.T) {
	attempts := 0
	if err := run([]string{"fetch-tags"}, &bytes.Buffer{}, func(call command) error {
		attempts++
		if attempts < 3 {
			return errors.New("transient")
		}
		return nil
	}); err != nil || attempts != 3 {
		t.Fatalf("error=%v attempts=%d", err, attempts)
	}
	attempts = 0
	if err := run([]string{"fetch-tags"}, &bytes.Buffer{}, func(command) error {
		attempts++
		return errors.New("persistent")
	}); err == nil || attempts != 3 {
		t.Fatalf("error=%v attempts=%d", err, attempts)
	}
}

func TestSystemRunnerCreatesOnlyRepositoryLocalBuildState(t *testing.T) {
	root := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	if err := systemRunner(command{Name: "go", Args: []string{"version"}}); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(root, "build", "acceptance")); err != nil || !info.IsDir() {
		t.Fatalf("repository-local build directory: info=%v error=%v", info, err)
	}
}
