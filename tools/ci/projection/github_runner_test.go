package projection

import (
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestGitHubNativeJobsHaveExplicitSelfHostedARM64OptIns(t *testing.T) {
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

	for _, platform := range []struct {
		id         string
		label      string
		hosted     string
		selfHosted string
	}{
		{id: "linux", label: "Linux", hosted: "ubuntu-latest", selfHosted: "aigw-github-linux-arm64-parallels-shadow"},
		{id: "windows", label: "Windows", hosted: "windows-latest", selfHosted: "aigw-github-windows-arm64-parallels-shadow"},
	} {
		inputName := "self_hosted_" + platform.id + "_arm64"
		input, ok := workflow.On.Dispatch.Inputs[inputName]
		if !ok {
			t.Errorf("GitHub Verify lacks the explicit self-hosted %s ARM64 opt-in", platform.label)
			continue
		}
		if input.Type != "boolean" || input.Default.Value != "false" || !strings.Contains(input.Description, "Native "+platform.label) {
			t.Errorf("self-hosted %s ARM64 input = %#v", platform.label, input)
		}

		fullQualityGuard := ""
		if platform.id == "windows" {
			fullQualityGuard = " && !inputs.full_quality"
		}
		selector := `${{ github.event_name == 'workflow_dispatch' && inputs.` + inputName + fullQualityGuard + ` && fromJSON('["self-hosted","` + platform.label + `","ARM64","` + platform.selfHosted + `"]') || '` + platform.hosted + `' }}`
		if got := workflow.Jobs["native-"+platform.id].RunsOn.Value; got != selector {
			t.Errorf("native %s runner selector = %q, want %q", platform.label, got, selector)
		}
	}
	for _, name := range []string{"accepted-ref-parity", "quality"} {
		if got := workflow.Jobs[name].RunsOn.Value; got != "ubuntu-latest" {
			t.Fatalf("%s runner = %q, want hosted default", name, got)
		}
	}
}
