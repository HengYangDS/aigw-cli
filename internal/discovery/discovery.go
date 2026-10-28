// Package discovery observes the host for client discovery without adopting or
// mutating anything: executables on the search path, file presence, and the
// platform from which each client adapter derives its own native paths.
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

// Host is the platform that one AIGW process observes: its operating system,
// the user's home directory and the environment. Adapters derive their native
// client paths from it.
type Host struct {
	GOOS string
	Home string
	Env  map[string]string
}

// Getenv returns one variable of the observed environment, or an empty string.
// Windows variable names are case-insensitive, as the operating system reads them.
func (h Host) Getenv(name string) string {
	if h.GOOS == "windows" {
		for key, value := range h.Env {
			if strings.EqualFold(key, name) {
				return value
			}
		}
		return ""
	}
	return h.Env[name]
}

// System discovers clients from one explicit host and executable search path.
type System struct {
	GOOS string
	Home string
	Env  map[string]string
	Path string
}

// Current returns the discovery context derived from the current process environment.
func Current() System {
	home, _ := os.UserHomeDir()
	env := map[string]string{}
	for _, entry := range os.Environ() {
		if key, value, ok := strings.Cut(entry, "="); ok && key != "" {
			env[key] = value
		}
	}
	return System{GOOS: runtime.GOOS, Home: home, Env: env, Path: os.Getenv("PATH")}
}

// Platform returns the observed host from which adapters derive their native paths.
func (s System) Platform() Host { return Host{GOOS: s.GOOS, Home: s.Home, Env: s.Env} }

// Executable returns the first runnable command with name on this host.
func (s System) Executable(name string) string { return s.find(name) }

// Runnable reports whether path names a runnable executable on this host.
func (s System) Runnable(path string) bool {
	available, _ := executableAvailable(s.GOOS, path)
	return available
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
