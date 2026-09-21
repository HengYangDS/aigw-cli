package projection

import (
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestGitHubLinuxNativeHasAnExplicitSelfHostedARM64OptIn(t *testing.T) {
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

	input, ok := workflow.On.Dispatch.Inputs["self_hosted_linux_arm64"]
	if !ok {
		t.Fatal("GitHub Verify lacks the explicit self-hosted Linux ARM64 opt-in")
	}
	if input.Type != "boolean" || input.Default.Value != "false" || !strings.Contains(input.Description, "Native Linux") {
		t.Fatalf("self-hosted Linux ARM64 input = %#v", input)
	}

	const selector = `${{ github.event_name == 'workflow_dispatch' && inputs.self_hosted_linux_arm64 && fromJSON('["self-hosted","Linux","ARM64","aigw-github-linux-arm64-parallels-shadow"]') || 'ubuntu-latest' }}`
	if got := workflow.Jobs["native-linux"].RunsOn.Value; got != selector {
		t.Fatalf("native Linux runner selector = %q, want %q", got, selector)
	}
	for _, name := range []string{"accepted-ref-parity", "quality"} {
		if got := workflow.Jobs[name].RunsOn.Value; got != "ubuntu-latest" {
			t.Fatalf("%s runner = %q, want hosted default", name, got)
		}
	}
}
