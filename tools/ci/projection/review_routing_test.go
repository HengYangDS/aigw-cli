package projection

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

func TestGitLabNativeRunnerTrustSplit(t *testing.T) {
	projections := canonicalProjections(t)
	var pipeline struct {
		Quality      gitLabJob `yaml:"quality"`
		Darwin       gitLabJob `yaml:"native-darwin"`
		DarwinReview gitLabJob `yaml:"native-darwin-review"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	tags := declaredMacOSRunnerTags(t)
	protected, review := pipeline.Darwin, pipeline.DarwinReview
	if !slices.Equal(protected.Tags, tags.Protected) || !slices.Equal(review.Tags, tags.Review) {
		t.Fatalf("native trust tags = protected %q, review %q", protected.Tags, review.Tags)
	}
	if !slices.Equal(protected.Script, review.Script) ||
		!slices.Equal(protected.BeforeScript, review.BeforeScript) ||
		!maps.Equal(protected.Variables, review.Variables) {
		t.Fatal("review and protected jobs diverged from the shared native command owner")
	}
	protectedRules, reviewRules := admissionRules(protected.Rules), admissionRules(review.Rules)
	if len(protectedRules) != 4 || len(reviewRules) != 3 ||
		protectedRules[0].If != "$CI_COMMIT_TAG" ||
		protectedRules[1].If != pipeline.Quality.Rules[2].If ||
		reviewRules[0].If != pipeline.Quality.Rules[1].If ||
		protectedRules[3].When != "never" || reviewRules[2].When != "never" {
		t.Fatalf("native trust event routes = protected %#v, review %#v", protectedRules, reviewRules)
	}
	if !strings.Contains(protectedRules[2].If, `$CI_COMMIT_REF_PROTECTED == "true"`) ||
		!strings.Contains(reviewRules[1].If, `$CI_COMMIT_REF_PROTECTED == "false"`) ||
		!strings.Contains(protectedRules[2].If, `$AIGW_NATIVE_PLATFORM == "darwin"`) ||
		!strings.Contains(reviewRules[1].If, `$AIGW_NATIVE_PLATFORM == "darwin"`) ||
		!strings.Contains(reviewRules[0].If, "$CI_MERGE_REQUEST_SOURCE_PROJECT_ID == $CI_PROJECT_ID") {
		t.Fatal("native manual or MR trust predicate is absent")
	}
}
