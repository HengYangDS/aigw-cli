package verification

import (
	"errors"
	"strings"
	"testing"
	"time"

	configuration "aigw-cli/internal/configuration"
)

func TestProtocolTimeoutAllowsColdClientStartup(t *testing.T) {
	if ProtocolTimeout < time.Minute {
		t.Fatalf("ProtocolTimeout = %s, want at least %s", ProtocolTimeout, time.Minute)
	}
}

func TestFailureAdvisesTheClaudeSettingOnlyForClaude(t *testing.T) {
	cause := errors.New("launch /Users/operator/private/claude: exit status 1")
	compatibility := `{"type":"result","is_error":true,"result":"API Error: 400 context_management: Extra inputs are not permitted; token=must-not-leak; /Users/operator/private"}`
	if err := Failure("Codex", configuration.ClientCodex, "", []byte(compatibility), cause); strings.Contains(err.Error(), "CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS") {
		t.Fatalf("Codex failure advised a Claude-specific setting: %v", err)
	}
	if err := Failure("Claude", configuration.ClientClaude, "", []byte(compatibility), cause); !strings.Contains(err.Error(), "CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1") || !errors.Is(err, cause) {
		t.Fatalf("Claude failure lost its compatibility advice or cause: %v", err)
	}
}
