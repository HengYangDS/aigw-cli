// Package discovery classifies installed client executables and Codex homes
// without adopting or mutating them.
package discovery

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	configuration "aigw-cli/internal/configuration"
)

type Result struct {
	Executables map[string]string
	Surfaces    []Surface
}

// Executable returns the discovered real executable for one admitted client.
// The result is keyed by stable client ID so generic workflows do not require
// a new field or switch when an adapter is admitted.
func (r Result) Executable(client string) string { return r.Executables[client] }

type Discoverer interface{ Discover() Result }

type System struct {
	GOOS string
	Home string
	Path string
}

func Current() System {
	home, _ := os.UserHomeDir()
	return System{GOOS: runtime.GOOS, Home: home, Path: os.Getenv("PATH")}
}

func (s System) Discover() Result {
	result := Result{
		Executables: map[string]string{
			configuration.ClientClaude: s.find(configuration.ClientClaude),
			configuration.ClientCodex:  s.find(configuration.ClientCodex),
		},
		Surfaces: s.discoverSurfaces(),
	}
	return result
}

// ExecutableAvailable reports whether path identifies a runnable executable on
// the current platform.
func ExecutableAvailable(path string) (bool, error) {
	return executableAvailable(runtime.GOOS, path)
}

func (s System) find(name string) string {
	names := []string{name}
	if s.GOOS == "windows" {
		names = []string{name + ".exe", name + ".cmd", name + ".bat", name}
	}
	for _, dir := range filepath.SplitList(s.Path) {
		for _, candidate := range names {
			path := filepath.Join(dir, candidate)
			available, err := executableAvailable(s.GOOS, path)
			if err != nil || !available {
				continue
			}
			absolute, err := filepath.Abs(path)
			if err == nil {
				return absolute
			}
		}
	}
	return ""
}

func executableAvailable(goos, path string) (bool, error) {
	if strings.TrimSpace(path) == "" {
		return false, nil
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.IsDir() {
		return false, nil
	}
	return goos == "windows" || info.Mode().Perm()&0o111 != 0, nil
}
