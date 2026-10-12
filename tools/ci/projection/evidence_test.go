package projection

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestNativeCoverageSurvivesJobFailureOnEachForge(t *testing.T) {
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
				If   string `yaml:"if"`
				Uses string `yaml:"uses"`
				With struct {
					Name           string `yaml:"name"`
					Path           string `yaml:"path"`
					IfNoFilesFound string `yaml:"if-no-files-found"`
				} `yaml:"with"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(projections[1].Content), &github); err != nil {
		t.Fatal(err)
	}
	for _, platform := range []string{"darwin", "linux", "windows"} {
		t.Run(platform, func(t *testing.T) {
			job := "native-" + platform
			profile := "build/acceptance/coverage-" + platform + ".out"
			var nativeJob struct {
				Artifacts struct {
					When  string   `yaml:"when"`
					Paths []string `yaml:"paths"`
				} `yaml:"artifacts"`
			}
			node := gitlab[job]
			if err := node.Decode(&nativeJob); err != nil {
				t.Fatal(err)
			}
			artifacts := nativeJob.Artifacts
			if artifacts.When != "always" || !slices.Contains(artifacts.Paths, profile) {
				t.Errorf("GitLab %s must retain the original coverage profile after failure: %#v", job, artifacts)
			}
			for _, step := range github.Jobs[job].Steps {
				if !slices.Contains(strings.Fields(step.With.Path), profile) {
					continue
				}
				if step.If != "always()" || !strings.HasPrefix(step.Uses, "actions/upload-artifact@") || step.With.Name != "native-coverage-"+platform || step.With.IfNoFilesFound != "warn" {
					t.Fatalf("GitHub %s must retain coverage without masking an earlier failure: %#v", job, step)
				}
				return
			}
			t.Errorf("GitHub %s does not retain the original coverage profile", job)
		})
	}
}
