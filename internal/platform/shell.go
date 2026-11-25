package platform

import (
	"regexp"
	"slices"
	"strings"
)

// plainWord matches a word that the platform's shells pass through
// unchanged, so a command line of such words runs as written: it starts with
// a letter, digit, dot or slash.
var plainWord = regexp.MustCompile(`^[A-Za-z0-9._/][A-Za-z0-9._/:@+=~%-]*$`)

// shellTilde finds a tilde that a shell would expand after an equals sign or
// a colon.
var shellTilde = regexp.MustCompile(`[=:]~`)

// PlainWord reports whether the platform's shells pass word through
// unchanged, so that a command line may name it without quoting.
func PlainWord(word string) bool {
	return plainWord.MatchString(word) && !shellTilde.MatchString(word)
}

// AIGWCommand returns the aigw command line of words, or "" when a word would
// need quoting in the platform's shells: no one quoting runs in every shell
// of a platform, so such a command is given in prose instead.
func AIGWCommand(words ...string) string {
	if !slices.ContainsFunc(words, func(word string) bool { return !PlainWord(word) }) {
		return strings.Join(append([]string{"aigw"}, words...), " ")
	}
	return ""
}
