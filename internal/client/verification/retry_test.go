package verification

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/claude"
	configuration "aigw-cli/internal/configuration"
)

// A failed request names a retry of the same invocation. An explicit Route
// takes its own isolated path, so its retry keeps --route, and the retry of a
// selected binding does not gain one.
func TestFailedVerificationRetriesTheSameInvocation(t *testing.T) {
	for _, explicitRoute := range []string{"", "explicit-route"} {
		suffix := ""
		if explicitRoute != "" {
			suffix = " --route " + explicitRoute
		}
		t.Run("route="+explicitRoute, func(t *testing.T) {
			claudeConfig := verificationConfig()
			claudeRuntime, err := claudeConfig.ResolveRuntime(configuration.ClientClaude, "")
			if err != nil {
				t.Fatal(err)
			}
			claudeRuntime.CredentialCommand = filepath.Join(t.TempDir(), "aigw")
			settings := filepath.Join(t.TempDir(), "settings.json")
			if _, err := claude.ReconcileSettings(settings, false, claudeRuntime, claudeRuntime.CredentialCommand, claudeRuntime.Model); err != nil {
				t.Fatal(err)
			}
			want := "`aigw verify --for claude" + suffix + "`"
			for _, runner := range []captureRunner{
				{output: []byte(`{"type":"result","is_error":true,"result":"rejected"}`)},
				{output: []byte(`{"type":"system","subtype":"api_retry","error_status":403}`), err: context.DeadlineExceeded},
			} {
				if err := VerifyClaudeRuntime(t.Context(), runner, "claude", settings, claudeRuntime, "token", explicitRoute); err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("Claude retry = %v, want %s", err, want)
				}
			}
		})
	}
}

func TestVerificationCommandKeepsOnlyAnExplicitRoute(t *testing.T) {
	if got := Command(configuration.ClientHermes, ""); got != "aigw verify --for hermes" {
		t.Fatalf("selected-binding command = %q", got)
	}
	if got := Command(configuration.ClientCodex, "team.codex-1"); got != "aigw verify --for codex --route team.codex-1" {
		t.Fatalf("explicit-Route command = %q", got)
	}
}
