//go:build !windows

package claude_test

import (
	"os"
	"path/filepath"
	"testing"

	"aigw-cli/internal/claude"
)

// These scenarios depend on Unix-only filesystem semantics: a literal
// double-quote byte inside a path component (NTFS rejects it outright) and a
// POSIX executable permission bit (Windows never reports one via os.Stat, so
// requireExecutableBit can never be satisfied there). They stay off native
// Windows behind this build tag; launcher_windows_runtime_test.go
// carries the real Windows-native equivalents.

func TestLauncherParsesQuotedWindowsLauncherTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, `AIGW "portable"`, "aigw.exe")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("aigw executable"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := claude.Launcher{GOOS: "windows", BinDir: filepath.Join(dir, "shim"), AIGWExecutable: target}
	if _, err := manager.EnableClaude(); err != nil {
		t.Fatal(err)
	}
	ready, err := manager.ClaudeLauncherReady()
	if err != nil || !ready {
		t.Fatalf("quoted Windows target readiness = %v, %v", ready, err)
	}
}

func TestLauncherReportsOnlyAnOwnedClaudeLauncherAsReady(t *testing.T) {
	dir := t.TempDir()
	manager := claude.Launcher{GOOS: "linux", BinDir: dir, AIGWExecutable: "/bin/sh"}
	ready, err := manager.ClaudeLauncherReady()
	if err != nil || ready {
		t.Fatalf("missing launcher readiness = %v, %v", ready, err)
	}
	if _, err := manager.EnableClaude(); err != nil {
		t.Fatal(err)
	}
	ready, err = manager.ClaudeLauncherReady()
	if err != nil || !ready {
		t.Fatalf("owned launcher readiness = %v, %v", ready, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "claude"), []byte("foreign launcher"), 0o755); err != nil {
		t.Fatal(err)
	}
	ready, err = manager.ClaudeLauncherReady()
	if err != nil || ready {
		t.Fatalf("foreign launcher readiness = %v, %v", ready, err)
	}
}
