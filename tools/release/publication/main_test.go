package publication

import (
	"aigw-cli/tools/gitfixture"
	"os"
	"testing"
)

// TestMain keeps host Git hooks and templates out of fixture repositories.
func TestMain(m *testing.M) {
	os.Exit(gitfixture.Run(m))
}
