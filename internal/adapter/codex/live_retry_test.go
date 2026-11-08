package codex

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// A failed request names a retry of the same check. A model that the check
// named takes its own isolated path, so its retry keeps --model, and the retry
// of the client's own selection does not gain one.
func TestFailedCodexVerificationRetriesTheSameInvocation(t *testing.T) {
	for _, explicitRoute := range []string{"", "team/explicit-model"} {
		suffix := ""
		if explicitRoute != "" {
			suffix = " --model " + explicitRoute
		}
		t.Run("model="+explicitRoute, func(t *testing.T) {
			cfg, selected := configuredCodexVerification(t, filepath.Join(t.TempDir(), "config.toml"))
			runner := &recordingCaptureRunner{version: "codex-cli 9.9.9", requestErr: errors.New("exit status 1")}
			want := "`aigw check codex" + suffix + " --depth client`"
			if _, err := VerifyInvocation(t.Context(), runner, cfg, selected, explicitRoute); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("Codex retry = %v, want %s", err, want)
			}
		})
	}
}
