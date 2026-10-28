package claudedesktop

import (
	"fmt"
	"path"

	"aigw-cli/internal/platform"
)

// LibraryPathFor returns Claude Desktop's official per-user third-party
// inference configuration library.
func LibraryPathFor(goos string, env map[string]string) (string, error) {
	switch goos {
	case "darwin":
		if env["HOME"] == "" {
			return "", fmt.Errorf("HOME is not set")
		}
		return path.Join(env["HOME"], "Library", "Application Support", "Claude-3p", "configLibrary"), nil
	case "linux":
		base := env["XDG_CONFIG_HOME"]
		if base == "" {
			if env["HOME"] == "" {
				return "", fmt.Errorf("HOME and XDG_CONFIG_HOME are not set")
			}
			base = path.Join(env["HOME"], ".config")
		}
		return path.Join(base, "Claude-3p", "configLibrary"), nil
	case "windows":
		if env["LOCALAPPDATA"] == "" {
			return "", fmt.Errorf("LOCALAPPDATA is not set")
		}
		return platform.AppendWindowsPath(env["LOCALAPPDATA"], "Claude-3p", "configLibrary"), nil
	default:
		return "", fmt.Errorf("unsupported operating system %q", goos)
	}
}

// ApplicationPathsFor returns Claude Desktop's native application locations in
// discovery order. Linux has no official Claude Desktop application.
func ApplicationPathsFor(goos string, env map[string]string) []string {
	switch goos {
	case "darwin":
		paths := []string{}
		if home := env["HOME"]; home != "" {
			paths = append(paths, path.Join(home, "Applications", "Claude.app", "Contents", "MacOS", "Claude"))
		}
		return append(paths, path.Join("/", "Applications", "Claude.app", "Contents", "MacOS", "Claude"))
	case "windows":
		if env["LOCALAPPDATA"] == "" {
			return nil
		}
		return []string{platform.AppendWindowsPath(env["LOCALAPPDATA"], "AnthropicClaude", "claude.exe")}
	default:
		return nil
	}
}
