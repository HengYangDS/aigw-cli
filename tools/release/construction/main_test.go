package construction

import (
	"os"
	"testing"
)

// TestMain keeps the runner's release-tag selection out of these tests; a tag
// push would otherwise select that tag for every candidate under test. Tests
// that need a selected tag set it explicitly. An empty Git template keeps a
// host init.templateDir from adding hooks, configuration or excludes to
// fixture repositories.
func TestMain(m *testing.M) {
	for _, name := range []string{"AIGW_CHANGELOG_RELEASE_TAG", "GITHUB_REF_TYPE", "GITHUB_REF_NAME", "CI_COMMIT_TAG"} {
		_ = os.Unsetenv(name)
	}
	_ = os.Setenv("GIT_TEMPLATE_DIR", "")
	os.Exit(m.Run())
}
