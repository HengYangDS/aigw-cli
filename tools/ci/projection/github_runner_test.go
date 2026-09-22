package projection

import (
	"path/filepath"
	"testing"

	"go.yaml.in/yaml/v3"
)

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
		"accepted-ref-parity": "ubuntu-latest",
		"quality":             "ubuntu-latest",
		"native-darwin":       "macos-latest",
		"native-linux":        "ubuntu-latest",
		"native-windows":      "windows-latest",
	} {
		if got := workflow.Jobs[name].RunsOn.Value; got != runner {
			t.Errorf("%s runner = %q, want %q", name, got, runner)
		}
	}
}
