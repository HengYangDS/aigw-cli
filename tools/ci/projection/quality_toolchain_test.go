package projection

import (
	"path/filepath"
	"slices"
	"strings"
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
	if !slices.Contains(strings.Split(qualityTools, ","), "github:sharkdp/hyperfine") {
		t.Fatal("quality toolchain omits Hyperfine required by its selected performance diagnostics")
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

func TestEverySourceConsumerRetainsDependencyEvidenceOnFailure(t *testing.T) {
	projections, err := renderProjections(filepath.Clean(filepath.Join("..", "..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	var gitlab map[string]yaml.Node
	if err := yaml.Unmarshal([]byte(projections[0].Content), &gitlab); err != nil {
		t.Fatal(err)
	}
	var github struct {
		Jobs map[string]struct {
			Steps []struct {
				If   string            `yaml:"if"`
				Uses string            `yaml:"uses"`
				With map[string]string `yaml:"with"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &github); err != nil {
		t.Fatal(err)
	}
	for _, job := range []string{"quality", "native-darwin", "native-linux", "native-windows"} {
		var declared struct {
			Artifacts struct {
				When  string   `yaml:"when"`
				Paths []string `yaml:"paths"`
			} `yaml:"artifacts"`
		}
		node := gitlab[job]
		if err := node.Decode(&declared); err != nil {
			t.Fatal(err)
		}
		for _, evidence := range []string{"build/verification/dependencies", "build/verification/workflows", "build/verification/coverage"} {
			retained := slices.Contains(declared.Artifacts.Paths, evidence)
			if job != "quality" {
				retained = slices.Contains(declared.Artifacts.Paths, "build/verification")
				if slices.Contains(declared.Artifacts.Paths, evidence) {
					t.Errorf("GitLab %s declares optional child evidence %s as an unconditional artifact", job, evidence)
				}
			}
			if declared.Artifacts.When != "always" || !retained {
				t.Errorf("GitLab %s loses native evidence %s after failure: %+v", job, evidence, declared.Artifacts)
			}
			found := false
			for _, step := range github.Jobs[job].Steps {
				found = found || step.If == "always()" && strings.Contains(step.Uses, "upload-artifact@") && slices.Contains(strings.Fields(step.With["path"]), evidence)
			}
			if !found {
				t.Errorf("GitHub %s loses native evidence %s after failure", job, evidence)
			}
		}
	}
}
