package projection

import (
	"slices"
	"strings"
)

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
