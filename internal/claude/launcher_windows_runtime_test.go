//go:build windows

package claude_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/claude"
)

func TestMain(m *testing.M) {
	if os.Getenv("AIGW_TEST_WINDOWS_LAUNCHER_HELPER") == "1" {
		if len(os.Args) < 2 || os.Args[1] != "__run-claude" {
			os.Exit(9)
		}
		fmt.Println("AIGW_WINDOWS_LAUNCHER_OK")
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func persistentTestExecutable(t *testing.T, directory string) string {
	t.Helper()
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	persistent := filepath.Join(directory, "aigw.exe")
	if err := os.WriteFile(persistent, data, 0o755); err != nil {
		t.Fatal(err)
	}
	return persistent
}

func runWindowsLauncher(t *testing.T, path string) []byte {
	t.Helper()
	command := exec.Command(path)
	command.Env = append(os.Environ(), "AIGW_TEST_WINDOWS_LAUNCHER_HELPER=1")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run Windows Claude launcher: %v: %s", err, output)
	}
	return output
}

func TestLauncherCreatesWindowsCommandLauncherThatCanRunAIGW(t *testing.T) {
	dir := t.TempDir()
	manager := claude.Launcher{GOOS: "windows", BinDir: dir, AIGWExecutable: persistentTestExecutable(t, filepath.Join(dir, "target"))}
	path, err := manager.EnableClaude()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(path) != ".cmd" {
		t.Fatalf("launcher path = %q, want .cmd", path)
	}
	output := runWindowsLauncher(t, path)
	if !strings.Contains(string(output), "AIGW_WINDOWS_LAUNCHER_OK") {
		t.Fatalf("Windows Claude launcher output = %q", output)
	}
}

// TestLauncherReportsOnlyAnOwnedWindowsClaudeLauncherAsReady is the real-Windows
// counterpart of the Unix owned/foreign readiness contract: it exercises
// ClaudeLauncherReady's marker and target checks against actual claude.cmd files
// on disk instead of the POSIX executable-bit path, which Windows does not
// support (os.Stat never reports an execute bit there).
func TestLauncherReportsOnlyAnOwnedWindowsClaudeLauncherAsReady(t *testing.T) {
	dir := t.TempDir()
	manager := claude.Launcher{GOOS: "windows", BinDir: dir, AIGWExecutable: persistentTestExecutable(t, filepath.Join(dir, "target"))}
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
	if err := os.WriteFile(filepath.Join(dir, "claude.cmd"), []byte("@echo off\r\nREM foreign launcher\r\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ready, err = manager.ClaudeLauncherReady()
	if err != nil || ready {
		t.Fatalf("foreign launcher readiness = %v, %v", ready, err)
	}
}

// TestLauncherHandlesWindowsTargetRequiringQuotesOnRealPath exercises the
// real reason claudeContent() wraps the AIGW target in double quotes: a path
// containing spaces and other shell-significant characters. A literal
// embedded double-quote byte, which the "" escape in claudeContent() also
// defends against, cannot exist in any real NTFS path (Windows rejects the
// character outright), so that escaping is covered as pure string parsing by
// TestWindowsLauncherTargetParsing instead.
func TestLauncherHandlesWindowsTargetRequiringQuotesOnRealPath(t *testing.T) {
	root := t.TempDir()
	targetDir := filepath.Join(root, "AIGW Portable (x64) & Co")
	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		t.Fatal(err)
	}
	target := persistentTestExecutable(t, targetDir)
	manager := claude.Launcher{GOOS: "windows", BinDir: filepath.Join(root, "shim"), AIGWExecutable: target}
	path, err := manager.EnableClaude()
	if err != nil {
		t.Fatal(err)
	}
	ready, err := manager.ClaudeLauncherReady()
	if err != nil || !ready {
		t.Fatalf("spaced Windows target readiness = %v, %v", ready, err)
	}
	output := runWindowsLauncher(t, path)
	if !strings.Contains(string(output), "AIGW_WINDOWS_LAUNCHER_OK") {
		t.Fatalf("Windows Claude launcher output = %q", output)
	}
}
