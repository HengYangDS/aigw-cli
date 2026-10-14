package readiness

import (
	"os"
	"testing"
)

// TestMain gives fixture repositories an empty Git template, so a host
// init.templateDir cannot add hooks, configuration or excludes to them.
func TestMain(m *testing.M) {
	_ = os.Setenv("GIT_TEMPLATE_DIR", "")
	os.Exit(m.Run())
}
