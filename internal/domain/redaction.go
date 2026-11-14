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

// bearerCredential is the word Bearer with the credential after it on its line.
var bearerCredential = regexp.MustCompile(`(?i)\bbearer[ \t]+[^[:space:],;\}"']+`)

// credentialHeader is an Authorization, Proxy-Authorization, Cookie or
// Set-Cookie header. Its value runs to the end of its line: a scheme and its
// credential, a scheme's parameters, or a cookie's pairs.
var credentialHeader = regexp.MustCompile(`(?i)(\b(?:authorization|cookie)[ \t]*:[ \t]*)[^\r\n]+`)

// credentialAssignment is a credential field with its value: quoted, or a bare
// word, with the authentication scheme when one stands before the credential
// on its line.
var credentialAssignment = regexp.MustCompile(`(?i)(["']?(api[_-]?key|token|secret|secret[_-]?key|access[_-]?key|password|passwd|client[_-]?secret|access[_-]?token|refresh[_-]?token|authorization)["']?[[:space:]]*[:=][[:space:]]*)("([^"\\]|\\.)*"|'([^'\\]|\\.)*'|(?:(?:bearer|basic|digest|token)[ \t]+)?[^[:space:],;&}]+)`)

// urlUserinfo is the user of a URL, with or without a password. A port is
// not one: nothing after a ? or a # is the user.
var urlUserinfo = regexp.MustCompile(`(://)[^/[:space:]:@?#]*(?::[^/[:space:]@?#]*)?@`)

// queryKey is a query parameter by which some providers take a credential or
// a signature.
var queryKey = regexp.MustCompile(`(?i)([?&](?:key|sig|signature)=)[^&#[:space:]"']+`)

// prefixedKey is a bare provider key in its sk- form, which is longer than a
// model or a directory name that begins that way.
var prefixedKey = regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}`)

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
	value = credentialHeader.ReplaceAllString(value, "${1}"+redactionReplacement)
	value = credentialAssignment.ReplaceAllString(value, "${1}"+redactionReplacement)
	value = bearerCredential.ReplaceAllString(value, "Bearer "+redactionReplacement)
	value = urlUserinfo.ReplaceAllString(value, "${1}"+redactionReplacement+"@")
	value = queryKey.ReplaceAllString(value, "${1}"+redactionReplacement)
	return prefixedKey.ReplaceAllString(value, redactionReplacement)
}
