package domain

import (
	"net/url"
	"regexp"
	"strings"
)

// The redaction rules remove known credentials from text that may reach a
// user, log, diagnostic, or error boundary.
const redactionReplacement = "[REDACTED]"

var bearerCredential = regexp.MustCompile(`(?i)\bbearer[[:space:]]+[^[:space:],;\}"']+`)

var credentialAssignment = regexp.MustCompile(`(?i)(["']?(api[_-]?key|token|secret|password|client[_-]?secret|access[_-]?token|refresh[_-]?token|authorization)["']?[[:space:]]*[:=][[:space:]]*)("([^"\\]|\\.)*"|'([^'\\]|\\.)*'|[^[:space:],;&}]+)`)

// Redact removes every supplied secret in its plain and URL-escaped forms, then
// removes bearer credentials whose original value is not otherwise known.
func Redact(value string, secrets ...string) string {
	for _, secret := range secrets {
		secret = strings.TrimSpace(secret)
		if secret == "" {
			continue
		}
		for _, variant := range []string{secret, url.QueryEscape(secret), url.PathEscape(secret)} {
			value = strings.ReplaceAll(value, variant, redactionReplacement)
		}
	}
	value = credentialAssignment.ReplaceAllString(value, "${1}"+redactionReplacement)
	return bearerCredential.ReplaceAllString(value, "Bearer "+redactionReplacement)
}
