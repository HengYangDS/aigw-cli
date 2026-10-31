package terminal

import (
	"slices"
	"strings"
)

// NextSteps splits a follow-up into the command that a script can run as it
// stands and the guidance that it cannot: a command with a placeholder, a
// command that asks a person for a value, or prose.
func NextSteps(next string) (action, detail string) {
	if strings.HasPrefix(next, "aigw ") && !strings.ContainsAny(next, "<>`;,") && !asksPerson(strings.Fields(next)[1:]) {
		return next, ""
	}
	return "", next
}

// asksPerson reports whether an aigw command line asks a person at a terminal
// for a value that AIGW cannot choose: bare setup asks for its Account and
// Route, and a command that stores a secret reads it from a prompt unless its
// standard-input flag supplies it. Setup connects an Account, and so reads
// its Token, unless it only imports a manifest; a command's help asks for
// nothing.
func asksPerson(words []string) bool {
	has := func(word string) bool { return slices.Contains(words, word) }
	switch {
	case len(words) == 0 || has("--help") || has("-h"):
		return false
	case words[0] == "setup":
		importOnly := has("--from") && !has("--account")
		return !importOnly && !has("--token-stdin")
	case words[0] == "add", words[0] == "rotate":
		return !has("--token-stdin")
	case len(words) >= 3 && words[0] == "account" && words[1] == "diagnostics" && words[2] == "enable":
		return !has("--system-token-stdin")
	}
	return false
}
