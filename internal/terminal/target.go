package terminal

import (
	"strings"
)

// HomeDirectory reads the user's home directory from a command environment.
func HomeDirectory(env []string) string {
	for _, name := range []string{"HOME", "USERPROFILE"} {
		for _, entry := range env {
			if value, ok := strings.CutPrefix(entry, name+"="); ok && value != "" {
				return value
			}
		}
	}
	return ""
}

// SnakeValue spells a state or kind in snake_case, as every JSON document
// does: the hyphens of an adapter's or a planner's own spelling become
// underscores.
func SnakeValue(value string) string {
	return strings.ReplaceAll(value, "-", "_")
}
