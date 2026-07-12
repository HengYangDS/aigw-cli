package shims_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/shims"
)

func TestManagerDoesNotExposeLegacyShimCompatibilityState(t *testing.T) {
	if _, found := reflect.TypeOf(shims.Manager{}).FieldByName("LegacyBinDir"); found {
		t.Fatal("Claude shim manager must not retain a legacy shared-bin compatibility path")
	}
}

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

func TestManagerRejectsOwnedUnixShimThatTargetsTemporaryDirectory(t *testing.T) {
	dir := t.TempDir()
	manager := shims.Manager{GOOS: "linux", BinDir: dir, AIGWExecutable: "/tmp/aigw-build"}
	if _, err := manager.EnableClaude(); err != nil {
		t.Fatal(err)
	}
	ready, err := manager.ClaudeShimReady()
	if err == nil || ready {
		t.Fatalf("temporary AIGW target readiness = %v, %v", ready, err)
	}
	if !strings.Contains(err.Error(), "temporary directory") {
		t.Fatalf("error = %v", err)
	}
}

func TestDisableDoesNotRewriteShellProfilesWithoutAIGWBlock(t *testing.T) {
	home := t.TempDir()
	profilePath := filepath.Join(home, ".zshrc")
	original := "export PATH=\"/custom:$PATH\"\n"
	if err := os.WriteFile(profilePath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	manager := shims.Manager{GOOS: "darwin", Home: home, Shell: "/bin/zsh", BinDir: filepath.Join(home, "Library", "Application Support", "aigw", "bin")}
	if err := manager.DisableClaude(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(profilePath)
	if err != nil || string(got) != original {
		t.Fatalf("profile changed without an owned block: %q, %v", got, err)
	}
}

func TestDisablePreservesActivationWhenDedicatedLauncherIsForeign(t *testing.T) {
	home := t.TempDir()
	dedicated := filepath.Join(home, "Library", "Application Support", "aigw", "bin")
	if err := os.MkdirAll(dedicated, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dedicated, "claude"), []byte("foreign launcher"), 0o755); err != nil {
		t.Fatal(err)
	}
	profilePath := filepath.Join(home, ".zshrc")
	original := "# >>> AIGW Claude shim PATH >>>\nexport PATH='" + dedicated + `':$PATH` + "\n# <<< AIGW Claude shim PATH <<<\n"
	if err := os.WriteFile(profilePath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	manager := shims.Manager{GOOS: "darwin", Home: home, Shell: "/bin/zsh", BinDir: dedicated}
	if err := manager.DisableClaude(); err == nil || !strings.Contains(err.Error(), "not owned") {
		t.Fatalf("disable error = %v", err)
	}
	got, err := os.ReadFile(profilePath)
	if err != nil || string(got) != original {
		t.Fatalf("activation changed after foreign-launcher refusal: %q, %v", got, err)
	}
}
