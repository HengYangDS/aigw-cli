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

// Result contains discovered client executables and independently addressable configuration surfaces.
type Result struct {
	Executables map[string]string
	Surfaces    []Surface
}

// Executable returns the discovered real executable for one admitted client.
// The result is keyed by stable client ID so generic workflows do not require
// a new field or switch when an adapter is admitted.
func (r Result) Executable(client string) string { return r.Executables[client] }

// Discoverer resolves currently available client executables and configuration surfaces.
type Discoverer interface{ Discover() Result }

// System discovers clients from one explicit operating-system, home, and search-path context.
type System struct {
	GOOS       string
	Home       string
	CodexHome  string
	HermesHome string
	Path       string
}

// Current returns the discovery context derived from the current process environment.
func Current() System {
	home, _ := os.UserHomeDir()
	return System{GOOS: runtime.GOOS, Home: home, CodexHome: os.Getenv("CODEX_HOME"), HermesHome: os.Getenv("HERMES_HOME"), Path: os.Getenv("PATH")}
}

// Executable returns the first runnable command with name on this host.
func (s System) Executable(name string) string { return s.find(name) }

// CodexHomeDirectory returns the explicit Codex Home or its platform default.
func (s System) CodexHomeDirectory() string {
	if s.CodexHome != "" {
		return s.CodexHome
	}
	return filepath.Join(s.Home, ".codex")
}

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

// HermesHomeDirectory resolves the native Hermes configuration root without creating it.
func (s System) HermesHomeDirectory() string {
	if s.HermesHome != "" {
		return s.HermesHome
	}
	return filepath.Join(s.Home, ".hermes")
}
