package cli

import (
	"aigw-cli/internal/verification"
	"testing"
	"time"
)

func TestProtocolVerificationDeadlineAllowsColdClaudeStartup(t *testing.T) {
	if verification.ProtocolTimeout < time.Minute {
		t.Fatalf("protocol verification timeout = %s, want at least %s for Claude process startup and one upstream request", verification.ProtocolTimeout, time.Minute)
	}
}
