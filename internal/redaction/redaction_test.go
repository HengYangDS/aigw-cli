package redaction_test

import (
	"strings"
	"testing"

	"aigw-cli/internal/redaction"
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

func TestTextSkipsBlankSecretsAndReturnsUnchangedInput(t *testing.T) {
	text := redaction.Text("nothing secret here", "", "   ")
	if text != "nothing secret here" {
		t.Fatalf("Text = %q", text)
	}
}

func TestTextRemovesUnknownStructuredCredentialFields(t *testing.T) {
	text := redaction.Text(`{"api_key":"unknown-json-key","token":"unknown-token","nested":{"client_secret":"unknown-secret"},"safe":"kept"} query api_key=unknown-query-key&mode=read`)
	for _, forbidden := range []string{"unknown-json-key", "unknown-token", "unknown-secret", "unknown-query-key"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("redacted text contains %q: %q", forbidden, text)
		}
	}
	if !strings.Contains(text, `"safe":"kept"`) {
		t.Fatalf("redaction removed unrelated diagnostic context: %q", text)
	}
}
