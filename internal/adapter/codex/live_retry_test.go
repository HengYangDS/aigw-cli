package codex

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// A failed request names a retry of the same invocation. An explicit Route
// takes its own isolated path, so its retry keeps --route, and the retry of a
// selected binding does not gain one.
func TestFailedCodexVerificationRetriesTheSameInvocation(t *testing.T) {
	for _, explicitRoute := range []string{"", "explicit-route"} {
		suffix := ""
		if explicitRoute != "" {
			suffix = " --route " + explicitRoute
		}
		t.Run("route="+explicitRoute, func(t *testing.T) {
			cfg, selected := configuredCodexVerification(t, filepath.Join(t.TempDir(), "config.toml"))
			runner := &recordingCaptureRunner{version: "codex-cli 9.9.9", requestErr: errors.New("exit status 1")}
			want := "`aigw verify --for codex" + suffix + "`"
			if _, err := VerifyInvocation(t.Context(), runner, cfg, selected, explicitRoute); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("Codex retry = %v, want %s", err, want)
			}
		})
	}
}
