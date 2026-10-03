package projection

import (
	"path/filepath"
	"slices"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestQualityJobsUseTheirExactToolClosure(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	projections, err := renderProjections(root)
	if err != nil {
		t.Fatal(err)
	}
	var pipeline struct {
		Quality gitLabJob `yaml:"quality"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	qualityTools := pipeline.Quality.Variables["MISE_ENABLE_TOOLS"]
	if qualityTools == "" {
		t.Fatal("GitLab quality job must declare its native toolchain")
	}
	if !slices.Equal(pipeline.Quality.Tags, []string{"$AIGW_CI_LINUX_RUNNER_TAG"}) {
		t.Fatalf("GitLab quality runner tags = %q", pipeline.Quality.Tags)
	}
	if !slices.Equal(pipeline.Quality.Extends, []string{".linux-toolchain"}) || len(pipeline.Quality.Script) < 1 || pipeline.Quality.Script[0] != "mise run bootstrap" {
		t.Fatalf("GitLab quality job lacks locked dependency preparation: %q", pipeline.Quality.Script)
	}
	var github struct {
		Jobs map[string]struct {
			Env   map[string]string `yaml:"env"`
			Steps []struct {
				Name string `yaml:"name"`
				Run  string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &github); err != nil {
		t.Fatal(err)
	}
	if got := github.Jobs["quality"].Env["MISE_ENABLE_TOOLS"]; got != qualityTools {
		t.Fatalf("GitHub quality tools = %q, want %q", got, qualityTools)
	}
	steps := github.Jobs["quality"].Steps
	if !slices.ContainsFunc(steps, func(step struct {
		Name string `yaml:"name"`
		Run  string `yaml:"run"`
	}) bool {
		return step.Name == "Prepare locked dependencies" && step.Run == "mise run bootstrap"
	}) {
		t.Fatalf("GitHub quality job lacks locked dependency preparation: %#v", steps)
	}
}
