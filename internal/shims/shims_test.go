package shims_test

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
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
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(path)
		if info.Mode().Perm() != 0o755 {
			t.Fatalf("mode = %o", info.Mode().Perm())
		}
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
	if !strings.Contains(string(data), `"`+filepath.Join(dir, "aigw.exe")+`" __run-claude`) {
		t.Fatalf("Windows shim must target the configured AIGW executable, got %s", data)
	}
	if strings.Contains(string(data), "%~dp0aigw.exe") {
		t.Fatalf("Windows shim must not assume the AIGW executable shares its directory: %s", data)
	}
}

func TestManagerRejectsWindowsShimWhoseTargetIsUnavailable(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "portable", "aigw.exe")
	manager := shims.Manager{GOOS: "windows", BinDir: filepath.Join(dir, "shim"), AIGWExecutable: target}
	if _, err := manager.EnableClaude(); err != nil {
		t.Fatal(err)
	}
	ready, err := manager.ClaudeShimReady()
	if ready || err == nil || !strings.Contains(err.Error(), "target is unavailable") {
		t.Fatalf("unavailable Windows target readiness = %v, %v", ready, err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("aigw executable"), 0o600); err != nil {
		t.Fatal(err)
	}
	ready, err = manager.ClaudeShimReady()
	if err != nil || !ready {
		t.Fatalf("available Windows target readiness = %v, %v", ready, err)
	}
}

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

func TestManagerRefusesToWriteClaudeShimFromTemporaryBuildExecutable(t *testing.T) {
	for _, executable := range []string{
		"/tmp/aigw-go-preview-20260718/23/aigw",
		"/var/folders/example/T/go-build123/b001/exe/aigw",
		`C:\Users\example\AppData\Local\Temp\go-build123\b001\exe\aigw.exe`,
	} {
		t.Run(filepath.Base(filepath.Dir(executable)), func(t *testing.T) {
			dir := t.TempDir()
			manager := shims.Manager{GOOS: "linux", BinDir: dir, AIGWExecutable: executable}
			if _, err := manager.EnableClaude(); err == nil || !strings.Contains(err.Error(), "temporary build executable") {
				t.Fatalf("EnableClaude() error = %v", err)
			}
			if _, err := os.Stat(filepath.Join(dir, "claude")); !os.IsNotExist(err) {
				t.Fatalf("temporary build unexpectedly wrote Claude shim: %v", err)
			}
		})
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
