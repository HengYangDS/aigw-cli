package credential

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"aigw-cli/internal/configuration"
)

// Command renders a native client's shell-based credential helper invocation.
// Arguments contain only non-secret projection identity; the token stays on stdout.
func Command(executable, client, scope, goos string) (string, error) {
	if !filepath.IsAbs(executable) || strings.TrimSpace(executable) != executable || strings.ContainsFunc(executable, unicode.IsControl) {
		return "", fmt.Errorf("credential executable must be one absolute path without control characters")
	}
	if !configuration.ValidIdentifier(client) || !configuration.ValidIdentifier(scope) {
		return "", fmt.Errorf("credential client and projection identity must be safe identifiers")
	}
	quoted := "'" + strings.ReplaceAll(executable, "'", "'\\''") + "'"
	if goos == "windows" {
		if strings.ContainsAny(executable, "\"%!^&|<>()") {
			return "", fmt.Errorf("credential executable contains Windows shell expansion characters")
		}
		quoted = `"` + executable + `"`
	}
	return quoted + " credential " + client + " " + scope, nil
}
