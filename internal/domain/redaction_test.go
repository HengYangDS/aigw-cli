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
// to the end of its line, whatever its scheme, a URL's user, a key or a
// signature in a query string and a bare sk- key. A rule stays on its line,
// and what only looks like a form stays: a port, an address after a ?, a
// short name that begins with sk-.
func TestRedactRemovesHeaderURLAndBareCredentialForms(t *testing.T) {
	for name, test := range map[string]struct{ text, want string }{
		"bearer header":       {"error: 401 Authorization: Bearer live-token-123 rejected", "error: 401 Authorization: [REDACTED]"},
		"basic header":        {"Authorization: Basic dXNlcjpwYXNz", "Authorization: [REDACTED]"},
		"other scheme":        {"rejected Authorization: ApiKey live-leak-0011 by the endpoint", "rejected Authorization: [REDACTED]"},
		"scheme parameters":   {`Proxy-Authorization: Digest username="u", response="0a1b", opaque="2c3d"`, "Proxy-Authorization: [REDACTED]"},
		"signed header":       {"Authorization: AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20261114, SignedHeaders=host, Signature=0a1b", "Authorization: [REDACTED]"},
		"header ends a line":  {"note Authorization: Bearer\nwarning: capability degraded", "note Authorization: [REDACTED]\nwarning: capability degraded"},
		"quoted field":        {`{"authorization":"Bearer live-token-123","status":401}`, `{"authorization":[REDACTED],"status":401}`},
		"token with bearer":   {"token: Bearer live-token-123 next", "token: [REDACTED] next"},
		"bearer ends a line":  {"token: Bearer\nnext line", "token: [REDACTED]\nnext line"},
		"cookie line":         {"Cookie: session=abc; csrf=def\nnext line", "Cookie: [REDACTED]\nnext line"},
		"empty cookie line":   {"Cookie:\nnext line", "Cookie:\nnext line"},
		"set-cookie line":     {"set-cookie: id=abc; Path=/", "set-cookie: [REDACTED]"},
		"url user, password":  {"GET https://user:hunter2@host.example.test/v1 failed", "GET https://[REDACTED]@host.example.test/v1 failed"},
		"url user alone":      {"clone https://ghtoken0001@host.example.test/repo", "clone https://[REDACTED]@host.example.test/repo"},
		"url password alone":  {"redis://:hunter2@host.example.test:6379", "redis://[REDACTED]@host.example.test:6379"},
		"query key":           {"POST https://host.example.test/v1?alt=sse&key=AIzaLive0001#frag", "POST https://host.example.test/v1?alt=sse&key=[REDACTED]#frag"},
		"query key, capital":  {"GET https://host.example.test/v1?Key=AIzaLive0001", "GET https://host.example.test/v1?Key=[REDACTED]"},
		"query signature":     {"GET https://host.example.test/blob?sv=1&sig=0a1b2c&se=2", "GET https://host.example.test/blob?sv=1&sig=[REDACTED]&se=2"},
		"bare key":            {"Incorrect API key provided: sk-proj-00000000aaaaaaaa.", "Incorrect API key provided: [REDACTED]."},
		"secret key":          {"secret_key: 0a1b2c3d", "secret_key: [REDACTED]"},
		"secret access key":   {"aws_secret_access_key=wJalrXUtnFEMI", "aws_secret_access_key=[REDACTED]"},
		"passwd":              {"passwd: hunter2", "passwd: [REDACTED]"},
		"short sk names stay": {"the sk-1 flag, model sk-model-preview-2026 and /tmp/sk-fixture-dir0001/", "the sk-1 flag, model sk-model-preview-2026 and /tmp/sk-fixture-dir0001/"},
		"url without user":    {"https://host.example.test:8443/v1 at 12:30", "https://host.example.test:8443/v1 at 12:30"},
		"address in a query":  {"http://127.0.0.1:8792?user=a@b.example.test", "http://127.0.0.1:8792?user=a@b.example.test"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := domain.Redact(test.text); got != test.want {
				t.Fatalf("Redact(%q) = %q, want %q", test.text, got, test.want)
			}
		})
	}
}
