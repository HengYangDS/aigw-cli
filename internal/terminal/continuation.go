package terminal

import "strings"

// NextSteps splits a follow-up into the command that a script can run as it
// stands and the guidance that it cannot: a command with a placeholder, or
// prose.
func NextSteps(next string) (action, detail string) {
	if strings.HasPrefix(next, "aigw ") && !strings.ContainsAny(next, "<>`;,") {
		return next, ""
	}
	return "", next
}
