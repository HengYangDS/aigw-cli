//go:build !windows

package hermes

import (
	"slices"
	"testing"
)

// A check run from inside a Hermes session starts Hermes without the
// variables through which that session selects its own provider, model and
// session, nor the ones that its launch flags --safe-mode,
// --ignore-user-config, --ignore-rules, --yolo and --accept-hooks export, and
// keeps the user's own.
func TestVerificationEnvironmentDropsAParentSessionsVariables(t *testing.T) {
	retained := []string{"PATH=/usr/bin", "HOME=/home/user", "HTTPS_PROXY=http://proxy.test", "LANG=C", "HERMES_TIMEZONE=UTC"}
	parent := []string{"HERMES_SESSION_ID=s", "HERMES_SESSION_PLATFORM=cli", "HERMES_GATEWAY_SESSION=1", "HERMES_INTERACTIVE=1", "HERMES_EXEC_ASK=1",
		"HERMES_INFERENCE_PROVIDER=p", "HERMES_INFERENCE_MODEL=m", "HERMES_MODEL=m", "HERMES_PROFILE=work", "HERMES_HOME=/parent", "HERMES_GATEWAY_LOCK_DIR=/parent/lock", "HERMES_DISABLE_LAZY_INSTALLS=0",
		"HERMES_SAFE_MODE=1", "HERMES_IGNORE_USER_CONFIG=1", "HERMES_IGNORE_RULES=1", "HERMES_YOLO_MODE=1", "HERMES_ACCEPT_HOOKS=1"}
	got := verificationEnvironment(slices.Concat(parent, retained), "/own/home", "/own/lock")
	want := slices.Concat(retained, []string{"HERMES_HOME=/own/home", "HERMES_DISABLE_LAZY_INSTALLS=1", "HERMES_GATEWAY_LOCK_DIR=/own/lock"})
	if !slices.Equal(got, want) {
		t.Fatalf("environment = %q, want %q", got, want)
	}
}
