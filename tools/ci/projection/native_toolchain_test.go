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
	var pipeline struct {
		Quality      gitLabJob `yaml:"quality"`
		NativeDarwin gitLabJob `yaml:"native-darwin"`
		NativeLinux  gitLabJob `yaml:"native-linux"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	for name, job := range map[string]gitLabJob{
		"native-darwin": pipeline.NativeDarwin,
		"native-linux":  pipeline.NativeLinux,
	} {
		for tool := range strings.SplitSeq(pipeline.Quality.Variables["MISE_ENABLE_TOOLS"], ",") {
			if !slices.Contains(strings.Split(job.Variables["MISE_ENABLE_TOOLS"], ","), tool) {
				t.Errorf("GitLab %s cannot run repository conformance tests: missing %s", name, tool)
			}
		}
		for _, tool := range []string{"github:goreleaser/goreleaser", "github:anchore/syft"} {
			if !slices.Contains(strings.Split(job.Variables["MISE_ENABLE_TOOLS"], ","), tool) {
				t.Errorf("GitLab %s lacks native release conformance tool %s", name, tool)
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

	var workflow struct {
		Jobs map[string]struct {
			Env   map[string]string `yaml:"env"`
			Steps []struct {
				Run string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	for _, projection := range projections[1:] {
		if err := yaml.Unmarshal([]byte(projection.Content), &workflow); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"native-darwin", "native-linux", "native-windows"} {
			job := workflow.Jobs[name]
			tools := job.Env["MISE_ENABLE_TOOLS"]
			for _, required := range []string{"go,node,npm", "github:goreleaser/goreleaser", "github:anchore/syft", "gh", "inputs.full_quality"} {
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
}

func TestGitHubWindowsARM64UsesThePortableNativeToolClosure(t *testing.T) {
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
	const runner = `${{ github.event_name == 'workflow_dispatch' && inputs.self_hosted_windows_arm64 && !inputs.full_quality && fromJSON('["self-hosted","Windows","ARM64","aigw-github-windows-arm64-parallels-shadow"]') || 'windows-latest' }}`
	if job.RunsOn.Value != runner {
		t.Fatalf("native Windows runner selector = %q, want %q", job.RunsOn.Value, runner)
	}
	tools := job.Env["MISE_ENABLE_TOOLS"]
	for _, required := range []string{"go,node,npm", "github:goreleaser/goreleaser", "github:anchore/syft"} {
		if !strings.Contains(tools, required) {
			t.Errorf("native Windows tool closure lacks %q: %q", required, tools)
		}
	}
	if !strings.Contains(tools, "inputs.full_quality") || !strings.Contains(tools, "github:lycheeverse/lychee") {
		t.Errorf("hosted full-quality closure is not selectable: %q", tools)
	}
	_, defaultTools, ok := strings.Cut(tools, " || '")
	if !ok {
		t.Fatalf("native Windows tool selection has no explicit default: %q", tools)
	}
	for _, unsupported := range []string{"github:indygreg/apple-platform-rs", "github:lycheeverse/lychee"} {
		if strings.Contains(strings.TrimSuffix(defaultTools, "' }}"), unsupported) {
			t.Errorf("Windows ARM64 default tool closure contains unsupported %q: %q", unsupported, tools)
		}
	}
}
