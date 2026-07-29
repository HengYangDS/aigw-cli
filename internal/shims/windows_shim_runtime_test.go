//go:build windows

package shims_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/shims"
)

// persistentTestExecutable copies the running test binary (which `go test`
// always builds under a "go-build*" temporary directory) into a directory
// name that does not match EnableClaude's ephemeral-build-executable guard.
// It stands in for an installed, persistent AIGW binary so these tests can
// exercise the real Windows launcher without tripping the very protection
// they are not meant to test.
func persistentTestExecutable(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	persistent := filepath.Join(t.TempDir(), "aigw.exe")
	if err := os.WriteFile(persistent, data, 0o755); err != nil {
		t.Fatal(err)
	}
	return persistent
}

func TestManagerCreatesWindowsCommandShimThatCanRunAIGW(t *testing.T) {
	dir := t.TempDir()
	manager := shims.Manager{GOOS: "windows", BinDir: dir, AIGWExecutable: persistentTestExecutable(t)}
	path, err := manager.EnableClaude()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(path) != ".cmd" {
		t.Fatalf("shim path = %q, want .cmd", path)
	}
	output, err := exec.Command(path, "-test.run=^$").CombinedOutput()
	if err != nil {
		t.Fatalf("run Windows Claude shim: %v: %s", err, output)
	}
	if !strings.Contains(string(output), "PASS") {
		t.Fatalf("Windows Claude shim output = %q", output)
	}
}

// TestManagerReportsOnlyAnOwnedWindowsClaudeShimAsReady is the real-Windows
// counterpart of the Unix owned/foreign readiness contract: it exercises
// ClaudeShimReady's marker and target checks against actual claude.cmd files
// on disk instead of the POSIX executable-bit path, which Windows does not
// support (os.Stat never reports an execute bit there).
func TestManagerReportsOnlyAnOwnedWindowsClaudeShimAsReady(t *testing.T) {
	dir := t.TempDir()
	manager := shims.Manager{GOOS: "windows", BinDir: dir, AIGWExecutable: persistentTestExecutable(t)}
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
	if err := os.WriteFile(filepath.Join(dir, "claude.cmd"), []byte("@echo off\r\nREM foreign launcher\r\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ready, err = manager.ClaudeShimReady()
	if err != nil || ready {
		t.Fatalf("foreign shim readiness = %v, %v", ready, err)
	}
}

// TestManagerHandlesWindowsTargetRequiringQuotesOnRealPath exercises the
// real reason claudeContent() wraps the AIGW target in double quotes: a path
// containing spaces and other shell-significant characters. A literal
// embedded double-quote byte, which the "" escape in claudeContent() also
// defends against, cannot exist in any real NTFS path (Windows rejects the
// character outright), so that escaping is covered as pure string parsing by
// TestWindowsShimTargetParsing instead.
func TestManagerHandlesWindowsTargetRequiringQuotesOnRealPath(t *testing.T) {
	root := t.TempDir()
	targetDir := filepath.Join(root, "AIGW Portable (x64) & Co")
	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(targetDir, "aigw.exe")
	if err := os.WriteFile(target, data, 0o755); err != nil {
		t.Fatal(err)
	}
	manager := shims.Manager{GOOS: "windows", BinDir: filepath.Join(root, "shim"), AIGWExecutable: target}
	path, err := manager.EnableClaude()
	if err != nil {
		t.Fatal(err)
	}
	ready, err := manager.ClaudeShimReady()
	if err != nil || !ready {
		t.Fatalf("spaced Windows target readiness = %v, %v", ready, err)
	}
	output, err := exec.Command(path, "-test.run=^$").CombinedOutput()
	if err != nil {
		t.Fatalf("run Windows Claude shim with quoted target: %v: %s", err, output)
	}
	if !strings.Contains(string(output), "PASS") {
		t.Fatalf("Windows Claude shim output = %q", output)
	}
}
