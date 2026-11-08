package claude

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// A failed request names a retry of the same check. A model that the check
// named takes its own isolated path, so its retry keeps --model, and the retry
// of the client's own selection does not gain one.
func TestFailedClaudeVerificationRetriesTheSameInvocation(t *testing.T) {
	for _, explicitRoute := range []string{"", "team/explicit-model"} {
		suffix := ""
		if explicitRoute != "" {
			suffix = " --model " + explicitRoute
		}
		t.Run("model="+explicitRoute, func(t *testing.T) {
			claudeConfig := verificationConfig()
			claudeRuntime, err := claudeConfig.ResolveRuntime(ID, "")
			if err != nil {
				t.Fatal(err)
			}
			claudeRuntime.CredentialCommand = filepath.Join(t.TempDir(), "aigw")
			settings := filepath.Join(t.TempDir(), "settings.json")
			if _, err := ReconcileSettings(settings, false, claudeRuntime, claudeRuntime.CredentialCommand, claudeRuntime.UpstreamModel); err != nil {
				t.Fatal(err)
			}
			want := "`aigw check claude" + suffix + " --depth client`"
			for _, runner := range []captureRunner{
				{output: []byte(`{"type":"result","is_error":true,"result":"rejected"}`)},
				{output: []byte(`{"type":"system","subtype":"api_retry","error_status":403}`), err: context.DeadlineExceeded},
			} {
				if err := verifyRuntime(t.Context(), runner, "claude", settings, claudeRuntime, "token", explicitRoute); err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("Claude retry = %v, want %s", err, want)
				}
			}
		})
	}
}
