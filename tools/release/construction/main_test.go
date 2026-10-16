package construction

import (
	"aigw-cli/tools/gitfixture"
	"os"
	"testing"
)

// TestMain keeps the runner's release-tag selection out of these tests; a tag
// push would otherwise select that tag for every candidate under test. Tests
// that need a selected tag set it explicitly. gitfixture keeps host Git hooks
// and templates out of fixture repositories.
func TestMain(m *testing.M) {
	for _, name := range []string{"AIGW_CHANGELOG_RELEASE_TAG", "GITHUB_REF_TYPE", "GITHUB_REF_NAME", "CI_COMMIT_TAG"} {
		_ = os.Unsetenv(name)
	}
	os.Exit(gitfixture.Run(m))
}
