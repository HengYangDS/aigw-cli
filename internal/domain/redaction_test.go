package domain_test

import (
	"strings"
	"testing"

	"aigw-cli/internal/domain"
)

func TestRedactRemovesKnownAndBearerCredentials(t *testing.T) {
	secret := "sk value/+?"
	text := domain.Redact("raw sk value/+? encoded sk+value%2F%2B%3F bearer unknown-token", secret)
	for _, forbidden := range []string{secret, "sk+value%2F%2B%3F", "unknown-token"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("redacted text contains %q: %q", forbidden, text)
		}
	}
	if strings.Count(text, "[REDACTED]") != 3 {
		t.Fatalf("redacted text = %q", text)
	}
}

func TestRedactSkipsBlankSecretsAndReturnsUnchangedInput(t *testing.T) {
	text := domain.Redact("nothing secret here", "", "   ")
	if text != "nothing secret here" {
		t.Fatalf("Redact = %q", text)
	}
}

func TestRedactRemovesUnknownStructuredCredentialFields(t *testing.T) {
	text := domain.Redact(`{"api_key":"unknown-json-key","token":"unknown-token","nested":{"client_secret":"unknown-secret"},"safe":"kept"} query api_key=unknown-query-key&mode=read`)
	for _, forbidden := range []string{"unknown-json-key", "unknown-token", "unknown-secret", "unknown-query-key"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("redacted text contains %q: %q", forbidden, text)
		}
	}
	if !strings.Contains(text, `"safe":"kept"`) {
		t.Fatalf("redaction removed unrelated diagnostic context: %q", text)
	}
}

// The forms in which a client or a provider writes a credential: a header
// with its authentication scheme, a cookie line, a URL's user and password, a
// key in a query string and a bare sk- key. The words around them stay; a
// bare value takes a closing quote with it, since a credential can hold one.
func TestRedactRemovesHeaderURLAndBareCredentialForms(t *testing.T) {
	for name, test := range map[string]struct{ text, want string }{
		"bearer header":       {"error: 401 Authorization: Bearer live-token-123 rejected", "error: 401 Authorization: [REDACTED] rejected"},
		"basic header":        {"Authorization: Basic dXNlcjpwYXNz", "Authorization: [REDACTED]"},
		"proxy header":        {"Proxy-Authorization: Digest opaque-0001, retry", "Proxy-Authorization: [REDACTED], retry"},
		"quoted header":       {`sent 'Authorization: Bearer live-token-123' to https://example.test`, `sent 'Authorization: [REDACTED] to https://example.test`},
		"token with scheme":   {"token: Bearer live-token-123", "token: [REDACTED]"},
		"scheme word alone":   {"password = basic", "password = [REDACTED]"},
		"cookie line":         {"Cookie: session=abc; csrf=def\nnext line", "Cookie: [REDACTED]\nnext line"},
		"set-cookie line":     {"set-cookie: id=abc; Path=/", "set-cookie: [REDACTED]"},
		"url userinfo":        {"GET https://user:hunter2@host.example.test/v1 failed", "GET https://[REDACTED]@host.example.test/v1 failed"},
		"query key":           {"POST https://host.example.test/v1?alt=sse&key=AIzaLive0001#frag", "POST https://host.example.test/v1?alt=sse&key=[REDACTED]#frag"},
		"bare key":            {"Incorrect API key provided: sk-proj-00000000aaaaaaaa.", "Incorrect API key provided: [REDACTED]."},
		"secret access key":   {"aws_secret_access_key=wJalrXUtnFEMI", "aws_secret_access_key=[REDACTED]"},
		"passwd":              {"passwd: hunter2", "passwd: [REDACTED]"},
		"short sk word stays": {"the sk-1 flag and a task-list", "the sk-1 flag and a task-list"},
		"url without user":    {"https://host.example.test:8443/v1 at 12:30", "https://host.example.test:8443/v1 at 12:30"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := domain.Redact(test.text); got != test.want {
				t.Fatalf("Redact(%q) = %q, want %q", test.text, got, test.want)
			}
		})
	}
}
