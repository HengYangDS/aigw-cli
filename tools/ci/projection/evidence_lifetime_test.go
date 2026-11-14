package projection

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// The CI model declares the lifetime of retained job evidence once, as a whole
// number of days. Every GitHub step that uploads an artifact carries it as its
// retention, and every GitLab job that declares artifacts as its expiry: an
// upload that forgets it would keep its artifact for the Forge's own default.
func TestEveryRetainedArtifactCarriesTheDeclaredLifetime(t *testing.T) {
	model, err := os.ReadFile(filepath.Join("..", "..", "..", ".config", "ci", "pipeline.cue"))
	if err != nil {
		t.Fatal(err)
	}
	declared := regexp.MustCompile(`(?m)^evidenceRetentionDays:\s+([1-9][0-9]*)$`).FindAllSubmatch(model, -1)
	if len(declared) != 1 {
		t.Fatalf("the CI model declares the evidence lifetime %d times, want once as a positive number of days", len(declared))
	}
	days := string(declared[0][1])
	projections := canonicalProjections(t)
	uploads, declarations, problems := evidenceLifetimeProblems(t, projections[0].Content, projections[1].Content, days)
	if uploads == 0 || declarations == 0 {
		t.Fatalf("found %d GitHub uploads and %d GitLab artifact declarations; the test reads neither", uploads, declarations)
	}
	for _, problem := range problems {
		t.Error(problem)
	}

	// The check itself: an upload without the field, another number and an
	// artifact declaration without an expiry are each a problem.
	for name, mutate := range map[string]func(gitlab, github string) (string, string){
		"GitHub upload without retention": func(gitlab, github string) (string, string) {
			return gitlab, strings.Replace(github, "retention-days: "+days, "compression-level: 6", 1)
		},
		"GitHub upload with another number": func(gitlab, github string) (string, string) {
			return gitlab, strings.Replace(github, "retention-days: "+days, "retention-days: 90", 1)
		},
		"GitLab artifacts without expiry": func(gitlab, github string) (string, string) {
			return strings.Replace(gitlab, "expire_in: "+days+" days", "untracked: false", 1), github
		},
		"GitLab artifacts with another expiry": func(gitlab, github string) (string, string) {
			return strings.Replace(gitlab, "expire_in: "+days+" days", "expire_in: never", 1), github
		},
	} {
		gitlab, github := mutate(projections[0].Content, projections[1].Content)
		if _, _, found := evidenceLifetimeProblems(t, gitlab, github, days); len(found) != 1 {
			t.Errorf("%s: the test reports %d problems, want 1: %v", name, len(found), found)
		}
	}
}

// evidenceLifetimeProblems counts the GitHub upload steps and the GitLab
// artifact declarations of the two projections, and names each one that does
// not carry days.
func evidenceLifetimeProblems(t *testing.T, gitlab, github, days string) (uploads, declarations int, problems []string) {
	t.Helper()
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				Name string            `yaml:"name"`
				Uses string            `yaml:"uses"`
				With map[string]string `yaml:"with"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(github), &workflow); err != nil {
		t.Fatal(err)
	}
	for job, declared := range workflow.Jobs {
		for _, step := range declared.Steps {
			if !strings.Contains(step.Uses, "upload-artifact@") {
				continue
			}
			uploads++
			if got := step.With["retention-days"]; got != days {
				problems = append(problems, fmt.Sprintf("GitHub %s step %q keeps its artifact for %q days, want %s", job, step.Name, got, days))
			}
		}
	}
	var pipeline map[string]yaml.Node
	if err := yaml.Unmarshal([]byte(gitlab), &pipeline); err != nil {
		t.Fatal(err)
	}
	for job, node := range pipeline {
		var declared struct {
			Artifacts *struct {
				ExpireIn string `yaml:"expire_in"`
			} `yaml:"artifacts"`
		}
		// A top-level key that is no job, such as a list of stages, declares none.
		if node.Kind != yaml.MappingNode || node.Decode(&declared) != nil || declared.Artifacts == nil {
			continue
		}
		declarations++
		if declared.Artifacts.ExpireIn != days+" days" {
			problems = append(problems, fmt.Sprintf("GitLab %s keeps its artifacts for %q, want %q", job, declared.Artifacts.ExpireIn, days+" days"))
		}
	}
	return uploads, declarations, problems
}
