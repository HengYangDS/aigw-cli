// Package discovery classifies installed client executables and Codex homes
// without adopting or mutating them.
package discovery

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"aigw-cli/internal/platform"
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
	GOOS                 string
	Home                 string
	CodexHome            string
	HermesHome           string
	ClaudeDesktopApp     string
	ClaudeDesktopLibrary string
	XDGConfigHome        string
	LocalAppData         string
	Path                 string
}

// Current returns the discovery context derived from the current process environment.
func Current() System {
	home, _ := os.UserHomeDir()
	return System{
		GOOS: runtime.GOOS, Home: home, CodexHome: os.Getenv("CODEX_HOME"), HermesHome: os.Getenv("HERMES_HOME"),
		XDGConfigHome: os.Getenv("XDG_CONFIG_HOME"), LocalAppData: os.Getenv("LOCALAPPDATA"), Path: os.Getenv("PATH"),
	}
}

// Executable returns the first runnable command with name on this host.
func (s System) Executable(name string) string { return s.find(name) }

// ClaudeDesktopExecutable returns the installed native Claude Desktop executable.
func (s System) ClaudeDesktopExecutable() string {
	if available, _ := executableAvailable(s.GOOS, s.ClaudeDesktopApp); available {
		return s.ClaudeDesktopApp
	}
	candidates := []string{}
	switch s.GOOS {
	case "darwin":
		candidates = []string{
			filepath.Join(string(filepath.Separator), "Applications", "Claude.app", "Contents", "MacOS", "Claude"),
			filepath.Join(s.Home, "Applications", "Claude.app", "Contents", "MacOS", "Claude"),
		}
	case "linux":
		return s.find("claude-desktop")
	case "windows":
		candidates = []string{filepath.Join(s.LocalAppData, "AnthropicClaude", "claude.exe")}
	}
	for _, candidate := range candidates {
		if available, _ := executableAvailable(s.GOOS, candidate); available {
			return candidate
		}
	}
	return ""
}

// ClaudeDesktopLibraryDirectory returns the native third-party configuration root.
func (s System) ClaudeDesktopLibraryDirectory() string {
	if s.ClaudeDesktopLibrary != "" {
		return s.ClaudeDesktopLibrary
	}
	library, _ := platform.ClaudeDesktopLibraryPathFor(s.GOOS, map[string]string{
		"HOME": s.Home, "XDG_CONFIG_HOME": s.XDGConfigHome, "LOCALAPPDATA": s.LocalAppData,
	})
	return library
}

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
