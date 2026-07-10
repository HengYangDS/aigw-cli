package discovery

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const claudeShimMarker = "AIGW managed Claude shim"

type Result struct {
	ClaudeExecutable string
	CodexExecutable  string
	CodexTargets     []string
}

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
		ClaudeExecutable: s.find("claude", true),
		CodexExecutable:  s.find("codex", false),
	}
	if s.GOOS == "darwin" {
		if result.CodexExecutable == "" {
			result.CodexExecutable = firstExecutable("/Applications/Codex.app/Contents/Resources/codex")
		}
	}
	for _, path := range s.codexConfigCandidates() {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			result.CodexTargets = append(result.CodexTargets, path)
		}
	}
	sort.Strings(result.CodexTargets)
	return result
}

func (s System) find(name string, skipManagedClaude bool) string {
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
			if skipManagedClaude {
				data, _ := os.ReadFile(path)
				if strings.Contains(string(data), claudeShimMarker) {
					continue
				}
			}
			if s.GOOS == "windows" || info.Mode()&0o111 != 0 {
				absolute, _ := filepath.Abs(path)
				return absolute
			}
		}
	}
	return ""
}

func (s System) codexConfigCandidates() []string {
	paths := []string{filepath.Join(s.Home, ".codex", "config.toml")}
	if s.GOOS == "darwin" {
		paths = append(paths,
			filepath.Join(s.Home, "Library", "Caches", "JetBrains", "PyCharm2026.1", "aia", "codex", "config.toml"),
			filepath.Join(s.Home, "Library", "Application Support", "JetBrains", "Air", ".codex", "config.toml"),
		)
	}
	return paths
}

func firstExecutable(paths ...string) string {
	for _, path := range paths {
		if info, err := os.Stat(path); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return path
		}
	}
	return ""
}
