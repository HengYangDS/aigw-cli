// Package platform derives portable AIGW-owned paths from the target operating
// system and explicit environment inputs.
package platform

import (
	"fmt"
	"path"
	"strings"
)

// Paths is the complete host-derived path contract for one AIGW process.
type Paths struct {
	Config         string
	Data           string
	Secrets        string
	ClaudeSettings string
	InstallDir     string
	InstallName    string
}

// PathsFor derives every host-owned path from one explicit platform snapshot.
func PathsFor(goos string, env map[string]string) (Paths, error) {
	config, err := ConfigPathFor(goos, env)
	if err != nil {
		return Paths{}, err
	}
	data, err := DataDirFor(goos, env)
	if err != nil {
		return Paths{}, err
	}
	claudeSettings, err := ClaudeSettingsPathFor(goos, env)
	if err != nil {
		return Paths{}, err
	}
	installDir, err := UserBinDirFor(goos, env)
	if err != nil {
		return Paths{}, err
	}
	installName := "aigw"
	secrets := path.Join(data, "secrets")
	if goos == "windows" {
		installName += ".exe"
		secrets = appendWindowsPath(data, "secrets")
	}
	return Paths{
		Config:         config,
		Data:           data,
		Secrets:        secrets,
		ClaudeSettings: claudeSettings,
		InstallDir:     installDir,
		InstallName:    installName,
	}, nil
}

// ConfigPathFor returns the platform-native AIGW configuration path from an explicit environment.
func ConfigPathFor(goos string, env map[string]string) (string, error) {
	switch goos {
	case "darwin":
		home := env["HOME"]
		if home == "" {
			return "", fmt.Errorf("HOME is not set")
		}
		return path.Join(home, "Library", "Application Support", "aigw", "config.toml"), nil
	case "linux":
		base := env["XDG_CONFIG_HOME"]
		if base == "" {
			if env["HOME"] == "" {
				return "", fmt.Errorf("HOME and XDG_CONFIG_HOME are not set")
			}
			base = path.Join(env["HOME"], ".config")
		}
		return path.Join(base, "aigw", "config.toml"), nil
	case "windows":
		base := env["APPDATA"]
		if base == "" {
			return "", fmt.Errorf("APPDATA is not set")
		}
		return appendWindowsPath(base, "aigw", "config.toml"), nil
	default:
		return "", fmt.Errorf("unsupported operating system %q", goos)
	}
}

// DataDirFor returns the platform-native AIGW data directory from an explicit environment.
func DataDirFor(goos string, env map[string]string) (string, error) {
	switch goos {
	case "darwin":
		if env["HOME"] == "" {
			return "", fmt.Errorf("HOME is not set")
		}
		return path.Join(env["HOME"], "Library", "Application Support", "aigw"), nil
	case "linux":
		base := env["XDG_DATA_HOME"]
		if base == "" {
			if env["HOME"] == "" {
				return "", fmt.Errorf("HOME and XDG_DATA_HOME are not set")
			}
			base = path.Join(env["HOME"], ".local", "share")
		}
		return path.Join(base, "aigw"), nil
	case "windows":
		base := env["LOCALAPPDATA"]
		if base == "" {
			base = env["APPDATA"]
		}
		if base == "" {
			return "", fmt.Errorf("LOCALAPPDATA and APPDATA are not set")
		}
		return appendWindowsPath(base, "aigw"), nil
	default:
		return "", fmt.Errorf("unsupported operating system %q", goos)
	}
}

// ClaudeSettingsPathFor returns Claude Code's official per-user settings file.
// The path is derived only from the target platform and explicit environment;
// AIGW never assumes a workstation-specific directory.
func ClaudeSettingsPathFor(goos string, env map[string]string) (string, error) {
	var home string
	if goos == "windows" {
		home = env["USERPROFILE"]
		if home == "" {
			return "", fmt.Errorf("USERPROFILE is not set")
		}
		return appendWindowsPath(home, ".claude", "settings.json"), nil
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

// UserBinDirFor returns the platform-native per-user executable directory from an explicit environment.
func UserBinDirFor(goos string, env map[string]string) (string, error) {
	switch goos {
	case "darwin", "linux":
		home := env["HOME"]
		if home == "" {
			return "", fmt.Errorf("HOME is not set")
		}
		return path.Join(home, ".local", "bin"), nil
	case "windows":
		base := env["LOCALAPPDATA"]
		if base == "" {
			base = env["APPDATA"]
		}
		if base == "" {
			return "", fmt.Errorf("LOCALAPPDATA and APPDATA are not set")
		}
		return appendWindowsPath(base, "Programs", "aigw", "bin"), nil
	default:
		return "", fmt.Errorf("unsupported operating system %q", goos)
	}
}

// appendWindowsPath appends owned components without reinterpreting the base namespace.
func appendWindowsPath(base string, components ...string) string {
	base = strings.ReplaceAll(base, "/", `\`)
	return strings.TrimRight(base, `\`) + `\` + strings.Join(components, `\`)
}
