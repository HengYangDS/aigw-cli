// Package redaction removes known credentials from text that may reach a user,
// log, diagnostic, or error boundary.
package redaction

import (
	"net/url"
	"regexp"
	"strings"
)

const replacement = "[REDACTED]"

var bearerCredential = regexp.MustCompile(`(?i)\bbearer[[:space:]]+[^[:space:],;\}"']+`)

// Text removes every supplied secret in its plain and URL-escaped forms, then
// removes bearer credentials whose original value is not otherwise known.
func Text(value string, secrets ...string) string {
	for _, secret := range secrets {
		secret = strings.TrimSpace(secret)
		if secret == "" {
			continue
		}
		for _, variant := range []string{secret, url.QueryEscape(secret), url.PathEscape(secret)} {
			value = strings.ReplaceAll(value, variant, replacement)
		}
	}
	return bearerCredential.ReplaceAllString(value, "Bearer "+replacement)
}
