package credential

import (
	"fmt"
	"path/filepath"
	"runtime"
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
	if goos == "windows" && strings.ContainsAny(executable, "\"%!^&|<>()") {
		return "", fmt.Errorf("credential executable contains Windows shell expansion characters")
	}
	if goos == "windows" && runtime.GOOS == "windows" {
		var err error
		executable, err = nativeShellPath(executable)
		if err != nil {
			return "", err
		}
	}
	quoted := "'" + strings.ReplaceAll(executable, "'", "'\\''") + "'"
	if goos == "windows" {
		quoted = `"` + executable + `"`
	}
	return quoted + " credential " + client + " " + scope, nil
}

// ExecutableFromCommand reverses only Command's exact shell grammar. It does
// not execute a command or treat the extracted path as an owned reader.
func ExecutableFromCommand(command, client, scope, goos string) (string, error) {
	suffix := " credential " + client + " " + scope
	if !strings.HasSuffix(command, suffix) {
		return "", fmt.Errorf("credential invocation does not match its client and scope")
	}
	quoted := strings.TrimSuffix(command, suffix)
	if len(quoted) < 2 {
		return "", fmt.Errorf("credential invocation has no quoted executable")
	}
	var executable string
	if goos == "windows" {
		if quoted[0] != '"' || quoted[len(quoted)-1] != '"' {
			return "", fmt.Errorf("credential invocation has no Windows-quoted executable")
		}
		executable = quoted[1 : len(quoted)-1]
	} else {
		if quoted[0] != '\'' || quoted[len(quoted)-1] != '\'' {
			return "", fmt.Errorf("credential invocation has no Unix-quoted executable")
		}
		executable = strings.ReplaceAll(quoted[1:len(quoted)-1], "'\\''", "'")
	}
	rendered, err := Command(executable, client, scope, goos)
	if err != nil || rendered != command {
		return "", fmt.Errorf("credential invocation differs from AIGW's exact command grammar")
	}
	if goos == "windows" && runtime.GOOS == "windows" {
		return nativeCanonicalPath(executable)
	}
	return executable, nil
}
