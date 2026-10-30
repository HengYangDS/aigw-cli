package claude

import (
	"fmt"
	"path"

	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/platform"
)

// SettingsPathFor returns Claude Code's official per-user settings file.
// The path is derived only from the target platform and explicit environment;
// AIGW never assumes a workstation-specific directory.
func SettingsPathFor(goos string, env map[string]string) (string, error) {
	var home string
	if goos == "windows" {
		home = env["USERPROFILE"]
		if home == "" {
			return "", fmt.Errorf("USERPROFILE is not set")
		}
		return platform.AppendWindowsPath(home, ".claude", "settings.json"), nil
	}
	if goos != "darwin" && goos != "linux" {
		return "", fmt.Errorf("unsupported operating system %q", goos)
	}
	home = env["HOME"]
	if home == "" {
		return "", fmt.Errorf("HOME is not set")
	}
	return path.Join(home, ".claude", "settings.json"), nil
}

// SettingsPath returns Claude Code's settings file on the observed host. An
// unobserved host names no file, which every settings operation refuses.
func SettingsPath(host discovery.Host) (string, error) {
	if host.GOOS == "" {
		return "", nil
	}
	return SettingsPathFor(host.GOOS, map[string]string{"HOME": host.Getenv("HOME"), "USERPROFILE": host.Getenv("USERPROFILE")})
}

// ValidateHost refuses a host on which Claude Code's settings file cannot be
// derived. Like AIGW's own platform paths, it matches variable names exactly.
func (claudeAdapter) ValidateHost(host discovery.Host) error {
	_, err := SettingsPathFor(host.GOOS, host.Env)
	return err
}
