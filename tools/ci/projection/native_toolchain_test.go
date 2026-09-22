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
		Quality       gitLabJob `yaml:"quality"`
		NativeDarwin  gitLabJob `yaml:"native-darwin"`
		NativeLinux   gitLabJob `yaml:"native-linux"`
		NativeWindows gitLabJob `yaml:"native-windows"`
	}
	if err := yaml.Unmarshal([]byte(content), &pipeline); err != nil {
		t.Fatal(err)
	}
	for name, job := range map[string]gitLabJob{
		"native-darwin":  pipeline.NativeDarwin,
		"native-linux":   pipeline.NativeLinux,
		"native-windows": pipeline.NativeWindows,
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
