package projection

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

// declaredValue decodes one expression of the CI declaration into value, so a
// test compares a projection with the declaration instead of repeating it.
func declaredValue(t *testing.T, expression string, value any) {
	t.Helper()
	output, err := projectionCommand(filepath.Clean(filepath.Join("..", "..", "..")), expression).Output()
	if err != nil {
		t.Fatalf("export %s: %v", expression, err)
	}
	if err := yaml.Unmarshal(output, value); err != nil {
		t.Fatalf("decode %s: %v", expression, err)
	}
}

// macosRunnerTags are the GitLab tag sets of the macOS runners: one for
// protected pipelines and one for review pipelines.
type macosRunnerTags struct {
	Protected []string `yaml:"protectedTags"`
	Review    []string `yaml:"reviewTags"`
}

func declaredMacOSRunnerTags(t *testing.T) macosRunnerTags {
	t.Helper()
	var tags macosRunnerTags
	declaredValue(t, "nativeEvidence.darwin.gitlab", &tags)
	// GitLab gives a job to a runner that holds all of its tags, so neither
	// set may hold the other: a review job never reaches a protected runner.
	if len(tags.Protected) == 0 || len(tags.Review) == 0 || subset(tags.Protected, tags.Review) || subset(tags.Review, tags.Protected) {
		t.Fatalf("macOS runner tags do not separate protected and review pipelines: %+v", tags)
	}
	return tags
}

// subset reports whether every element of part is in whole.
func subset(part, whole []string) bool {
	for _, element := range part {
		if !slices.Contains(whole, element) {
			return false
		}
	}
	return true
}

// nativeCommandFlags returns the flags of a native acceptance command line for
// the platform, leaving out --selection-matrix, whose placement the tools/ci
// tests own; ok is false for any other line.
func nativeCommandFlags(line, platform string) (flags []string, ok bool) {
	base := "mise exec --locked -- go run ./tools/ci native --platform " + platform
	if line != base && !strings.HasPrefix(line, base+" ") {
		return nil, false
	}
	flags = strings.Fields(strings.TrimPrefix(line, base))
	return slices.DeleteFunc(flags, func(flag string) bool { return flag == "--selection-matrix" }), true
}

// isNativeJobCommand reports whether a GitLab script line runs the platform's
// native command in its job-input form.
func isNativeJobCommand(platform string) func(line string) bool {
	return func(line string) bool {
		flags, ok := nativeCommandFlags(line, platform)
		return ok && slices.Equal(flags, []string{"--from-env"})
	}
}
