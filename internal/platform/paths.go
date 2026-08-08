// Package platform derives portable AIGW-owned paths from the target operating
// system and explicit environment inputs.
package platform

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

func ConfigPathFor(goos string, env map[string]string) (string, error) {
	switch goos {
	case "darwin":
		home := env["HOME"]
		if home == "" {
			return "", fmt.Errorf("HOME is not set")
		}
		return filepath.Join(home, "Library", "Application Support", "aigw", "config.toml"), nil
	case "linux":
		base := env["XDG_CONFIG_HOME"]
		if base == "" {
			if env["HOME"] == "" {
				return "", fmt.Errorf("HOME and XDG_CONFIG_HOME are not set")
			}
			base = filepath.Join(env["HOME"], ".config")
		}
		return filepath.Join(base, "aigw", "config.toml"), nil
	case "windows":
		base := env["APPDATA"]
		if base == "" {
			return "", fmt.Errorf("APPDATA is not set")
		}
		return windowsJoin(base, "aigw", "config.toml"), nil
	default:
		return "", fmt.Errorf("unsupported operating system %q", goos)
	}
}

func DataDirFor(goos string, env map[string]string) (string, error) {
	switch goos {
	case "darwin":
		if env["HOME"] == "" {
			return "", fmt.Errorf("HOME is not set")
		}
		return filepath.Join(env["HOME"], "Library", "Application Support", "aigw"), nil
	case "linux":
		base := env["XDG_DATA_HOME"]
		if base == "" {
			if env["HOME"] == "" {
				return "", fmt.Errorf("HOME and XDG_DATA_HOME are not set")
			}
			base = filepath.Join(env["HOME"], ".local", "share")
		}
		return filepath.Join(base, "aigw"), nil
	case "windows":
		base := env["LOCALAPPDATA"]
		if base == "" {
			base = env["APPDATA"]
		}
		if base == "" {
			return "", fmt.Errorf("LOCALAPPDATA and APPDATA are not set")
		}
		return windowsJoin(base, "aigw"), nil
	default:
		return "", fmt.Errorf("unsupported operating system %q", goos)
	}
}

func UserBinDirFor(goos string, env map[string]string) (string, error) {
	switch goos {
	case "darwin", "linux":
		home := env["HOME"]
		if home == "" {
			return "", fmt.Errorf("HOME is not set")
		}
		return filepath.Join(home, ".local", "bin"), nil
	case "windows":
		base := env["LOCALAPPDATA"]
		if base == "" {
			base = env["APPDATA"]
		}
		if base == "" {
			return "", fmt.Errorf("LOCALAPPDATA and APPDATA are not set")
		}
		return windowsJoin(base, "Programs", "aigw", "bin"), nil
	default:
		return "", fmt.Errorf("unsupported operating system %q", goos)
	}
}

// LauncherDirFor returns the AIGW-owned launcher directory. Unix launchers must not
// live in the shared user bin directory: package managers and workstation
// maintenance tools legitimately manage that surface. Windows already has a
// dedicated AIGW user-program directory, so its launcher remains there.
func LauncherDirFor(goos string, env map[string]string) (string, error) {
	if goos == "windows" {
		return UserBinDirFor(goos, env)
	}
	dataDir, err := DataDirFor(goos, env)
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "bin"), nil
}

func DefaultLauncherDirFor(goos string, env map[string]string, executable string) (string, error) {
	if value := strings.TrimSpace(env["AIGW_LAUNCHER_DIR"]); value != "" {
		return value, nil
	}
	if dir, err := LauncherDirFor(goos, env); err == nil {
		return dir, nil
	}
	return ExecutableDirFor(goos, executable), nil
}

func ExecutableDirFor(goos, executable string) string {
	if goos == "windows" {
		return windowsDirName(executable)
	}
	return path.Dir(executable)
}

func windowsDirName(name string) string {
	trimmed := strings.TrimRight(name, `\/`)
	if trimmed == "" {
		return name
	}
	idx := strings.LastIndexAny(trimmed, `\/`)
	if idx < 0 {
		return "."
	}
	if idx == 0 {
		return trimmed[:1]
	}
	if idx == 2 && trimmed[1] == ':' {
		return trimmed[:idx+1]
	}
	return trimmed[:idx]
}

func windowsJoin(parts ...string) string {
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(part, `\/`)
		if part != "" {
			clean = append(clean, part)
		}
	}
	if len(clean) == 0 {
		return ""
	}
	if len(parts[0]) > 0 && (strings.HasPrefix(parts[0], `\`) || strings.HasPrefix(parts[0], `/`)) {
		return `\` + strings.Join(clean, `\`)
	}
	return strings.Join(clean, `\`)
}
