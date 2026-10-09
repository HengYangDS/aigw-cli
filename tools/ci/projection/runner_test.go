package projection

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestGitLabQualityAndControlRunnerSelectors(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var pipeline struct {
		Parity         gitLabJob `yaml:"accepted-ref-parity"`
		Quality        gitLabJob `yaml:"quality"`
		NativeDarwin   gitLabJob `yaml:"native-darwin"`
		ReleaseVersion gitLabJob `yaml:"release-version"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	for name, job := range map[string]gitLabJob{
		"accepted-ref-parity": pipeline.Parity,
		"native-darwin":       pipeline.NativeDarwin,
		"release-version":     pipeline.ReleaseVersion,
	} {
		if want := []string{"ci-macos-arm64-shell"}; !slices.Equal(job.Tags, want) {
			t.Errorf("%s runner tags = %q, want %q", name, job.Tags, want)
		}
	}
	if want := []string{"$AIGW_CI_LINUX_RUNNER_TAG"}; !slices.Equal(pipeline.Quality.Tags, want) {
		t.Errorf("quality runner tags = %q, want %q", pipeline.Quality.Tags, want)
	}
}

func TestGitLabDarwinReviewQualifiesDeclaredProtectedResource(t *testing.T) {
	projections, err := renderProjections(filepath.Clean(filepath.Join("..", "..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	var pipeline struct {
		Darwin       gitLabJob `yaml:"native-darwin"`
		DarwinReview gitLabJob `yaml:"native-darwin-review"`
		Linux        gitLabJob `yaml:"native-linux"`
		Windows      gitLabJob `yaml:"native-windows"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	const resourceArgument = `--protected-file "$AIGW_REVIEW_PROTECTED_FILE"`
	for name, job := range map[string]gitLabJob{
		"native-darwin": pipeline.Darwin, "native-darwin-review": pipeline.DarwinReview,
		"native-linux": pipeline.Linux, "native-windows": pipeline.Windows,
	} {
		declared := strings.Contains(strings.Join(job.Script, "\n"), resourceArgument)
		if declared != strings.HasPrefix(name, "native-darwin") {
			t.Errorf("%s native review resource qualification declared=%t", name, declared)
		}
		if declared && !strings.Contains(strings.Join(job.Script, "\n"), `[ "${CI_COMMIT_REF_PROTECTED:-}" = false ] && [ -n "${AIGW_REVIEW_PROTECTED_FILE:-}" ]`) {
			t.Fatal("native review depends on an undeclared infrastructure resource")
		}
	}
}

func TestGitLabLinuxJobsSelectRunnerByRefTrust(t *testing.T) {
	projections, err := renderProjections(filepath.Clean(filepath.Join("..", "..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	var pipeline map[string]yaml.Node
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	const selector = "AIGW_CI_LINUX_RUNNER_TAG"
	for _, name := range []string{"quality", "native-linux", "linux-secret-service", "release-assets"} {
		node := pipeline[name]
		var job gitLabJob
		if err := node.Decode(&job); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(job.Tags, []string{"$" + selector}) {
			t.Errorf("%s relies on an external runner selector: %q", name, job.Tags)
		}
		for _, rule := range job.Rules {
			if _, duplicated := rule.Variables[selector]; duplicated {
				t.Errorf("%s duplicates workflow runner policy", name)
			}
		}
	}
	var workflow struct {
		Rules []struct {
			If        string            `yaml:"if"`
			When      string            `yaml:"when"`
			Variables map[string]string `yaml:"variables"`
		} `yaml:"rules"`
	}
	node := pipeline["workflow"]
	if err := node.Decode(&workflow); err != nil {
		t.Fatal(err)
	}
	if len(workflow.Rules) != 6 {
		t.Fatal("workflow must cover tag, review, protected push and both manual ref states")
	}
	for _, rule := range workflow.Rules {
		if rule.When == "never" {
			continue
		}
		want := "ci-linux-arm64-container-protected"
		if strings.Contains(rule.If, "merge_request_event") || strings.Contains(rule.If, `$CI_COMMIT_REF_PROTECTED == "false"`) {
			want = "ci-linux-arm64-container"
		}
		if got := rule.Variables[selector]; got != want {
			t.Errorf("workflow event %q selects %q, want %q", rule.If, got, want)
		}
	}
}

func TestGitHubNativeJobsUseHostedRunners(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}

	var workflow struct {
		On struct {
			Dispatch struct {
				Inputs map[string]struct {
					Type        string    `yaml:"type"`
					Default     yaml.Node `yaml:"default"`
					Description string    `yaml:"description"`
				} `yaml:"inputs"`
			} `yaml:"workflow_dispatch"`
		} `yaml:"on"`
		Jobs map[string]struct {
			RunsOn yaml.Node `yaml:"runs-on"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &workflow); err != nil {
		t.Fatal(err)
	}

	for _, input := range []string{"self_hosted_linux_arm64", "self_hosted_windows_arm64"} {
		if _, present := workflow.On.Dispatch.Inputs[input]; present {
			t.Errorf("GitHub Verify retains obsolete input %q", input)
		}
	}
	for name, runner := range map[string]string{
		"accepted-ref-parity":  "ubuntu-24.04",
		"linux-secret-service": "ubuntu-24.04",
		"quality":              "ubuntu-24.04",
		"native-darwin":        "macos-26-intel",
		"native-linux":         "ubuntu-24.04",
		"native-windows":       "windows-2025",
	} {
		if got := workflow.Jobs[name].RunsOn.Value; got != runner {
			t.Errorf("%s runner = %q, want %q", name, got, runner)
		}
	}
}

func TestForgeProjectionsIncludeTheCompleteNativeMatrix(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var gitlab struct {
		NativeDarwin  *gitLabJob `yaml:"native-darwin"`
		NativeLinux   *gitLabJob `yaml:"native-linux"`
		NativeWindows *gitLabJob `yaml:"native-windows"`
		SecretService *gitLabJob `yaml:"linux-secret-service"`
		Assets        gitLabJob  `yaml:"release-assets"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	if gitlab.NativeDarwin == nil || gitlab.NativeLinux == nil || gitlab.NativeWindows == nil || gitlab.SecretService == nil {
		t.Fatal("GitLab projection lacks required native evidence")
	}
	if strings.Contains(projections[0].Content, "allow_failure:") {
		t.Fatal("GitLab projection weakens a native job with allow_failure")
	}
	var gotNeeds []string
	for _, need := range gitlab.Assets.Needs {
		gotNeeds = append(gotNeeds, need.Job)
	}
	if want := []string{"quality", "native-darwin", "native-linux", "native-windows", "linux-secret-service", "release-version"}; !slices.Equal(gotNeeds, want) {
		t.Fatalf("GitLab release dependencies = %q, want %q", gotNeeds, want)
	}

	for _, projectionIndex := range []int{1} {
		var github struct {
			Jobs map[string]any `yaml:"jobs"`
		}
		if err := yaml.Unmarshal([]byte(projections[projectionIndex].Content), &github); err != nil {
			t.Fatal(err)
		}
		for _, platform := range []string{"darwin", "linux", "windows"} {
			if _, present := github.Jobs["native-"+platform]; !present {
				t.Fatalf("GitHub projection %d lacks required native %s evidence", projectionIndex, platform)
			}
		}
	}
}

func TestGitLabWindowsVerifiesRunnerOwnedMiseBeforeRepositoryTools(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	identityOutput, err := projectionCommand(root, "{version: miseVersion, executable: miseWindowsArm64ExecutableSHA256, shim: miseWindowsArm64ShimSHA256}").Output()
	if err != nil {
		t.Fatal(err)
	}
	var identity struct {
		Version    string `yaml:"version"`
		Executable string `yaml:"executable"`
		Shim       string `yaml:"shim"`
	}
	if err := yaml.Unmarshal(identityOutput, &identity); err != nil {
		t.Fatal(err)
	}
	if identity.Version == "" || len(identity.Executable) != 64 || len(identity.Shim) != 64 {
		t.Fatal("CUE must declare an exact Mise version and executable/shim digests")
	}
	var gitlab struct {
		Windows struct {
			Script      []string `yaml:"script"`
			AfterScript []string `yaml:"after_script"`
		} `yaml:"native-windows"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	commands := gitlab.Windows.Script
	if len(commands) < 2 || !strings.Contains(commands[0], `Join-Path $env:ProgramFiles 'mise\bin\mise.exe'`) ||
		!strings.Contains(commands[0], identity.Version) ||
		!strings.Contains(commands[0], identity.Executable) ||
		!strings.Contains(commands[0], "Get-FileHash -LiteralPath $mise -Algorithm SHA256") ||
		!strings.Contains(commands[1], "mise install --locked") {
		t.Fatalf("Windows runner Mise identity is not pinned before the locked toolchain: %v", commands)
	}
	if len(gitlab.Windows.AfterScript) != 1 || !strings.Contains(gitlab.Windows.AfterScript[0], "ci-mise-$env:CI_JOB_ID") {
		t.Fatalf("Windows Mise job state has no exact cleanup: %v", gitlab.Windows.AfterScript)
	}
	for _, required := range []string{
		"whoami.exe /user",
		"$shim = Join-Path (Split-Path -Parent $mise) 'mise-shim.exe'",
		"Get-FileHash -LiteralPath $shim -Algorithm SHA256",
		identity.Shim,
		"icacls.exe $shim",
	} {
		if !strings.Contains(commands[0], required) {
			t.Errorf("Windows Mise preflight omits %q", required)
		}
	}
	hashReads := 0
	for line := range strings.SplitSeq(commands[0], "\n") {
		if strings.Contains(line, "Get-FileHash -LiteralPath") {
			if !strings.Contains(line, "'mise.lock'") {
				hashReads++
			}
			if !strings.Contains(line, "-ErrorAction Stop") {
				t.Errorf("Windows hash read can hide its actual failure: %s", line)
			}
		}
	}
	if hashReads != 2 {
		t.Errorf("Windows preflight has %d hash reads, want only the two supplied executable identities", hashReads)
	}
	for _, required := range []string{
		"robocopy.exe $emptyDirectory $jobDirectory /MIR /R:1 /W:1",
		"$mirrorExit -ge 8",
		"[IO.Directory]::Delete($jobDirectory)",
		"$cause.GetType().FullName",
		"$cause.HResult",
	} {
		if !strings.Contains(gitlab.Windows.AfterScript[0], required) {
			t.Errorf("Windows cleanup lacks exact owned-tree mirror or structured failure %q", required)
		}
	}
}

func requireWindowsMiseJobStorage(t *testing.T, name string, script, cleanup []string, directory string) {
	t.Helper()
	if len(script) == 0 || len(cleanup) != 1 ||
		!strings.Contains(script[0], "$jobDirectory = "+directory) ||
		!strings.Contains(cleanup[0], directory) ||
		!strings.Contains(cleanup[0], "Test-Path -LiteralPath $jobDirectory") {
		t.Fatalf("%s Mise lifecycle enters the Go module or lacks exact teardown", name)
	}
}

func TestGitLabWindowsLockedToolsUseJobScopedMirror(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	type windowsJob struct {
		Script      []string `yaml:"script"`
		AfterScript []string `yaml:"after_script"`
	}
	var gitlab struct {
		NativeWindows       windowsJob `yaml:"native-windows"`
		NativeWindowsReview windowsJob `yaml:"native-windows-review"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	windows := gitlab.NativeWindows
	if len(windows.Script) < 2 {
		t.Fatalf("Windows job lacks the Mise preflight and locked install: %+v", windows)
	}
	if !strings.Contains(windows.Script[1], "mise install --locked") {
		t.Fatalf("Windows job does not install locked tools: %+v", windows)
	}
	const jobDirectory = `Join-Path (Split-Path -Parent $env:CI_PROJECT_DIR) "aigw-ci-mise-$env:CI_JOB_ID"`
	for name, job := range map[string]windowsJob{"protected": windows, "review": gitlab.NativeWindowsReview} {
		requireWindowsMiseJobStorage(t, name, job.Script, job.AfterScript, jobDirectory)
		selected := strings.Index(job.Script[0], "if ($env:AIGW_TOOL_SOURCE -eq 'peer')")
		credentials := strings.Index(job.Script[0], "$netrc = ")
		if selected < 0 || credentials <= selected {
			t.Errorf("%s Windows job acquires mirror credentials before explicit selection", name)
		}
		if !strings.Contains(job.Script[0], `= "${mirrorBase}" + 'release-$1-$2-$3.json'`) {
			t.Errorf("%s Windows job expands Mise regex captures before Mise receives them", name)
		}
	}
	if len(gitlab.NativeWindowsReview.Script) == 0 || windows.Script[0] != gitlab.NativeWindowsReview.Script[0] ||
		!reflect.DeepEqual(windows.AfterScript, gitlab.NativeWindowsReview.AfterScript) {
		t.Fatal("protected and review Windows jobs use different Mise admission or cleanup")
	}
	if _, err := os.Stat(filepath.Join(root, "tools", "ci", "bootstrap", "mise-windows.ps1")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("obsolete Windows bootstrap remains: %v", err)
	}
	for _, required := range []string{"MISE_URL_REPLACEMENTS", "MISE_NETRC_FILE", "CI_SERVER_HOST", "CI_JOB_TOKEN", "packages/generic/mise-github/", "mise.lock", "Get-FileHash", "$lockDigest/", "release-$1-$2-$3.json", "https://github.com/", "https://api.github.com/", "icacls"} {
		if !strings.Contains(windows.Script[0], required) {
			t.Errorf("Windows job omits scoped mirror control %q", required)
		}
	}
	probe := strings.Index(windows.Script[0], "$reported = & $mise --version")
	if probe < 0 {
		t.Fatal("Windows job never probes the runner-owned Mise executable")
	}
	for _, name := range []string{"MISE_CONFIG_DIR", "MISE_CACHE_DIR", "MISE_STATE_DIR", "MISE_DATA_DIR"} {
		assignment := strings.Index(windows.Script[0], "'"+name+"'")
		if assignment < 0 || assignment > probe {
			t.Fatalf("%s is not owned by the job before Mise loads configuration", name)
		}
	}
	if assignment := strings.Index(windows.Script[0], "[Environment]::SetEnvironmentVariable($name, $path)"); assignment < 0 || assignment > probe ||
		!strings.Contains(windows.Script[0], "$path = Join-Path $jobDirectory $name") {
		t.Fatal("Windows job does not confine Mise state before its first invocation")
	}
	if trust := strings.Index(windows.Script[0], "$env:MISE_TRUSTED_CONFIG_PATHS = $env:CI_PROJECT_DIR"); trust < 0 || trust > probe {
		t.Fatal("Windows job does not trust its exact checkout before Mise walks config ancestors")
	}
	if !strings.Contains(windows.Script[0], "throw 'Runner-owned Mise failed to start under the job identity.'") {
		t.Fatal("Windows job conflates a Mise startup failure with a version mismatch")
	}
}
