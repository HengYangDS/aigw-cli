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
// for a value that AIGW cannot choose: setup without a team file asks for a
// provider, a client and a model, setup asks for the Token of each provider
// that it names, and a command that stores a Token reads it from a prompt
// unless its standard-input flag supplies it; a command's help asks for
// nothing.
func asksPerson(words []string) bool {
	has := func(word string) bool { return slices.Contains(words, word) }
	switch {
	case len(words) == 0 || has("--help") || has("-h"):
		return false
	case words[0] == "setup":
		teamOnly := has("--from") && !has("--provider")
		return !teamOnly && !has("--token-stdin")
	case len(words) >= 2 && words[0] == "provider" && (words[1] == "add" || words[1] == "rotate"):
		return !has("--token-stdin")
	}
	return false
}
