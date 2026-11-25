// Package platform derives portable AIGW-owned paths from the target operating
// system and explicit environment inputs. Its guarded, byte-exact filesystem
// mutation and compensation primitives serve the configuration store, the
// credential entrypoint, the program's installation and the client adapters'
// native files.
package platform

import (
	"fmt"
	"path"
)

// Paths is the complete host-derived path contract for one AIGW process.
type Paths struct {
	Config      string
	Data        string
	Secrets     string
	InstallDir  string
	InstallName string
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
	installDir, err := UserBinDirFor(goos, env)
	if err != nil {
		return Paths{}, err
	}
	return Paths{
		Config:      config,
		Data:        data,
		Secrets:     path.Join(data, "secrets"),
		InstallDir:  installDir,
		InstallName: "aigw",
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
		return path.Join(home, "Library", "Application Support", "aigw", "aigw.toml"), nil
	case "linux":
		base := env["XDG_CONFIG_HOME"]
		if base == "" {
			if env["HOME"] == "" {
				return "", fmt.Errorf("HOME and XDG_CONFIG_HOME are not set")
			}
			base = path.Join(env["HOME"], ".config")
		}
		return path.Join(base, "aigw", "aigw.toml"), nil
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
	default:
		return "", fmt.Errorf("unsupported operating system %q", goos)
	}
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
	default:
		return "", fmt.Errorf("unsupported operating system %q", goos)
	}
}
