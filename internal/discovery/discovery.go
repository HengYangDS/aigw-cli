// Package discovery classifies installed client executables and Codex homes
// without adopting or mutating them.
package discovery

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

// Executable returns the first runnable command with name on this host.
func (s System) Executable(name string) string { return s.find(name) }

// HomeDirectory returns the user home observed by this discovery source.
func (s System) HomeDirectory() string { return s.Home }

// FilePresent reports whether path currently names a non-directory entry.
func (s System) FilePresent(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && !info.IsDir()
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
