package projection

import (
	"path/filepath"
	"strings"
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
		"native-darwin":       "macos-26-intel",
		"native-linux":        "ubuntu-latest",
		"native-windows":      "windows-latest",
	} {
		if got := workflow.Jobs[name].RunsOn.Value; got != runner {
			t.Errorf("%s runner = %q, want %q", name, got, runner)
		}
	}
}

func TestForgeProjectionsFollowDeclaredNativeCapacity(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var gitlab struct {
		NativeDarwin  *gitLabJob `yaml:"native-darwin"`
		NativeLinux   *gitLabJob `yaml:"native-linux"`
		NativeWindows *gitLabJob `yaml:"native-windows"`
		Assets        gitLabJob  `yaml:"release-assets"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	if gitlab.NativeDarwin == nil {
		t.Fatal("GitLab projection lacks declared native Darwin capacity")
	}
	if gitlab.NativeLinux != nil || gitlab.NativeWindows != nil {
		t.Fatal("GitLab projection advertises native capacity that is not qualified")
	}
	if strings.Contains(projections[0].Content, "allow_failure:") {
		t.Fatal("GitLab projection weakens a native job with allow_failure")
	}
	for _, need := range gitlab.Assets.Needs {
		if need.Job == "native-linux" || need.Job == "native-windows" {
			t.Fatal("GitLab release assets depend on undeclared native capacity")
		}
	}

	for _, projectionIndex := range []int{1} {
		var github struct {
			Jobs map[string]any `yaml:"jobs"`
		}
		if err := yaml.Unmarshal([]byte(projections[projectionIndex].Content), &github); err != nil {
			t.Fatal(err)
		}
		if _, present := github.Jobs["native-windows"]; !present {
			t.Fatalf("GitHub projection %d lacks required native Windows evidence", projectionIndex)
		}
	}
}
