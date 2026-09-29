package projection

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestGitLabWindowsBootstrapsPinnedMiseBeforeRepositoryTools(t *testing.T) {
	projections, err := renderProjections(filepath.Clean(filepath.Join("..", "..", "..")))
	if err != nil {
		t.Fatal(err)
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
	if len(commands) < 2 || !strings.Contains(commands[0], ". ./tools/ci/bootstrap/mise-windows.ps1") ||
		!strings.Contains(commands[0], "2026.9.16") ||
		!strings.Contains(commands[0], "8e021ea855f50880ee4c8515f483b2cd07b27edb6109a8b4364ff09af65136b5") ||
		!strings.Contains(commands[1], "mise install --locked") {
		t.Fatalf("Windows Mise bootstrap is not pinned before the locked toolchain: %v", commands)
	}
	if len(gitlab.Windows.AfterScript) != 1 || !strings.Contains(gitlab.Windows.AfterScript[0], "ci-mise-$env:CI_JOB_ID") {
		t.Fatalf("Windows Mise bootstrap has no exact job-owned cleanup: %v", gitlab.Windows.AfterScript)
	}
}

func TestGitLabUnixLockedToolsUseJobScopedMirrorWithoutChangingGitHub(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var gitlab struct {
		LinuxToolchain struct {
			BeforeScript []string `yaml:"before_script"`
			AfterScript  []string `yaml:"after_script"`
		} `yaml:".linux-toolchain"`
		NativeDarwin struct {
			Script      []string `yaml:"script"`
			AfterScript []string `yaml:"after_script"`
		} `yaml:"native-darwin"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	for name, commands := range map[string][]string{
		"Linux": gitlab.LinuxToolchain.BeforeScript,
		"macOS": gitlab.NativeDarwin.Script,
	} {
		mirror := slices.IndexFunc(commands, func(command string) bool {
			return strings.Contains(command, "MISE_URL_REPLACEMENTS")
		})
		install := slices.IndexFunc(commands, func(command string) bool {
			return strings.Contains(command, "mise install --locked")
		})
		if mirror < 0 || install <= mirror {
			t.Errorf("GitLab %s must configure the mirror before locked installation: %v", name, commands)
			continue
		}
		prelude := commands[mirror]
		for _, required := range []string{"CI_API_V4_URL", "CI_PROJECT_ID", "CI_SERVER_HOST", "CI_JOB_TOKEN", "MISE_NETRC_FILE", "github.com/", "api.github.com/", "mise-github/v1/", "CI_JOB_ID"} {
			if !strings.Contains(prelude, required) {
				t.Errorf("GitLab %s mirror prelude omits %q", name, required)
			}
		}
		metadata := strings.Index(prelude, "regex:^https://api[.]github[.]com/repos/")
		fallback := strings.Index(prelude, `"https://api.github.com/"`)
		if metadata < 0 || fallback <= metadata || !strings.Contains(prelude, "release-$1-$2-$3.json") {
			t.Errorf("GitLab %s must resolve mirrored release metadata before the API fallback", name)
		}
		for _, forbidden := range []string{"192.168.64.101", "projects/456", "$HOME/.netrc"} {
			if strings.Contains(prelude, forbidden) {
				t.Errorf("GitLab %s mirror prelude hard-codes %q", name, forbidden)
			}
		}
	}
	for name, commands := range map[string][]string{
		"Linux": gitlab.LinuxToolchain.AfterScript,
		"macOS": gitlab.NativeDarwin.AfterScript,
	} {
		if !slices.ContainsFunc(commands, func(command string) bool {
			return strings.Contains(command, "CI_JOB_ID") && strings.Contains(command, "mise-mirror")
		}) {
			t.Errorf("GitLab %s has no exact job-owned mirror credential cleanup: %v", name, commands)
		}
	}
	if strings.Contains(projections[1].Content, "mise-github/v1/") {
		t.Fatal("GitHub projection unexpectedly depends on the GitLab tool mirror")
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
	if len(windows.Script) < 2 ||
		!strings.Contains(windows.Script[0], "mise-windows.ps1") ||
		!strings.Contains(windows.Script[0], "-MirrorResource 'packages/generic/mise-github/v1/'") ||
		!strings.Contains(windows.Script[0], "-ReleaseMetadataPattern 'regex:^https://api[.]github[.]com/repos/") ||
		!strings.Contains(windows.Script[0], "-ReleaseMetadataResource 'release-$1-$2-$3.json'") ||
		!strings.Contains(windows.Script[1], "mise install --locked") ||
		!slices.ContainsFunc(windows.AfterScript, func(command string) bool {
			return strings.Contains(command, "ci-mise-$env:CI_JOB_ID")
		}) {
		t.Fatalf("Windows must bootstrap the mirror and clean its exact job directory: %+v", windows)
	}
	const jobDirectory = `Join-Path (Split-Path -Parent $env:CI_PROJECT_DIR) "aigw-ci-mise-$env:CI_JOB_ID"`
	for name, job := range map[string]windowsJob{"protected": windows, "review": gitlab.NativeWindowsReview} {
		requireWindowsMiseJobStorage(t, name, job.Script, job.AfterScript, jobDirectory)
	}
	windowsBootstrap, err := os.ReadFile(filepath.Join(root, "tools", "ci", "bootstrap", "mise-windows.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(windowsBootstrap), "$expected = "+jobDirectory) {
		t.Fatal("Windows bootstrap still puts the installed Go tree inside the module")
	}
	for _, required := range []string{"MISE_URL_REPLACEMENTS", "MISE_NETRC_FILE", "CI_SERVER_HOST", "CI_JOB_TOKEN", "$MirrorResource", "$ReleaseMetadataPattern", "$ReleaseMetadataResource", "https://github.com/", "https://api.github.com/", "icacls"} {
		if !strings.Contains(string(windowsBootstrap), required) {
			t.Errorf("Windows bootstrap omits job-scoped mirror control %q", required)
		}
	}
	probe := strings.Index(string(windowsBootstrap), "$reported = & $executable --version")
	if probe < 0 {
		t.Fatal("Windows bootstrap never probes the pinned Mise executable")
	}
	for _, name := range []string{"MISE_CONFIG_DIR", "MISE_CACHE_DIR", "MISE_STATE_DIR", "MISE_DATA_DIR"} {
		assignment := strings.Index(string(windowsBootstrap), "$env:"+name+" = Join-Path $Directory")
		if assignment < 0 || assignment > probe {
			t.Fatalf("%s is not owned by the job before Mise loads configuration", name)
		}
	}
	if trust := strings.Index(string(windowsBootstrap), "$env:MISE_TRUSTED_CONFIG_PATHS = $env:CI_PROJECT_DIR"); trust < 0 || trust > probe {
		t.Fatal("Windows bootstrap does not trust its exact checkout before Mise walks config ancestors")
	}
	if !strings.Contains(string(windowsBootstrap), "throw 'Pinned Mise executable failed to start under the job runtime.'") {
		t.Fatal("Windows bootstrap conflates a Mise startup failure with a version mismatch")
	}
}

func requireWindowsMiseJobStorage(t *testing.T, name string, script, cleanup []string, directory string) {
	t.Helper()
	if len(script) == 0 || len(cleanup) != 1 ||
		!strings.Contains(script[0], "-Directory ("+directory+")") ||
		!strings.Contains(cleanup[0], directory) ||
		!strings.Contains(cleanup[0], "Test-Path -LiteralPath $jobDirectory") {
		t.Fatalf("%s Mise lifecycle enters the Go module or lacks exact teardown", name)
	}
}

func TestNativeJobsEnableTheirExactCommandToolClosure(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	checkGitLabNativeToolClosure(t, projections[0].Content)
	for _, item := range projections[1:] {
		if item.Path == ".github/workflows/verify.yml" {
			checkGitHubNativeToolClosure(t, item)
			return
		}
	}
	t.Fatal("GitHub verification projection is missing")
}

func TestLinuxSecretServiceUsesOnlyItsLockedExecutableClosure(t *testing.T) {
	projections, err := renderProjections(filepath.Clean(filepath.Join("..", "..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	const tools = "go,github:goreleaser/goreleaser"
	var gitlab struct {
		SecretService gitLabJob `yaml:"linux-secret-service"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	if got := gitlab.SecretService.Variables["MISE_ENABLE_TOOLS"]; got != tools {
		t.Fatalf("GitLab Secret Service toolchain = %q, want %q", got, tools)
	}
	if len(gitlab.SecretService.Script) != 1 || !strings.Contains(gitlab.SecretService.Script[0], "TestNativeProductJourney/system_credential_store") {
		t.Fatal("GitLab Secret Service job runs more than its focused qualification")
	}
	var github struct {
		Jobs map[string]struct {
			Env   map[string]string `yaml:"env"`
			Steps []struct {
				Name string `yaml:"name"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &github); err != nil {
		t.Fatal(err)
	}
	job, present := github.Jobs["linux-secret-service"]
	if !present || job.Env["MISE_ENABLE_TOOLS"] != tools {
		t.Fatalf("GitHub Secret Service toolchain = %q, present=%t", job.Env["MISE_ENABLE_TOOLS"], present)
	}
	if len(job.Steps) != 3 || job.Steps[2].Name != "Qualify Linux Secret Service" {
		t.Fatal("GitHub Secret Service job runs more than its focused qualification")
	}
}

func TestLinuxSecretServiceHasItsOwnRequiredJob(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				Name string `yaml:"name"`
				Run  string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &workflow); err != nil {
		t.Fatal(err)
	}
	for _, step := range workflow.Jobs["native-linux"].Steps {
		if step.Name == "Qualify Linux Secret Service" {
			t.Fatal("native Linux job duplicates the Secret Service qualification")
		}
	}
	secretService, present := workflow.Jobs["linux-secret-service"]
	if !present {
		t.Fatal("GitHub lacks independent Linux Secret Service evidence")
	}
	if len(secretService.Steps) != 3 || secretService.Steps[2].Name != "Qualify Linux Secret Service" {
		t.Fatal("GitHub native Linux CI does not qualify real Secret Service")
	}
	githubQualification := secretService.Steps[2].Run
	for _, required := range []string{
		"dbus-x11 gnome-keyring",
		"sudo -n timeout --verbose --kill-after=5s 240s",
		"Acquire::http::Timeout=30",
		"dbus-run-session",
		"SetAlias default /org/freedesktop/secrets/collection/session",
		"AIGW_VERIFY_SYSTEM_KEYRING=1",
		"TestNativeProductJourney/system_credential_store",
		"grep -Fq -- \"--- PASS: TestNativeProductJourney/system_credential_store\"",
	} {
		if !strings.Contains(githubQualification, required) {
			t.Fatalf("Linux Secret Service qualification omits %q", required)
		}
	}
	var gitlab struct {
		Linux         gitLabJob  `yaml:"native-linux"`
		SecretService *gitLabJob `yaml:"linux-secret-service"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	if gitlab.SecretService == nil {
		t.Fatal("GitLab lacks independent Linux Secret Service evidence")
	}
	for _, command := range gitlab.Linux.Script {
		if strings.Contains(command, "TestNativeProductJourney/system_credential_store") {
			t.Fatal("GitLab native Linux job duplicates Secret Service qualification")
		}
	}
	if !slices.Equal(gitlab.SecretService.Extends, []string{".linux-toolchain"}) ||
		!slices.Equal(gitlab.SecretService.Tags, gitlab.Linux.Tags) ||
		!reflect.DeepEqual(gitlab.SecretService.Rules, gitlab.Linux.Rules) {
		t.Fatal("GitLab Secret Service job must use the same Linux runner and event admission")
	}
	if len(gitlab.SecretService.Script) != 1 {
		t.Fatal("GitLab native Linux CI does not qualify real Secret Service")
	}
	gitlabQualification := gitlab.SecretService.Script[0]
	for _, required := range []string{
		"DEBIAN_FRONTEND=noninteractive timeout --verbose --kill-after=5s 240s",
		"Acquire::http::Timeout=30",
		"install --no-install-recommends -y dbus-x11 gnome-keyring libglib2.0-bin",
	} {
		if !strings.Contains(gitlabQualification, required) {
			t.Fatalf("GitLab Secret Service preparation omits %q", required)
		}
	}
	githubBus := strings.Index(githubQualification, "dbus-run-session")
	gitlabBus := strings.Index(gitlabQualification, "dbus-run-session")
	if githubBus < 0 || gitlabBus < 0 || githubQualification[githubBus:] != gitlabQualification[gitlabBus:] {
		t.Fatal("GitHub and GitLab native Linux jobs must run the same Secret Service journey")
	}
}

func checkGitLabNativeToolClosure(t *testing.T, content string) {
	t.Helper()
	var pipeline struct {
		NativeDarwin  *gitLabJob `yaml:"native-darwin"`
		NativeLinux   *gitLabJob `yaml:"native-linux"`
		NativeWindows *gitLabJob `yaml:"native-windows"`
	}
	if err := yaml.Unmarshal([]byte(content), &pipeline); err != nil {
		t.Fatal(err)
	}
	if pipeline.NativeDarwin == nil || pipeline.NativeLinux == nil || pipeline.NativeWindows == nil {
		t.Fatal("GitLab must project all three native acceptance jobs")
	}
	for name, job := range map[string]gitLabJob{
		"native-darwin":  *pipeline.NativeDarwin,
		"native-linux":   *pipeline.NativeLinux,
		"native-windows": *pipeline.NativeWindows,
	} {
		enabled := strings.Split(job.Variables["MISE_ENABLE_TOOLS"], ",")
		// The native Go suite includes real glab loopback tests in internal/upgrade.
		for _, tool := range []string{"go", "node", "npm", "github:golangci/golangci-lint", "github:goreleaser/goreleaser", "github:anchore/syft", "gh", "glab"} {
			if !slices.Contains(enabled, tool) {
				t.Errorf("GitLab %s lacks native acceptance tool %s", name, tool)
			}
		}
		hasDarwinSigner := slices.Contains(enabled, "github:indygreg/apple-platform-rs")
		if hasDarwinSigner != (name == "native-darwin") {
			t.Errorf("GitLab %s Darwin signer presence = %t", name, hasDarwinSigner)
		}
		bootstrap := slices.Index(job.Script, "mise run bootstrap")
		if bootstrap < 0 || bootstrap >= len(job.Script)-1 {
			t.Errorf("GitLab %s must prepare locked dependencies before native acceptance", name)
		}
	}
}

func checkGitHubNativeToolClosure(t *testing.T, projection projection) {
	t.Helper()
	var workflow struct {
		Jobs map[string]struct {
			Env   map[string]string `yaml:"env"`
			Steps []struct {
				Run string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projection.Content), &workflow); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"native-darwin", "native-linux", "native-windows"} {
		job := workflow.Jobs[name]
		tools := job.Env["MISE_ENABLE_TOOLS"]
		for _, required := range []string{"go,node,npm", "github:golangci/golangci-lint", "github:goreleaser/goreleaser", "github:anchore/syft", "gh", "glab", "inputs.full_quality"} {
			if !strings.Contains(tools, required) {
				t.Errorf("%s %s tool closure lacks %q: %q", projection.Path, name, required, tools)
			}
		}
		hasDarwinSigner := strings.Contains(tools, "github:indygreg/apple-platform-rs")
		if hasDarwinSigner != (name == "native-darwin") {
			t.Errorf("%s %s Darwin signer presence = %t", projection.Path, name, hasDarwinSigner)
		}
		if name != "native-windows" && !strings.Contains(tools, "github:lycheeverse/lychee") {
			t.Errorf("%s %s lacks the supported link checker: %q", projection.Path, name, tools)
		}
		bootstrap := false
		for _, step := range job.Steps {
			if strings.Contains(step.Run, "./tools/ci native") && !bootstrap {
				t.Errorf("%s %s runs native acceptance before locked dependency preparation", projection.Path, name)
			}
			bootstrap = bootstrap || step.Run == "mise run bootstrap"
		}
	}
}

func TestGitHubWindowsUsesThePortableNativeToolClosure(t *testing.T) {
	projections, err := renderProjections(filepath.Clean(filepath.Join("..", "..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			RunsOn yaml.Node         `yaml:"runs-on"`
			Env    map[string]string `yaml:"env"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &workflow); err != nil {
		t.Fatal(err)
	}

	job := workflow.Jobs["native-windows"]
	const runner = "windows-2025"
	if job.RunsOn.Value != runner {
		t.Fatalf("native Windows runner selector = %q, want %q", job.RunsOn.Value, runner)
	}
	tools := job.Env["MISE_ENABLE_TOOLS"]
	if !strings.Contains(tools, "inputs.full_quality") || !strings.Contains(tools, "github:lycheeverse/lychee") {
		t.Errorf("hosted full-quality closure is not selectable: %q", tools)
	}
	_, defaultTools, ok := strings.Cut(tools, " || '")
	if !ok {
		t.Fatalf("native Windows tool selection has no explicit default: %q", tools)
	}
	for _, required := range []string{"go,node,npm", "github:golangci/golangci-lint", "github:goreleaser/goreleaser", "github:anchore/syft"} {
		if !strings.Contains(defaultTools, required) {
			t.Errorf("native Windows default tool closure lacks %q: %q", required, defaultTools)
		}
	}
	for _, unsupported := range []string{"github:indygreg/apple-platform-rs", "github:lycheeverse/lychee"} {
		if strings.Contains(strings.TrimSuffix(defaultTools, "' }}"), unsupported) {
			t.Errorf("Windows ARM64 default tool closure contains unsupported %q: %q", unsupported, tools)
		}
	}
}

func TestWindowsClientInstallerUsesPinnedContentAPI(t *testing.T) {
	projections, err := renderProjections(filepath.Clean(filepath.Join("..", "..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	type step struct {
		Name           string            `yaml:"name"`
		If             string            `yaml:"if"`
		Run            string            `yaml:"run"`
		TimeoutMinutes int               `yaml:"timeout-minutes"`
		Env            map[string]string `yaml:"env"`
	}
	var workflow struct {
		Jobs map[string]struct {
			Steps []step `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &workflow); err != nil {
		t.Fatal(err)
	}
	windows := workflow.Jobs["native-windows"].Steps
	fetchIndex := slices.IndexFunc(windows, func(item step) bool { return item.Name == "Fetch pinned Hermes installer" })
	historicalIndex := slices.IndexFunc(windows, func(item step) bool { return item.Name == "Run historical release acceptance" })
	if fetchIndex < 0 || historicalIndex <= fetchIndex {
		t.Fatalf("Windows installer fetch order: fetch=%d historical=%d", fetchIndex, historicalIndex)
	}
	fetch, historical := windows[fetchIndex], windows[historicalIndex]
	for _, field := range []struct{ name, got, want string }{
		{"condition", fetch.If, "github.event_name == 'workflow_dispatch' && inputs.windows_clients && inputs.baseline_tag != ''"},
		{"token", fetch.Env["GH_TOKEN"], "${{ github.token }}"},
		{"no prompt", fetch.Env["GH_PROMPT_DISABLED"], "1"},
	} {
		if field.got != field.want {
			t.Fatalf("Hermes installer fetch %s = %q, want %q", field.name, field.got, field.want)
		}
	}
	if fetch.TimeoutMinutes != 2 {
		t.Fatalf("Hermes installer fetch deadline = %d minutes", fetch.TimeoutMinutes)
	}
	const commit = "345cd2b057a452236de401d3534b8502a7465e8d"
	const digest = "226c70a90ad47e8a4d34cb11aca4ecbeb649e2f9b67fbd009ea49791de2d56f5"
	for _, required := range []string{
		"$hermesCommit = '" + commit + "'",
		"gh api \"repos/NousResearch/hermes-agent/contents/scripts/install.ps1?ref=$hermesCommit\"",
		"[Convert]::FromBase64String",
		digest,
	} {
		if !strings.Contains(fetch.Run, required) {
			t.Fatalf("Hermes installer fetch lacks %q", required)
		}
	}
	for _, required := range []string{
		"$hermesCommit = '" + commit + "'",
		"$hermesInstaller = Join-Path $env:RUNNER_TEMP 'aigw-hermes-install.ps1'",
		"Remove-Item -LiteralPath (Join-Path $env:RUNNER_TEMP 'aigw-hermes-install.ps1')",
		digest,
	} {
		if !strings.Contains(historical.Run, required) {
			t.Fatalf("Windows historical acceptance lacks %q", required)
		}
	}
	removeToken := strings.Index(historical.Run, "Remove-Item Env:GH_TOKEN")
	installPackages := strings.Index(historical.Run, "npm install")
	invokeInstaller := strings.Index(historical.Run, "pwsh -NoProfile -File $hermesInstaller")
	if removeToken < 0 || installPackages <= removeToken || invokeInstaller <= removeToken {
		t.Fatal("Windows client installation can inherit GH_TOKEN")
	}
	for _, platform := range []string{"darwin", "linux", "windows"} {
		steps := workflow.Jobs["native-"+platform].Steps
		if platform != "windows" && slices.ContainsFunc(steps, func(item step) bool { return item.Name == fetch.Name }) {
			t.Fatalf("%s has a Windows-only Hermes installer fetch", platform)
		}
		historicalIndex := slices.IndexFunc(steps, func(item step) bool { return item.Name == historical.Name })
		if historicalIndex < 0 || strings.Contains(steps[historicalIndex].Run, "raw.githubusercontent.com/NousResearch/hermes-agent") {
			t.Fatalf("%s historical acceptance lacks a safe installer source", platform)
		}
	}
}
