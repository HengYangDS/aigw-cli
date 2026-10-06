package projection

import (
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
	if len(gitlab.SecretService.Script) != 2 || !strings.Contains(gitlab.SecretService.Script[0], "TestNativeProductJourney/system_credential_store") ||
		!strings.Contains(gitlab.SecretService.Script[1], "Mise job-owned supply state retired.") {
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

func TestNativePrebuiltAcceptanceKeepsBootstrapConditional(t *testing.T) {
	projections, err := renderProjections(filepath.Clean(filepath.Join("..", "..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	var gitlab struct {
		Linux   gitLabJob `yaml:"native-linux"`
		Darwin  gitLabJob `yaml:"native-darwin-review"`
		Windows gitLabJob `yaml:"native-windows-review"`
		Quality gitLabJob `yaml:"quality"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	manualQuality := gitlab.Quality.Rules[len(gitlab.Quality.Rules)-2].If
	for _, required := range []string{"AIGW_CANDIDATE_ARTIFACTS", "AIGW_CANDIDATE_TAG", "AIGW_FULL_NATIVE_QUALITY", "AIGW_REFRESH_LOCKS"} {
		if !strings.Contains(manualQuality, required) {
			t.Errorf("manual quality admission ignores artifact scope %s", required)
		}
	}
	for name, job := range map[string]gitLabJob{"native-linux": gitlab.Linux, "native-darwin-review": gitlab.Darwin, "native-windows-review": gitlab.Windows} {
		for _, script := range job.Script {
			if script == "mise run bootstrap" {
				t.Errorf("%s installs the source toolchain unconditionally before prebuilt acceptance", name)
			}
		}
		joined := strings.Join(append(slices.Clone(job.BeforeScript), job.Script...), "\n")
		if !strings.Contains(joined, "AIGW_CANDIDATE_ARTIFACTS") || !strings.Contains(joined, "go,gh,glab,github:goreleaser/goreleaser") {
			t.Errorf("%s lacks the explicit prebuilt tool closure", name)
		}
	}
	var github struct {
		Jobs map[string]struct {
			If    string `yaml:"if"`
			Steps []struct {
				Run string `yaml:"run"`
				If  string `yaml:"if"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &github); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(github.Jobs["quality"].If, "inputs.candidate_tag") || !strings.Contains(github.Jobs["quality"].If, "inputs.full_quality") {
		t.Fatal("manual prebuilt acceptance still installs the quality toolchain")
	}
	for _, name := range []string{"native-linux", "native-darwin", "native-windows"} {
		for _, step := range github.Jobs[name].Steps {
			if step.Run == "mise run bootstrap" && !strings.Contains(step.If, "inputs.candidate_tag") {
				t.Errorf("%s does not scope source bootstrap away from explicit prebuilt inputs", name)
			}
		}
	}
}

func TestNativeArtifactBootstrapExecutesTheDeclaredScope(t *testing.T) {
	if runtime.GOOS == "windows" {
		return // POSIX projection execution is qualified on Unix hosts.
	}
	projections, err := renderProjections(filepath.Clean(filepath.Join("..", "..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	var pipeline struct {
		Linux gitLabJob `yaml:"native-linux"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	selectTools := pipeline.Linux.BeforeScript[len(pipeline.Linux.BeforeScript)-2]
	bootstrap := pipeline.Linux.Script[0]
	for _, test := range []struct {
		name, artifacts, tag, full, refresh, performance string
		tools                                            string
	}{
		{"candidate", "/candidate with spaces", "", "false", "false", "false", "go,gh,glab,github:goreleaser/goreleaser"},
		{"tag", "", "v0.3.1", "false", "false", "false", "go,gh,glab,github:goreleaser/goreleaser"},
		{"source", "", "", "false", "false", "false", "source-tools"},
		{"full quality", "/candidate", "", "true", "false", "false", "source-tools"},
		{"lock refresh", "/candidate", "", "false", "true", "false", "source-tools"},
		{"candidate performance", "/candidate", "", "false", "false", "true", "go,gh,glab,github:goreleaser/goreleaser,github:sharkdp/hyperfine"},
		{"tag performance", "", "v0.3.1", "false", "false", "true", "go,gh,glab,github:goreleaser/goreleaser,github:sharkdp/hyperfine"},
	} {
		t.Run(test.name, func(t *testing.T) {
			command := exec.CommandContext(t.Context(), "sh", "-c", "set -eu\nmise() { printf 'BOOTSTRAP\\n'; }\n"+selectTools+"\n"+bootstrap+"\nprintf 'TOOLS=%s\\n' \"$MISE_ENABLE_TOOLS\"")
			command.Env = append(os.Environ(),
				"MISE_ENABLE_TOOLS=source-tools", "AIGW_CANDIDATE_ARTIFACTS="+test.artifacts,
				"AIGW_CANDIDATE_TAG="+test.tag, "AIGW_FULL_NATIVE_QUALITY="+test.full, "AIGW_REFRESH_LOCKS="+test.refresh,
				"AIGW_NATIVE_PERFORMANCE="+test.performance)
			output, err := command.CombinedOutput()
			if err != nil || strings.Contains(string(output), "BOOTSTRAP") != (test.tools == "source-tools") || !strings.HasSuffix(string(output), "TOOLS="+test.tools+"\n") {
				t.Fatalf("native tool scope: %v, %s", err, output)
			}
		})
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
	prepared := false
	for _, step := range workflow.Jobs["native-linux"].Steps {
		if step.Name == "Prepare native Secret Service" {
			prepared = strings.Contains(step.Run, "dbus-x11 gnome-keyring libglib2.0-bin")
		}
		if step.Name == "Qualify Linux Secret Service" {
			t.Fatal("native Linux job duplicates the Secret Service qualification")
		}
	}
	if !prepared {
		t.Fatal("Linux source coverage lacks its native Secret Service preparation")
	}
	secretService, present := workflow.Jobs["linux-secret-service"]
	if !present || len(secretService.Steps) != 3 || secretService.Steps[2].Name != "Qualify Linux Secret Service" {
		t.Fatal("GitHub requires independent native Linux Secret Service qualification")
	}
	githubQualification := secretService.Steps[2].Run
	if strings.Contains(githubQualification, "./internal/secrets/native") {
		t.Fatal("standalone Secret Service job repeats source-level credential qualification")
	}
	for _, required := range []string{
		"dbus-x11 gnome-keyring",
		"sudo -n timeout --verbose --kill-after=5s 240s",
		"Acquire::http::Timeout=30",
		"dbus-run-session",
		"SetAlias default /org/freedesktop/secrets/collection/session",
		"AIGW_VERIFY_SYSTEM_KEYRING=1",
		"TestNativeProductJourney/system_credential_store",
		"AIGW_VERIFY_SYSTEM_KEYRING=1 mise exec --locked -- go test ./tools/release -run \"^TestNativeProductJourney/system_credential_store$\" -count=1 -v\nAIGW_SECRET_SERVICE",
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
	if strings.Contains(strings.Join(gitlab.Linux.Script, "\n"), "TestNativeProductJourney/system_credential_store") {
		t.Fatal("GitLab native Linux job duplicates Secret Service qualification")
	}
	if !slices.Equal(gitlab.SecretService.Extends, []string{".linux-toolchain"}) ||
		!slices.Equal(gitlab.SecretService.Tags, gitlab.Linux.Tags) ||
		!reflect.DeepEqual(gitlab.SecretService.Rules, gitlab.Linux.Rules) {
		t.Fatal("GitLab Secret Service job must use the same Linux runner and event admission")
	}
	if len(gitlab.SecretService.Script) != 2 {
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

func TestLinuxCompilerPrerequisitesBelongOnlyToNativeRaceExecution(t *testing.T) {
	projections, err := renderProjections(filepath.Clean(filepath.Join("..", "..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	var pipeline struct {
		Toolchain     gitLabJob `yaml:".linux-toolchain"`
		Quality       gitLabJob `yaml:"quality"`
		SecretService gitLabJob `yaml:"linux-secret-service"`
		NativeLinux   gitLabJob `yaml:"native-linux"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	for name, job := range map[string]gitLabJob{
		"quality": pipeline.Quality, "linux-secret-service": pipeline.SecretService,
	} {
		if job.Variables["CGO_ENABLED"] != "0" {
			t.Errorf("%s adds a compiler without executing cgo or race", name)
		}
	}
	shared := strings.Join(pipeline.Toolchain.BeforeScript, "\n")
	for _, compiler := range []string{" gcc ", " libc6-dev "} {
		if strings.Contains(shared, compiler) {
			t.Errorf("shared bootstrap downloads native-race prerequisite %q", compiler)
		}
	}
	if pipeline.NativeLinux.Variables["CGO_ENABLED"] != "1" || len(pipeline.NativeLinux.BeforeScript) < 2 {
		t.Fatal("native Linux race lacks its explicit compiler prerequisite")
	}
	compiler := pipeline.NativeLinux.BeforeScript[1]
	if !strings.Contains(compiler, " install --no-install-recommends -y gcc libc6-dev") {
		t.Fatalf("native Linux compiler preparation = %q", compiler)
	}
	var github struct {
		Jobs map[string]struct {
			Env   map[string]string `yaml:"env"`
			Steps []struct {
				Name string            `yaml:"name"`
				Env  map[string]string `yaml:"env"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &github); err != nil {
		t.Fatal(err)
	}
	if github.Jobs["linux-secret-service"].Env["CGO_ENABLED"] != "0" {
		t.Fatal("GitHub adds cgo to the pure-Go Secret Service journey")
	}
	for _, step := range github.Jobs["quality"].Steps {
		if step.Name == "Run quality and governance" && step.Env["CGO_ENABLED"] != "0" {
			t.Fatal("GitHub static quality adds a compiler without running race")
		}
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
		for _, tool := range []string{"go", "node", "npm", "github:golangci/golangci-lint", "github:goreleaser/goreleaser", "github:anchore/syft", "github:lycheeverse/lychee", "gh", "glab"} {
			if !slices.Contains(enabled, tool) {
				t.Errorf("GitLab %s lacks native acceptance tool %s", name, tool)
			}
		}
		hasDarwinSigner := slices.Contains(enabled, "github:indygreg/apple-platform-rs")
		if hasDarwinSigner != (name == "native-darwin") {
			t.Errorf("GitLab %s Darwin signer presence = %t", name, hasDarwinSigner)
		}
		bootstrap := slices.IndexFunc(job.Script, func(value string) bool { return strings.Contains(value, "mise run bootstrap") })
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
		for _, required := range []string{"go,node,npm", "github:golangci/golangci-lint", "github:goreleaser/goreleaser", "github:anchore/syft", "github:lycheeverse/lychee", "gh", "glab"} {
			if !strings.Contains(tools, required) {
				t.Errorf("%s %s tool closure lacks %q: %q", projection.Path, name, required, tools)
			}
		}
		hasDarwinSigner := strings.Contains(tools, "github:indygreg/apple-platform-rs")
		if hasDarwinSigner != (name == "native-darwin") {
			t.Errorf("%s %s Darwin signer presence = %t", projection.Path, name, hasDarwinSigner)
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
	for _, required := range []string{"go", "node", "npm", "github:golangci/golangci-lint", "github:goreleaser/goreleaser", "github:anchore/syft", "github:lycheeverse/lychee"} {
		if !strings.Contains(tools, required) {
			t.Errorf("native Windows tool closure lacks %q: %q", required, tools)
		}
	}
	if strings.Contains(tools, "github:indygreg/apple-platform-rs") {
		t.Errorf("Windows tool closure contains the macOS signer: %q", tools)
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
	supplyIndex := slices.IndexFunc(windows, func(item step) bool { return item.Name == "Prepare official Windows clients" })
	nativeIndex := slices.IndexFunc(windows, func(item step) bool { return item.Name == "Run historical release acceptance" })
	cleanupIndex := slices.IndexFunc(windows, func(item step) bool { return item.Name == "Remove official Windows client supply" })
	if fetchIndex < 0 || supplyIndex <= fetchIndex || nativeIndex <= supplyIndex || cleanupIndex <= nativeIndex {
		t.Fatalf("Windows native supply ordering: fetch=%d supply=%d native=%d cleanup=%d", fetchIndex, supplyIndex, nativeIndex, cleanupIndex)
	}
	fetch, supply := windows[fetchIndex], windows[supplyIndex]
	const selection = "github.event_name == 'workflow_dispatch' && inputs.windows_clients && inputs.baseline_tag != ''"
	for _, field := range []struct{ got, want string }{
		{fetch.If, selection}, {supply.If, selection}, {windows[cleanupIndex].If, "always() && " + selection},
		{fetch.Env["GH_TOKEN"], "${{ github.token }}"}, {fetch.Env["GH_PROMPT_DISABLED"], "1"},
	} {
		if field.got != field.want {
			t.Fatalf("Windows official supply input = %q, want %q", field.got, field.want)
		}
	}
	if fetch.TimeoutMinutes != 2 || supply.TimeoutMinutes != 12 {
		t.Fatal("Windows official supply lost its caller deadlines")
	}
	const commit = "f97608f178d1ffeca59860195ab7da295f7c8e5f"
	const digest = "0a80dfeb7434229933bac32e73140d10086dff81bd84b156e71be9abc87cddf2"
	if !strings.Contains(fetch.Run, commit) || !strings.Contains(fetch.Run, digest) || !strings.Contains(fetch.Run, "gh api") || !strings.Contains(fetch.Run, "[Convert]::FromBase64String") {
		t.Fatal("Hermes installer must preserve original pinned Git blob bytes")
	}
	for _, key := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if value, present := supply.Env[key]; !present || value != "" {
			t.Fatalf("Windows official installation inherits %s", key)
		}
	}
	for _, required := range []string{commit, digest, "npm install", "npm audit signatures", "pwsh -NoProfile -File $hermesInstaller", "hash-verified via uv.lock", "AIGW_ACCEPTANCE_CODEX", "AIGW_ACCEPTANCE_CLAUDE", "AIGW_ACCEPTANCE_HERMES", "AIGW_ACCEPTANCE_CLIENT_PATH", "CLAUDE_CODE_GIT_BASH_PATH", "GITHUB_ENV", "$env:GIT_CONFIG_GLOBAL = Join-Path $clients 'gitconfig'", "git config --file $env:GIT_CONFIG_GLOBAL core.autocrlf false", "git -C $hermesInstall status --porcelain=v1 --untracked-files=no"} {
		if !strings.Contains(supply.Run, required) {
			t.Fatalf("Windows native supply lost %q", required)
		}
	}
	for _, platform := range []string{"darwin", "linux"} {
		if slices.ContainsFunc(workflow.Jobs["native-"+platform].Steps, func(item step) bool {
			return item.Name == fetch.Name || item.Name == supply.Name || item.Name == windows[cleanupIndex].Name
		}) {
			t.Fatalf("%s contains Windows-only supply orchestration", platform)
		}
	}
}

func TestGitLabNativeAcceptanceForwardsPeerLocalArtifactAndClientInputs(t *testing.T) {
	projections, err := renderProjections(filepath.Clean(filepath.Join("..", "..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	var jobs struct {
		Linux   gitLabJob `yaml:"native-linux"`
		Darwin  gitLabJob `yaml:"native-darwin-review"`
		Windows gitLabJob `yaml:"native-windows-review"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &jobs); err != nil {
		t.Fatal(err)
	}
	for name, job := range map[string]gitLabJob{"native-linux": jobs.Linux, "native-darwin-review": jobs.Darwin, "native-windows-review": jobs.Windows} {
		if job.Variables["GLAB_ENABLE_CI_AUTOLOGIN"] != "" {
			t.Fatal("CI login must be scoped to native downloads, not the source or test environment")
		}
		script := strings.Join(job.Script, "\n")
		for _, input := range []string{"AIGW_BASELINE_TAG", "AIGW_CANDIDATE_TAG", "AIGW_CANDIDATE_ARTIFACTS", "AIGW_CANDIDATE_SOURCE", "AIGW_NATIVE_CLIENTS", "AIGW_NATIVE_DIAGNOSTIC_CLIENT", "--baseline-tag", "--artifacts", "--candidate", "--candidate-source", "--clients", "--diagnostic-client", "--peer", "gitlab", "--repository", "CI_PROJECT_URL"} {
			if !strings.Contains(script, input) {
				t.Errorf("%s omits native release input %s", name, input)
			}
		}
		if strings.Contains(script, "gh release download") || !strings.Contains(script, "native --platform") {
			t.Errorf("%s must use its own peer and the existing native controller", name)
		}
		if !strings.Contains(script, "Native diagnostics and performance require a manual pipeline") {
			t.Errorf("%s does not fence diagnostics from required review and release jobs", name)
		}
	}
}
