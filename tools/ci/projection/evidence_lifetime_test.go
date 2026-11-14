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
	// The projections are rendered from the model, GitLab's first and then every
	// GitHub workflow; `project --check` holds the tracked files to them.
	projections := canonicalProjections(t)
	gitlab, workflows := projections[0].Content, make([]string, 0, len(projections)-1)
	for _, workflow := range projections[1:] {
		workflows = append(workflows, workflow.Content)
	}
	// A workflow that the model does not render would be read by no check.
	tracked, err := filepath.Glob(filepath.Join("..", "..", "..", ".github", "workflows", "*.y*ml"))
	if err != nil || len(tracked) != len(workflows) {
		t.Fatalf("the repository tracks %d GitHub workflows and the model renders %d (%v)", len(tracked), len(workflows), err)
	}
	uploads, declarations, problems := evidenceLifetimeProblems(t, gitlab, workflows, days)
	if uploads == 0 || declarations == 0 {
		t.Fatalf("found %d GitHub uploads and %d GitLab artifact declarations; the test reads neither", uploads, declarations)
	}
	for _, problem := range problems {
		t.Error(problem)
	}

	// The check itself: an upload without the field, another number and an
	// artifact declaration without an expiry are each a problem.
	for name, mutate := range map[string]func(gitlab, verify string) (string, string){
		"GitHub upload without retention": func(gitlab, verify string) (string, string) {
			return gitlab, strings.Replace(verify, "retention-days: "+days, "compression-level: 6", 1)
		},
		"GitHub upload with another number": func(gitlab, verify string) (string, string) {
			return gitlab, strings.Replace(verify, "retention-days: "+days, "retention-days: 90", 1)
		},
		"GitLab artifacts without expiry": func(gitlab, verify string) (string, string) {
			return strings.Replace(gitlab, "expire_in: "+days+" days", "untracked: false", 1), verify
		},
		"GitLab artifacts with another expiry": func(gitlab, verify string) (string, string) {
			return strings.Replace(gitlab, "expire_in: "+days+" days", "expire_in: never", 1), verify
		},
	} {
		mutatedGitLab, mutatedVerify := mutate(gitlab, workflows[0])
		if _, _, found := evidenceLifetimeProblems(t, mutatedGitLab, append([]string{mutatedVerify}, workflows[1:]...), days); len(found) != 1 {
			t.Errorf("%s: the test reports %d problems, want 1: %v", name, len(found), found)
		}
	}
	// A workflow other than the first is read too, and so is the action that
	// merges artifacts into a new one: each upload there without the lifetime
	// is a problem.
	for name, action := range map[string]string{"upload": "actions/upload-artifact", "merge": "actions/upload-artifact/merge"} {
		release := "jobs:\n  publish:\n    steps:\n      - name: Retain\n        uses: " + action + "@0000000000000000000000000000000000000000\n"
		if _, _, found := evidenceLifetimeProblems(t, gitlab, []string{workflows[0], release}, days); len(found) != 1 {
			t.Errorf("the %s action in a second workflow: the test reports %d problems, want 1: %v", name, len(found), found)
		}
	}
}

// evidenceLifetimeProblems counts the upload steps of the GitHub workflows and
// the artifact declarations of the GitLab pipeline, and names each one that
// does not carry days.
func evidenceLifetimeProblems(t *testing.T, gitlab string, workflows []string, days string) (uploads, declarations int, problems []string) {
	t.Helper()
	for _, github := range workflows {
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
				if !strings.Contains(step.Uses, "/upload-artifact@") && !strings.Contains(step.Uses, "/upload-artifact/") {
					continue
				}
				uploads++
				if got := step.With["retention-days"]; got != days {
					problems = append(problems, fmt.Sprintf("GitHub %s step %q keeps its artifact for %q days, want %s", job, step.Name, got, days))
				}
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
