package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestSourceRunsThePortableGateSequence(t *testing.T) {
	t.Setenv("AIGW_RELEASE_AUTHOR_EMAIL", "")
	t.Setenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE", "")
	want := [][]string{
		{"cue", "fmt", "--check", "--files", ".config/ci"},
		{"go", "run", "./tools/ci", "project", "--check"},
		{"openspec", "validate", "--all", "--strict", "--no-interactive"},
		{"ec", "-disable-indentation", "-disable-indent-size"},
		{"prettier", "--check", "--config", ".config/checks/markdown/prettier.json", "--ignore-path", ".config/checks/markdown/prettier-ignore", "*.md", "docs/**/*.md", "openspec/**/*.md"},
		{"markdownlint-cli2", "--config", ".config/checks/markdown/policy.yaml"},
		{"go", "run", "./tools/ci", "links", "."},
		{"gitleaks", "dir", "--redact", "--no-banner", "."},
		{"go", "run", "./tools/release", "validate-toolchain", "go.mod"},
		{"go", "run", "./tools/release", "validate-release-sources"},
		{"go", "run", "./tools/architecture", "--root", "."},
		{"go", "test", "./tools/architecture"},
		{"go", "run", "./tools/coverage", "--race"},
		{"go", "vet", "./..."},
		{"go", "tool", "staticcheck", "-checks=SA*,S1*", "./..."},
		{"go", "tool", "errcheck", "./..."},
		{"go", "run", "./tools/repository", "--root", ".", "go-format"},
		{"go", "test", "./tools/repository"},
		{"go", "run", "./tools/repository", "--root", ".", "protected-lifecycle"},
		{"go", "test", "./internal/upgrade", "./tools/release"},
		{"actionlint"},
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

func TestLinksChecksCurrentRepositoryMarkdown(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		process := exec.Command("git", append([]string{"-C", root}, args...)...)
		if output, err := process.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	write := func(relative, content string) {
		t.Helper()
		path := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "--quiet")
	write("README.md", "[valid](docs/guide.md)\n")
	write("--literal.md", "# Literal\n")
	write("docs/guide.md", "# Guide\n")
	write("new-guide.md", "# New guide\n")
	write("retired.md", "# Retired\n")
	write(".git/private.md", "[broken](missing.md)\n")
	git("add", "--", "README.md", "--literal.md", "docs/guide.md", "retired.md")
	if err := os.Remove(filepath.Join(root, "retired.md")); err != nil {
		t.Fatal(err)
	}

	var got command
	if err := run([]string{"links", root}, &bytes.Buffer{}, func(call command) error {
		got = call
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	want := command{Name: "lychee", Args: []string{
		"--offline", "--no-progress", "--cache=false", "--",
		filepath.Join(root, "--literal.md"), filepath.Join(root, "README.md"),
		filepath.Join(root, "docs/guide.md"), filepath.Join(root, "new-guide.md"),
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("link command = %#v, want %#v", got, want)
	}
}

func TestLinksRejectsInvalidRepositoriesAndEmptyMarkdownSets(t *testing.T) {
	invalidRoot := t.TempDir()
	if _, err := currentRepositoryMarkdown(invalidRoot); err == nil || !strings.Contains(err.Error(), "list repository Markdown") {
		t.Fatalf("non-repository error = %v", err)
	}
	if err := run([]string{"links", invalidRoot}, &bytes.Buffer{}, func(command) error { return nil }); err == nil || !strings.Contains(err.Error(), "list repository Markdown") {
		t.Fatalf("links non-repository error = %v", err)
	}

	root := t.TempDir()
	process := exec.Command("git", "-C", root, "init", "--quiet")
	if output, err := process.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	if markdown, err := currentRepositoryMarkdown(root); err == nil || !strings.Contains(err.Error(), "no current Markdown") || markdown != nil {
		t.Fatalf("empty Markdown set = %#v, error = %v", markdown, err)
	}
}

func TestLinksPropagatesLycheeFailure(t *testing.T) {
	root := t.TempDir()
	process := exec.Command("git", "-C", root, "init", "--quiet")
	if output, err := process.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# AIGW\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	process = exec.Command("git", "-C", root, "add", "--", "README.md")
	if output, err := process.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, output)
	}

	want := errors.New("lychee failed")
	err := run([]string{"links", root}, &bytes.Buffer{}, func(command) error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("link error = %v", err)
	}
}

func TestStaticRunsTheNonBehaviorGateSequence(t *testing.T) {
	var got [][]string
	runner := func(call command) error {
		got = append(got, append([]string{call.Name}, call.Args...))
		return nil
	}
	if err := run([]string{"static"}, &bytes.Buffer{}, runner); err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(got, func(call []string) bool {
		return slices.Equal(call, []string{"go", "run", "./tools/coverage", "--race"})
	}) {
		t.Fatalf("static gate duplicated behavior coverage: %#v", got)
	}
	if len(got) == 0 {
		t.Fatal("static gate ran no checks")
	}
}

func TestSourceStopsAtTheFirstFailedGate(t *testing.T) {
	t.Setenv("AIGW_RELEASE_AUTHOR_EMAIL", "")
	t.Setenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE", "")
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

func TestSourceIncludesProductProvenanceWhenConfigured(t *testing.T) {
	t.Setenv("AIGW_RELEASE_AUTHOR_EMAIL", "maintainer@example.com")
	t.Setenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE", "trust/allowed-signers")
	var got []command
	if err := run([]string{"source"}, &bytes.Buffer{}, func(call command) error {
		got = append(got, call)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	want := command{Name: "go", Args: []string{"run", "./tools/forge", "commits", "--email", "maintainer@example.com", "--allowed-signers", "trust/allowed-signers"}}
	if !slices.ContainsFunc(got, func(call command) bool { return reflect.DeepEqual(call, want) }) {
		t.Fatalf("missing provenance command: %#v", got)
	}
	gitleaks := command{Name: "gitleaks", Args: []string{"dir", "--redact", "--no-banner", "."}}
	if !slices.ContainsFunc(got, func(call command) bool { return reflect.DeepEqual(call, gitleaks) }) {
		t.Fatalf("missing Forge-scoped gitleaks command: %#v", got)
	}
}

func TestGitleaksRemainsInProductSourceGraph(t *testing.T) {
	t.Setenv("AIGW_RELEASE_AUTHOR_EMAIL", "maintainer@example.com")
	t.Setenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE", "trust/allowed-signers")
	commands, err := configuredSourceCommands()
	if err != nil {
		t.Fatal(err)
	}
	want := command{Name: "gitleaks", Args: []string{"dir", "--redact", "--no-banner", "."}}
	if !slices.ContainsFunc(commands, func(call command) bool { return reflect.DeepEqual(call, want) }) {
		t.Fatalf("commands = %#v", commands)
	}
}

func TestSourceConfigurationRejectsIncompleteProductProvenance(t *testing.T) {
	for _, missing := range []string{"email", "signers"} {
		t.Run(missing, func(t *testing.T) {
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

func TestSourceReportsInvalidArgumentsAndConfiguredSourceFailure(t *testing.T) {
	if err := run([]string{"source", "extra"}, &bytes.Buffer{}, func(command) error { return nil }); err == nil {
		t.Fatal("source accepted an extra argument")
	}
	t.Setenv("AIGW_RELEASE_AUTHOR_EMAIL", "maintainer@example.com")
	t.Setenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE", "")
	if err := run([]string{"source"}, &bytes.Buffer{}, func(command) error { return nil }); err == nil || !strings.Contains(err.Error(), "allowed signers") {
		t.Fatalf("configured source error = %v", err)
	}
}

func TestRunRejectsInvalidCommandShapes(t *testing.T) {
	for _, args := range [][]string{
		{"project", "extra"},
		{"static", "extra"},
		{"source", "extra"},
		{"links"},
		{"links", ".", "extra"},
		{"native", "--platform", "linux", "extra"},
		{"trust-input"},
		{"trust-input", "--output", "out", "--github-env", "env", "extra"},
	} {
		if err := run(args, &bytes.Buffer{}, func(command) error { return nil }); err == nil {
			t.Fatalf("accepted %#v", args)
		}
	}
}

func TestTrustInputRejectsMissingOrUnwritableDestinations(t *testing.T) {
	t.Setenv("AIGW_RELEASE_ALLOWED_SIGNERS", "")
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
			calls := nativeCommands(platform, "1.2.3")
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
			if got := calls[2]; !slices.Contains(got.Args, "-ldflags=-X=aigw-cli/internal/cli.Version=1.2.3") {
				t.Fatalf("native build lacks VERSION-derived identity: %#v", got)
			}
		})
	}
}

func TestNativeAcceptanceRequiresTheRealHostPlatform(t *testing.T) {
	root := repositoryRoot(t)
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	var calls []command
	if err := run([]string{"native", "--platform", runtime.GOOS}, &bytes.Buffer{}, func(call command) error {
		calls = append(calls, call)
		return nil
	}); err != nil || len(calls) != 7 {
		t.Fatalf("native host error=%v calls=%d", err, len(calls))
	}
	if runtime.GOOS == "windows" {
		if got := calls[0]; got.Name != "go" || !slices.Equal(got.Args, []string{"test", "./..."}) {
			t.Fatalf("native Windows test command = %#v", got)
		}
	} else {
		wantProfile := filepath.Join("build", "acceptance", "coverage-"+runtime.GOOS+".out")
		if got := calls[0]; got.Name != "go" || !slices.Equal(got.Args, []string{"run", "./tools/coverage", "--race", "--profile-output", wantProfile}) {
			t.Fatalf("native coverage command = %#v", got)
		}
	}
	other := "linux"
	if runtime.GOOS == other {
		other = "darwin"
	}
	if err := run([]string{"native", "--platform", other}, &bytes.Buffer{}, func(command) error { return nil }); err == nil || !strings.Contains(err.Error(), "requires "+other+" host") {
		t.Fatalf("cross-host native acceptance error = %v", err)
	}
}

func TestSourceVersionUsesOneValidatedSSOT(t *testing.T) {
	root := t.TempDir()
	valid := filepath.Join(root, "valid")
	if err := os.WriteFile(valid, []byte("1.2.3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := sourceVersion(valid); err != nil || got != "1.2.3" {
		t.Fatalf("valid source version = %q, %v", got, err)
	}
	for name, content := range map[string]string{"empty": "\n", "spaced": "1.2.3 invalid\n"} {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := sourceVersion(path); err == nil || !strings.Contains(err.Error(), "invalid source version") {
			t.Fatalf("%s source version error = %v", name, err)
		}
	}
	if _, err := sourceVersion(filepath.Join(root, "missing")); err == nil || !strings.Contains(err.Error(), "read source version") {
		t.Fatalf("missing source version error = %v", err)
	}
}

func TestNativeAcceptanceRefusesARepositoryWithoutVersionTruth(t *testing.T) {
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	calls := 0
	err = run([]string{"native", "--platform", runtime.GOOS}, &bytes.Buffer{}, func(command) error {
		calls++
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "read source version") || calls != 0 {
		t.Fatalf("missing VERSION error=%v calls=%d", err, calls)
	}
}

func TestNativeCommandsUsePortableArtifactNames(t *testing.T) {
	windows := nativeCommands("windows", "1.2.3")
	if windows[2].Args[3] != filepath.Join("build", "acceptance", "aigw.exe") ||
		!slices.Equal(windows[0].Args, []string{"test", "./..."}) {
		t.Fatalf("Windows native commands = %#v", windows)
	}
	linux := nativeCommands("linux", "1.2.3")
	if linux[2].Args[3] != filepath.Join("build", "acceptance", "aigw") ||
		linux[0].Args[4] != filepath.Join("build", "acceptance", "coverage-linux.out") {
		t.Fatalf("Linux native commands = %#v", linux)
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
	if err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0o600) {
		t.Fatalf("trust mode=%v error=%v", info, err)
	}
	envData, err := os.ReadFile(environment)
	if err != nil || string(envData) != "AIGW_RELEASE_ALLOWED_SIGNERS_FILE="+output+"\n" {
		t.Fatalf("GitHub environment=%q error=%v", envData, err)
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
