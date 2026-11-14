package domain

import (
	"net/url"
	"regexp"
	"strings"
)

// The redaction rules remove known credentials and the credential forms below
// from text that may reach a user, log, diagnostic, or error boundary. They
// are forms, not a proof: a credential that no form describes and that the
// caller does not supply stays.
const redactionReplacement = "[REDACTED]"

var bearerCredential = regexp.MustCompile(`(?i)\bbearer[[:space:]]+[^[:space:],;\}"']+`)

// credentialAssignment is a credential field with its value: quoted, or a bare
// word, with the authentication scheme that an Authorization header names
// before its credential.
var credentialAssignment = regexp.MustCompile(`(?i)(["']?(api[_-]?key|token|secret|secret[_-]?key|access[_-]?key|password|passwd|client[_-]?secret|access[_-]?token|refresh[_-]?token|authorization)["']?[[:space:]]*[:=][[:space:]]*)("([^"\\]|\\.)*"|'([^'\\]|\\.)*'|(?:(?:bearer|basic|digest|token)[[:space:]]+)?[^[:space:],;&}]+)`)

// cookieHeader is a Cookie or Set-Cookie header, whose pairs run to the end of
// its line.
var cookieHeader = regexp.MustCompile(`(?i)(\bcookie[[:space:]]*:[[:space:]]*)[^\r\n]+`)

// urlUserinfo is the user and password of a URL.
var urlUserinfo = regexp.MustCompile(`(://)[^/[:space:]:@]+:[^/[:space:]@]+@`)

// queryKey is the key parameter of a query string, by which some providers
// take their credential.
var queryKey = regexp.MustCompile(`([?&]key=)[^&#[:space:]"']+`)

// prefixedKey is a bare provider key in its sk- form.
var prefixedKey = regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{8,}`)

// Redact removes every supplied secret in its plain and URL-escaped forms, then
// the credential forms that the rules above describe.
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
	value = cookieHeader.ReplaceAllString(value, "${1}"+redactionReplacement)
	value = credentialAssignment.ReplaceAllString(value, "${1}"+redactionReplacement)
	value = bearerCredential.ReplaceAllString(value, "Bearer "+redactionReplacement)
	value = urlUserinfo.ReplaceAllString(value, "${1}"+redactionReplacement+"@")
	value = queryKey.ReplaceAllString(value, "${1}"+redactionReplacement)
	return prefixedKey.ReplaceAllString(value, redactionReplacement)
}
