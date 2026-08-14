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

	"go.yaml.in/yaml/v3"
)

func TestSourceRunsThePortableGateSequence(t *testing.T) {
	t.Setenv("AIGW_FORGE_PROVIDER", "")
	t.Setenv("AIGW_RELEASE_AUTHOR_EMAIL", "")
	t.Setenv("AIGW_RELEASE_ALLOWED_SIGNERS_FILE", "")
	want := [][]string{
		{"go", "run", "./tools/ci", "project", "--check"},
		{"openspec", "validate", "--all", "--strict", "--no-interactive"},
		{"go", "run", "./tools/ci", "links", "."},
		{"go", "run", "./tools/release", "validate-toolchain", "go.mod"},
		{"go", "run", "./tools/release", "validate-release-sources"},
		{"go", "run", "./tools/architecture", "--root", "."},
		{"go", "test", "./tools/architecture"},
		{"go", "run", "./tools/coverage", "--race"},
		{"go", "vet", "./..."},
		{"go", "tool", "staticcheck", "-checks=all,-ST1000,-ST1005", "./..."},
		{"go", "tool", "errcheck", "./..."},
		{"go", "run", "./tools/repository", "--root", ".", "go-format"},
		{"go", "run", "./tools/repository", "--root", ".", "product-surface"},
		{"go", "run", "./tools/repository", "--root", ".", "english-text"},
		{"go", "run", "./tools/repository", "--root", ".", "credentials"},
		{"go", "test", "./tools/repository"},
		{"go", "run", "./tools/repository", "--root", ".", "governance"},
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

func TestProjectionReconciliationWritesAndChecksExactContent(t *testing.T) {
	root := t.TempDir()
	projections := []projection{
		{Path: ".gitlab-ci.yml", Content: "stages: [verify]\n"},
		{Path: ".github/workflows/verify.yml", Content: "name: Verify\n"},
	}

	if err := reconcileProjections(root, projections, true); err != nil {
		t.Fatal(err)
	}
	if err := reconcileProjections(root, projections, false); err != nil {
		t.Fatalf("fresh projections drifted: %v", err)
	}

	drifted := filepath.Join(root, ".gitlab-ci.yml")
	if err := os.WriteFile(drifted, []byte("manual: edit\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := reconcileProjections(root, projections, false); err == nil || !strings.Contains(err.Error(), ".gitlab-ci.yml") {
		t.Fatalf("projection drift error = %v", err)
	}
}

func TestProjectionReconciliationRejectsInvalidManifestPaths(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"", "/tmp/workflow.yml", "../workflow.yml", ".github/../workflow.yml"} {
		err := reconcileProjections(root, []projection{{Path: path, Content: "content\n"}}, true)
		if err == nil {
			t.Fatalf("accepted projection path %q", path)
		}
	}
}

func TestProjectCommandRendersAndChecksTheTrackedProjections(t *testing.T) {
	root := t.TempDir()
	model := filepath.Join(root, ".config", "ci", "pipeline.cue")
	if err := os.MkdirAll(filepath.Dir(model), 0o755); err != nil {
		t.Fatal(err)
	}
	content := `package ci
gitlab: {stages: ["verify"]}
githubVerify: {name: "Verify"}
githubRelease: {name: "Release"}
`
	if err := os.WriteFile(model, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := run([]string{"project", "--root", root}, &bytes.Buffer{}, nil); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"project", "--check", "--root", root}, &bytes.Buffer{}, nil); err != nil {
		t.Fatalf("fresh projection check: %v", err)
	}
	for _, item := range projectionExpressions {
		path := filepath.Join(root, item.path)
		if data, err := os.ReadFile(path); err != nil || len(data) == 0 {
			t.Fatalf("projection %s: data=%q error=%v", item.path, data, err)
		}
	}
}

func TestGitLabToolchainImagesYieldToTheRunnerShell(t *testing.T) {
	root := t.TempDir()
	model := filepath.Join(root, ".config", "ci", "pipeline.cue")
	if err := os.MkdirAll(filepath.Dir(model), 0o755); err != nil {
		t.Fatal(err)
	}
	content := `package ci
#ToolchainImage: {
	name: "example.invalid/toolchain@sha256:fixture"
	entrypoint: [""]
}
gitlab: {
	"source-and-governance": {image: #ToolchainImage}
	"native-linux": {image: #ToolchainImage}
}
githubVerify: {name: "Verify"}
githubRelease: {name: "Release"}
`
	if err := os.WriteFile(model, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	gitlab := projections[0].Content
	if got := strings.Count(gitlab, "entrypoint:\n      - \"\""); got != 2 {
		t.Fatalf("empty GitLab image entrypoints = %d, want 2:\n%s", got, gitlab)
	}
}

func TestGitLabContainerJobsBootstrapRepositoryMiseBeforeLockedExecution(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var pipeline struct {
		SourceAndGovernance gitLabJob `yaml:"source-and-governance"`
		NativeLinux         gitLabJob `yaml:"native-linux"`
		ReleaseReadiness    gitLabJob `yaml:"release-readiness"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	for name, job := range map[string]gitLabJob{
		"source-and-governance": pipeline.SourceAndGovernance,
		"native-linux":          pipeline.NativeLinux,
		"release-readiness":     pipeline.ReleaseReadiness,
	} {
		lockedExecution := slices.IndexFunc(job.Script, func(command string) bool {
			return strings.HasPrefix(command, "mise exec --locked")
		})
		if len(job.Script) < 2 || job.Script[0] != "tools/ci/bootstrap-mise" || lockedExecution < 1 {
			t.Fatalf("%s script does not bootstrap repository mise before locked execution: %q", name, job.Script)
		}
	}
}

func TestNativeJobsEnableOnlyTheGoToolchain(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var pipeline struct {
		NativeDarwin  gitLabJob `yaml:"native-darwin"`
		NativeLinux   gitLabJob `yaml:"native-linux"`
		NativeWindows gitLabJob `yaml:"native-windows"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	for name, job := range map[string]gitLabJob{
		"native-darwin":  pipeline.NativeDarwin,
		"native-linux":   pipeline.NativeLinux,
		"native-windows": pipeline.NativeWindows,
	} {
		if job.Variables["MISE_ENABLE_TOOLS"] != "go" {
			t.Fatalf("%s enabled tools = %q, want go", name, job.Variables["MISE_ENABLE_TOOLS"])
		}
	}
}

type gitLabJob struct {
	Script    []string          `yaml:"script"`
	Variables map[string]string `yaml:"variables"`
}

func TestGitLabForgeContextIsScopedToSourceGovernance(t *testing.T) {
	root := t.TempDir()
	model := filepath.Join(root, ".config", "ci", "pipeline.cue")
	if err := os.MkdirAll(filepath.Dir(model), 0o755); err != nil {
		t.Fatal(err)
	}
	content := `package ci
gitlab: {
	variables: {GIT_DEPTH: "0"}
	"source-and-governance": {
		variables: AIGW_FORGE_PROVIDER: "gitlab"
	}
	"native-darwin": {}
	"native-linux": {}
	"native-windows": {}
}
githubVerify: {name: "Verify"}
githubRelease: {name: "Release"}
`
	if err := os.WriteFile(model, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	gitlab := projections[0].Content
	if got := strings.Count(gitlab, "AIGW_FORGE_PROVIDER: gitlab"); got != 1 {
		t.Fatalf("GitLab Forge context occurrences = %d, want 1:\n%s", got, gitlab)
	}
}

func TestProjectionRenderingReportsModelAndExecutableFailures(t *testing.T) {
	root := t.TempDir()
	model := filepath.Join(root, ".config", "ci", "pipeline.cue")
	if err := os.MkdirAll(filepath.Dir(model), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(model, []byte("not valid CUE"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := renderProjections(root); err == nil || !strings.Contains(err.Error(), "render .gitlab-ci.yml") {
		t.Fatalf("invalid model error = %v", err)
	}
	if err := run([]string{"project", "--root", root}, &bytes.Buffer{}, nil); err == nil || !strings.Contains(err.Error(), "render .gitlab-ci.yml") {
		t.Fatalf("project command model error = %v", err)
	}

	t.Setenv("PATH", t.TempDir())
	if _, err := renderProjections(root); err == nil || !strings.Contains(err.Error(), "executable file not found") {
		t.Fatalf("missing CUE error = %v", err)
	}
}

func TestProjectionReconciliationReportsReadAndWriteFailures(t *testing.T) {
	root := t.TempDir()
	missing := []projection{{Path: ".gitlab-ci.yml", Content: "content\n"}}
	if err := reconcileProjections(root, missing, false); err == nil || !strings.Contains(err.Error(), "read projection") {
		t.Fatalf("missing projection error = %v", err)
	}

	blockedRoot := filepath.Join(root, "blocked")
	if err := os.WriteFile(blockedRoot, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := reconcileProjections(blockedRoot, missing, true); err == nil || !strings.Contains(err.Error(), "create projection directory") {
		t.Fatalf("directory creation error = %v", err)
	}

	directoryTarget := filepath.Join(root, "directory.yml")
	if err := os.Mkdir(directoryTarget, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeProjection(directoryTarget, []byte("content\n")); err == nil || !strings.Contains(err.Error(), "write projection") {
		t.Fatalf("projection write error = %v", err)
	}
}

func TestLinksChecksOnlyTrackedMarkdown(t *testing.T) {
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
	write("untracked.md", "[broken](missing.md)\n")
	write(".git/private.md", "[broken](missing.md)\n")
	git("add", "--", "README.md", "--literal.md", "docs/guide.md")

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
		filepath.Join(root, "docs/guide.md"),
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("link command = %#v, want %#v", got, want)
	}
}

func TestLinksRejectsInvalidRepositoriesAndEmptyMarkdownSets(t *testing.T) {
	invalidRoot := t.TempDir()
	if _, err := trackedMarkdown(invalidRoot); err == nil || !strings.Contains(err.Error(), "list tracked Markdown") {
		t.Fatalf("non-repository error = %v", err)
	}
	if err := run([]string{"links", invalidRoot}, &bytes.Buffer{}, func(command) error { return nil }); err == nil || !strings.Contains(err.Error(), "list tracked Markdown") {
		t.Fatalf("links non-repository error = %v", err)
	}

	root := t.TempDir()
	process := exec.Command("git", "-C", root, "init", "--quiet")
	if output, err := process.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	if markdown, err := trackedMarkdown(root); err == nil || !strings.Contains(err.Error(), "no tracked Markdown") || markdown != nil {
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
	t.Setenv("AIGW_FORGE_PROVIDER", "")
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

func TestSourceReportsInvalidArgumentsAndConfiguredSourceFailure(t *testing.T) {
	if err := run([]string{"source", "extra"}, &bytes.Buffer{}, func(command) error { return nil }); err == nil {
		t.Fatal("source accepted an extra argument")
	}
	t.Setenv("AIGW_FORGE_PROVIDER", "unsupported")
	if err := run([]string{"source"}, &bytes.Buffer{}, func(command) error { return nil }); err == nil || !strings.Contains(err.Error(), "gitlab or github") {
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
		{"fetch-tags", "extra"},
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
	var calls []command
	if err := run([]string{"native", "--platform", runtime.GOOS}, &bytes.Buffer{}, func(call command) error {
		calls = append(calls, call)
		return nil
	}); err != nil || len(calls) != 7 {
		t.Fatalf("native host error=%v calls=%d", err, len(calls))
	}
	wantProfile := filepath.Join("build", "acceptance", "coverage-"+runtime.GOOS+".out")
	if got := calls[0]; got.Name != "go" || !slices.Equal(got.Args, []string{"run", "./tools/coverage", "--race", "--profile-output", wantProfile}) {
		t.Fatalf("native coverage command = %#v", got)
	}
	other := "linux"
	if runtime.GOOS == other {
		other = "darwin"
	}
	if err := run([]string{"native", "--platform", other}, &bytes.Buffer{}, func(command) error { return nil }); err == nil || !strings.Contains(err.Error(), "requires "+other+" host") {
		t.Fatalf("cross-host native acceptance error = %v", err)
	}
}

func TestNativeCommandsUsePortableArtifactNames(t *testing.T) {
	windows := nativeCommands("windows")
	if windows[2].Args[2] != filepath.Join("build", "acceptance", "aigw.exe") ||
		windows[0].Args[4] != filepath.Join("build", "acceptance", "coverage-windows.out") {
		t.Fatalf("Windows native commands = %#v", windows)
	}
	linux := nativeCommands("linux")
	if linux[2].Args[2] != filepath.Join("build", "acceptance", "aigw") ||
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
