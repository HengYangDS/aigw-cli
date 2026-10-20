package projection

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestGitLabNativeRunnerTrustSplit(t *testing.T) {
	projections := canonicalProjections(t)
	var pipeline struct {
		Quality       gitLabJob `yaml:"quality"`
		Darwin        gitLabJob `yaml:"native-darwin"`
		DarwinReview  gitLabJob `yaml:"native-darwin-review"`
		Windows       gitLabJob `yaml:"native-windows"`
		WindowsReview gitLabJob `yaml:"native-windows-review"`
	}
	if err := yaml.Unmarshal([]byte(projections[0].Content), &pipeline); err != nil {
		t.Fatal(err)
	}
	for _, route := range []struct {
		platform, protectedTag, reviewTag string
		protected, review                 gitLabJob
	}{
		{"darwin", "ci-macos-arm64-shell", "ci-macos-arm64-review", pipeline.Darwin, pipeline.DarwinReview},
		{"windows", "ci-windows-arm64-shell", "ci-windows-arm64-review", pipeline.Windows, pipeline.WindowsReview},
	} {
		t.Run(route.platform, func(t *testing.T) {
			if !slices.Equal(route.protected.Tags, []string{route.protectedTag}) || !slices.Equal(route.review.Tags, []string{route.reviewTag}) {
				t.Fatalf("native trust tags = protected %q, review %q", route.protected.Tags, route.review.Tags)
			}
			if !slices.Equal(route.protected.Script, route.review.Script) ||
				!slices.Equal(route.protected.BeforeScript, route.review.BeforeScript) ||
				!maps.Equal(route.protected.Variables, route.review.Variables) {
				t.Fatal("review and protected jobs diverged from the shared native command owner")
			}
			protected, review := admissionRules(route.protected.Rules), admissionRules(route.review.Rules)
			if len(protected) != 4 || len(review) != 3 ||
				protected[0].If != "$CI_COMMIT_TAG" ||
				protected[1].If != pipeline.Quality.Rules[2].If ||
				review[0].If != pipeline.Quality.Rules[1].If ||
				protected[3].When != "never" || review[2].When != "never" {
				t.Fatalf("native trust event routes = protected %#v, review %#v", protected, review)
			}
			if !strings.Contains(protected[2].If, `$CI_COMMIT_REF_PROTECTED == "true"`) ||
				!strings.Contains(review[1].If, `$CI_COMMIT_REF_PROTECTED == "false"`) ||
				!strings.Contains(protected[2].If, `$AIGW_NATIVE_PLATFORM == "`+route.platform+`"`) ||
				!strings.Contains(review[1].If, `$AIGW_NATIVE_PLATFORM == "`+route.platform+`"`) ||
				!strings.Contains(review[0].If, "$CI_MERGE_REQUEST_SOURCE_PROJECT_ID == $CI_PROJECT_ID") {
				t.Fatal("native manual or MR trust predicate is absent")
			}
		})
	}
}
