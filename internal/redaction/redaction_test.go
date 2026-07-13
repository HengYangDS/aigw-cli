package redaction_test

import (
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/redaction"
)

func TestTextRemovesKnownAndBearerCredentials(t *testing.T) {
	secret := "sk value/+?"
	text := redaction.Text("raw sk value/+? encoded sk+value%2F%2B%3F bearer unknown-token", secret)
	for _, forbidden := range []string{secret, "sk+value%2F%2B%3F", "unknown-token"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("redacted text contains %q: %q", forbidden, text)
		}
	}
	if strings.Count(text, "[REDACTED]") != 3 {
		t.Fatalf("redacted text = %q", text)
	}
}
