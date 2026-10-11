package projection

import (
	"os"
	"os/exec"
	"path/filepath"
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

func checkGitLabNativeToolClosure(t *testing.T, content string) {
	t.Helper()
	var pipeline struct {
		Quality       gitLabJob  `yaml:"quality"`
		NativeDarwin  gitLabJob  `yaml:"native-darwin"`
		NativeLinux   *gitLabJob `yaml:"native-linux"`
		NativeWindows *gitLabJob `yaml:"native-windows"`
	}
	if err := yaml.Unmarshal([]byte(content), &pipeline); err != nil {
		t.Fatal(err)
	}
	for name, job := range map[string]*gitLabJob{
		"native-darwin": &pipeline.NativeDarwin,
		"native-linux":  pipeline.NativeLinux, "native-windows": pipeline.NativeWindows,
	} {
		if job == nil {
			t.Errorf("GitLab lacks %s", name)
			continue
		}
		for _, tool := range []string{"go", "node", "npm", "github:golangci/golangci-lint", "github:goreleaser/goreleaser", "github:anchore/syft", "github:sharkdp/hyperfine", "gh", "glab"} {
			if !slices.Contains(strings.Split(job.Variables["MISE_ENABLE_TOOLS"], ","), tool) {
				t.Errorf("GitLab %s lacks native acceptance tool %s", name, tool)
			}
		}
		hasDarwinSigner := slices.Contains(strings.Split(job.Variables["MISE_ENABLE_TOOLS"], ","), "github:indygreg/apple-platform-rs")
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
				If  string `yaml:"if"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projection.Content), &workflow); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"native-darwin", "native-linux", "native-windows"} {
		job := workflow.Jobs[name]
		tools := job.Env["MISE_ENABLE_TOOLS"]
		for _, required := range []string{"go,node,npm", "github:golangci/golangci-lint", "github:goreleaser/goreleaser", "github:anchore/syft", "github:sharkdp/hyperfine", "gh", "inputs.full_quality"} {
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
			if step.Run == "mise run bootstrap" && !strings.Contains(step.If, "inputs.full_quality") {
				t.Errorf("%s %s full quality cannot prepare its locked repository dependencies", projection.Path, name)
			}
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
	const runner = "windows-latest"
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

func TestWindowsCheckoutReceivesNativeLongPathConfiguration(t *testing.T) {
	projections, err := renderProjections(filepath.Clean(filepath.Join("..", "..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	var gitlab struct {
		NativeWindows gitLabJob `yaml:"native-windows"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	var github struct {
		Env map[string]string `yaml:"env"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &github); err != nil {
		t.Fatal(err)
	}
	for peer, variables := range map[string]map[string]string{
		"GitLab": gitlab.NativeWindows.Variables, "GitHub": github.Env,
	} {
		for _, field := range []struct{ name, value string }{
			{"GIT_CONFIG_COUNT", "2"},
			{"GIT_CONFIG_KEY_0", "init.defaultBranch"},
			{"GIT_CONFIG_VALUE_0", "main"},
			{"GIT_CONFIG_KEY_1", "core.longpaths"},
			{"GIT_CONFIG_VALUE_1", "true"},
		} {
			if value := variables[field.name]; value != field.value {
				t.Errorf("%s checkout environment %s = %q, want %q", peer, field.name, value, field.value)
			}
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
		if platform != "windows" {
			if historicalIndex >= 0 {
				t.Fatalf("%s has Windows-only client provisioning", platform)
			}
			continue
		}
		if historicalIndex < 0 || strings.Contains(steps[historicalIndex].Run, "raw.githubusercontent.com/NousResearch/hermes-agent") {
			t.Fatalf("%s historical acceptance lacks a safe installer source", platform)
		}
	}
}

func TestNativeInputProjectionsInvokeTheOwnedCandidateOnce(t *testing.T) {
	projections, err := renderProjections(filepath.Clean(filepath.Join("..", "..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	var github struct {
		Jobs map[string]struct {
			If    string `yaml:"if"`
			Steps []struct {
				Name string            `yaml:"name"`
				If   string            `yaml:"if"`
				Run  string            `yaml:"run"`
				Env  map[string]string `yaml:"env"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &github); err != nil {
		t.Fatal(err)
	}
	releaseTagGuard := ""
	for _, step := range github.Jobs["quality"].Steps {
		if step.Name == "Verify pushed release tag provenance" {
			releaseTagGuard = step.If
		}
	}
	if releaseTagGuard != "github.ref_type == 'tag' && startsWith(github.ref_name, 'v')" {
		t.Fatal("input transport tags must retain native carrier verification without release-version classification")
	}
	for _, platform := range []string{"darwin", "linux", "windows"} {
		condition := github.Jobs["native-"+platform].If
		selection := " && (github.event_name != 'workflow_dispatch' || inputs.native_platform == '' || inputs.native_platform == 'all' || inputs.native_platform == '" + platform + "')"
		if !strings.HasSuffix(condition, selection) {
			t.Fatalf("manual native acceptance ignores the selected %s platform: %s", platform, condition)
		}
	}
	platform := runtime.GOOS
	steps := github.Jobs["native-"+platform].Steps
	index := slices.IndexFunc(steps, func(step struct {
		Name string            `yaml:"name"`
		If   string            `yaml:"if"`
		Run  string            `yaml:"run"`
		Env  map[string]string `yaml:"env"`
	}) bool {
		return step.Name == "Run selected native artifact acceptance"
	})
	if index < 0 {
		t.Fatal("CUE lacks a prebuilt native candidate entry")
	}
	step := steps[index]
	if step.Env["AIGW_NATIVE_INPUT_RELEASE"] != "${{ inputs.input_release }}" || step.Env["AIGW_CANDIDATE_SOURCE"] != "${{ inputs.candidate_source }}" || step.Env["AIGW_NATIVE_INPUT_SHA256"] != "${{ inputs.input_sha256 }}" {
		t.Fatal("prebuilt native candidate entry lost its exact source and byte identity")
	}
	output, err := projectionCommand(filepath.Clean(filepath.Join("..", "..", "..")), "nativeInputCommands.github."+platform).Output()
	if err != nil {
		t.Fatal(err)
	}
	var invocation string
	if err := yaml.Unmarshal(output, &invocation); err != nil || !strings.Contains(step.Run, invocation) {
		t.Fatalf("native artifact step is not bound to its owning command: %v", err)
	}
	result, bin := filepath.Join(t.TempDir(), "native-call"), t.TempDir()
	source := filepath.Join(bin, "capture.go")
	if err := os.WriteFile(source, []byte("package main\nimport (\"os\"; \"strings\")\nfunc main() { if err := os.WriteFile(os.Getenv(\"AIGW_TEST_CALL\"), []byte(strings.Join(os.Args[1:], \"\\n\")), 0600); err != nil { panic(err) } }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(bin, "mise")
	if platform == "windows" {
		executable += ".exe"
	}
	if output, err := exec.CommandContext(t.Context(), "go", "build", "-o", executable, source).CombinedOutput(); err != nil {
		t.Fatalf("build native command-capture fixture: %v: %s", err, output)
	}
	for _, selection := range []struct{ name, quality, candidate, source, carrier, checksum string }{
		{"candidate", "false", "true", strings.Repeat("a", 40), "native-inputs-" + strings.Repeat("b", 40), strings.Repeat("c", 64)},
		{"full-quality candidate", "true", "true", strings.Repeat("a", 40), "native-inputs-" + strings.Repeat("b", 40), strings.Repeat("c", 64)},
		{"source with published baseline", "false", "false", "", "", ""},
	} {
		t.Run(selection.name, func(t *testing.T) {
			command := exec.CommandContext(t.Context(), "bash", "-euo", "pipefail", "-c", invocation)
			if platform == "windows" {
				command = exec.CommandContext(t.Context(), "pwsh", "-NoProfile", "-Command", invocation)
			}
			command.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "AIGW_TEST_CALL="+result,
				"AIGW_CANDIDATE_SOURCE="+selection.source, "AIGW_NATIVE_INPUT_RELEASE="+selection.carrier,
				"AIGW_NATIVE_INPUT_SHA256="+selection.checksum, "AIGW_BASELINE_TAG=v0.3.3", "GITHUB_REPOSITORY=group/product",
				"AIGW_FULL_NATIVE_QUALITY="+selection.quality, "AIGW_NATIVE_CANDIDATE="+selection.candidate, "AIGW_NATIVE_CLIENTS=false", "AIGW_CANDIDATE_TAG=", "AIGW_PERFORMANCE_OUTPUT=")
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("projected native selection failed: %v: %s", err, output)
			}
			data, err := os.ReadFile(result)
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"exec", "--locked", "--", "go", "run", "./tools/ci", "native", "--platform", platform, "--full-quality=" + selection.quality, "--", "--candidate=" + selection.candidate, "--candidate-source=" + selection.source,
				"--input-release=" + selection.carrier, "--input-sha256=" + selection.checksum, "--baseline-tag=v0.3.3", "--tag=", "--peer=github", "--repository=group/product", "--clients=false", "--performance="}
			if !slices.Equal(strings.Split(string(data), "\n"), want) {
				t.Fatalf("CUE projected a different native selection: %s", data)
			}
		})
	}
}
