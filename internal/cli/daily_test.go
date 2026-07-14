package cli

import (
	"testing"
	"time"
)

func TestProtocolVerificationDeadlineAllowsColdClaudeStartup(t *testing.T) {
	if protocolVerificationTimeout < time.Minute {
		t.Fatalf("protocol verification timeout = %s, want at least %s for Claude process startup and one upstream request", protocolVerificationTimeout, time.Minute)
	}
}
