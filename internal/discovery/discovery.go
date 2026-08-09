// Package discovery classifies installed client executables and Codex homes
// without adopting or mutating them.
package discovery

import (
	configuration "aigw-cli/internal/configuration"
	"os"
	"path/filepath"
	"runtime"
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

func (s System) find(name string) string {
	names := []string{name}
	if s.GOOS == "windows" {
		names = []string{name + ".exe", name + ".cmd", name + ".bat", name}
	}
	for _, dir := range filepath.SplitList(s.Path) {
		for _, candidate := range names {
			path := filepath.Join(dir, candidate)
			info, err := os.Stat(path)
			if err != nil || info.IsDir() {
				continue
			}
			if s.GOOS == "windows" || info.Mode()&0o111 != 0 {
				absolute, _ := filepath.Abs(path)
				return absolute
			}
		}
	}
	return ""
}
