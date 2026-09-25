package projection

import (
	"path/filepath"
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
		for _, tool := range []string{"go", "node", "npm", "github:golangci/golangci-lint", "github:goreleaser/goreleaser", "github:anchore/syft", "gh", "glab"} {
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
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projection.Content), &workflow); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"native-darwin", "native-linux", "native-windows"} {
		job := workflow.Jobs[name]
		tools := job.Env["MISE_ENABLE_TOOLS"]
		for _, required := range []string{"go,node,npm", "github:golangci/golangci-lint", "github:goreleaser/goreleaser", "github:anchore/syft", "gh", "inputs.full_quality"} {
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
