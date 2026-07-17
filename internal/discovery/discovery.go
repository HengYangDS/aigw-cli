package discovery

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const claudeShimMarker = "AIGW managed Claude shim"

type Result struct {
	ClaudeExecutable string
	CodexExecutable  string
	Surfaces         []Surface
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
		Surfaces:         s.discoverSurfaces(),
	}
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
