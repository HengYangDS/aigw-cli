//go:build !windows

package shims_test

import (
	"os"
	"path/filepath"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/shims"
)

// These scenarios depend on Unix-only filesystem semantics: a literal
// double-quote byte inside a path component (NTFS rejects it outright) and a
// POSIX executable permission bit (Windows never reports one via os.Stat, so
// requireExecutableBit can never be satisfied there). They stay off native
// Windows behind this build tag; internal/shims/windows_shim_runtime_test.go
// carries the real Windows-native equivalents.

func TestManagerParsesQuotedWindowsShimTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, `AIGW "portable"`, "aigw.exe")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("aigw executable"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := shims.Manager{GOOS: "windows", BinDir: filepath.Join(dir, "shim"), AIGWExecutable: target}
	if _, err := manager.EnableClaude(); err != nil {
		t.Fatal(err)
	}
	ready, err := manager.ClaudeShimReady()
	if err != nil || !ready {
		t.Fatalf("quoted Windows target readiness = %v, %v", ready, err)
	}
}

func TestManagerReportsOnlyAnOwnedClaudeShimAsReady(t *testing.T) {
	dir := t.TempDir()
	manager := shims.Manager{GOOS: "linux", BinDir: dir, AIGWExecutable: "/bin/sh"}
	ready, err := manager.ClaudeShimReady()
	if err != nil || ready {
		t.Fatalf("missing shim readiness = %v, %v", ready, err)
	}
	if _, err := manager.EnableClaude(); err != nil {
		t.Fatal(err)
	}
	ready, err = manager.ClaudeShimReady()
	if err != nil || !ready {
		t.Fatalf("owned shim readiness = %v, %v", ready, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "claude"), []byte("foreign launcher"), 0o755); err != nil {
		t.Fatal(err)
	}
	ready, err = manager.ClaudeShimReady()
	if err != nil || ready {
		t.Fatalf("foreign shim readiness = %v, %v", ready, err)
	}
}
