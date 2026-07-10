package shims_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/shims"
)

func TestManagerCreatesAndRemovesOwnedUnixClaudeShim(t *testing.T) {
	dir := t.TempDir()
	manager := shims.Manager{GOOS: "linux", BinDir: dir, AIGWExecutable: filepath.Join(dir, "aigw")}
	path, err := manager.EnableClaude()
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "AIGW managed Claude shim") || !strings.Contains(string(data), "__run-claude") {
		t.Fatalf("shim = %s", data)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
	if err := manager.DisableClaude(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("owned shim still exists: %v", err)
	}
}

func TestManagerCreatesWindowsCommandShim(t *testing.T) {
	dir := t.TempDir()
	manager := shims.Manager{GOOS: "windows", BinDir: dir, AIGWExecutable: filepath.Join(dir, "aigw.exe")}
	path, err := manager.EnableClaude()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "claude.cmd" {
		t.Fatalf("path = %s", path)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "%~dp0aigw.exe") || !strings.Contains(string(data), "__run-claude") {
		t.Fatalf("Windows shim = %s", data)
	}
}

func TestManagerRefusesForeignClaudeShim(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "claude")
	if err := os.WriteFile(path, []byte("foreign launcher"), 0o755); err != nil {
		t.Fatal(err)
	}
	manager := shims.Manager{GOOS: "linux", BinDir: dir, AIGWExecutable: filepath.Join(dir, "aigw")}
	if _, err := manager.EnableClaude(); err == nil || !strings.Contains(err.Error(), "not owned") {
		t.Fatalf("error = %v", err)
	}
	if err := manager.DisableClaude(); err == nil || !strings.Contains(err.Error(), "not owned") {
		t.Fatalf("disable error = %v", err)
	}
}
