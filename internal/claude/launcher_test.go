package claude_test

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"aigw-cli/internal/claude"
)

func TestLauncherDoesNotExposeLegacyLauncherCompatibilityState(t *testing.T) {
	if _, found := reflect.TypeOf(claude.Launcher{}).FieldByName("LegacyBinDir"); found {
		t.Fatal("Claude launcher manager must not retain a legacy shared-bin compatibility path")
	}
}

func TestLauncherCreatesAndRemovesOwnedUnixClaudeLauncher(t *testing.T) {
	dir := t.TempDir()
	manager := claude.Launcher{GOOS: "linux", BinDir: dir, AIGWExecutable: filepath.Join(dir, "aigw")}
	path, err := manager.EnableClaude()
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "AIGW managed Claude launcher") || !strings.Contains(string(data), "__run-claude") {
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
		t.Fatalf("owned launcher still exists: %v", err)
	}
}

func TestEnableClaudeRestoresExistingLauncherWhenActivationFails(t *testing.T) {
	home := t.TempDir()
	binDir := t.TempDir()
	manager := claude.Launcher{GOOS: "linux", Home: home, Shell: "/bin/zsh", BinDir: binDir, AIGWExecutable: "/bin/sh"}
	path, err := manager.EnableClaude()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(home, ".zshrc")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(home, 0o500); err != nil {
		t.Fatal(err)
	}
	manager.AIGWExecutable = "/bin/echo"
	_, enableErr := manager.EnableClaude()
	if err := os.Chmod(home, 0o700); err != nil {
		t.Fatal(err)
	}
	if enableErr == nil {
		t.Skip("filesystem permissions did not reject the activation write")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) || info.Mode().Perm() != 0o700 {
		t.Fatalf("launcher was not restored after activation failure: mode=%o", info.Mode().Perm())
	}
}

func TestLauncherCreatesWindowsCommandLauncher(t *testing.T) {
	dir := t.TempDir()
	manager := claude.Launcher{GOOS: "windows", BinDir: dir, AIGWExecutable: filepath.Join(dir, "aigw.exe")}
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

func TestLauncherRejectsWindowsLauncherWhoseTargetIsUnavailable(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "portable", "aigw.exe")
	manager := claude.Launcher{GOOS: "windows", BinDir: filepath.Join(dir, "shim"), AIGWExecutable: target}
	if _, err := manager.EnableClaude(); err != nil {
		t.Fatal(err)
	}
	ready, err := manager.ClaudeLauncherReady()
	if ready || err == nil || !strings.Contains(err.Error(), "target is unavailable") {
		t.Fatalf("unavailable Windows target readiness = %v, %v", ready, err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("aigw executable"), 0o600); err != nil {
		t.Fatal(err)
	}
	ready, err = manager.ClaudeLauncherReady()
	if err != nil || !ready {
		t.Fatalf("available Windows target readiness = %v, %v", ready, err)
	}
}

func TestLauncherRefusesForeignClaudeLauncher(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "claude")
	if err := os.WriteFile(path, []byte("foreign launcher"), 0o755); err != nil {
		t.Fatal(err)
	}
	manager := claude.Launcher{GOOS: "linux", BinDir: dir, AIGWExecutable: filepath.Join(dir, "aigw")}
	if _, err := manager.EnableClaude(); err == nil || !strings.Contains(err.Error(), "not owned") {
		t.Fatalf("error = %v", err)
	}
	if err := manager.DisableClaude(); err == nil || !strings.Contains(err.Error(), "not owned") {
		t.Fatalf("disable error = %v", err)
	}
}

func TestLauncherRejectsOwnedUnixLauncherThatTargetsTemporaryDirectory(t *testing.T) {
	dir := t.TempDir()
	manager := claude.Launcher{GOOS: "linux", BinDir: dir, AIGWExecutable: "/tmp/aigw-build"}
	if _, err := manager.EnableClaude(); err != nil {
		t.Fatal(err)
	}
	ready, err := manager.ClaudeLauncherReady()
	if err == nil || ready {
		t.Fatalf("temporary AIGW target readiness = %v, %v", ready, err)
	}
	if !strings.Contains(err.Error(), "temporary directory") {
		t.Fatalf("error = %v", err)
	}
}

func TestLauncherRefusesToWriteClaudeLauncherFromTemporaryBuildExecutable(t *testing.T) {
	for _, executable := range []string{
		"/tmp/aigw-go-preview-20260718/23/aigw",
		"/var/folders/example/T/go-build123/b001/exe/aigw",
		`C:\Users\example\AppData\Local\Temp\go-build123\b001\exe\aigw.exe`,
	} {
		t.Run(filepath.Base(filepath.Dir(executable)), func(t *testing.T) {
			dir := t.TempDir()
			manager := claude.Launcher{GOOS: "linux", BinDir: dir, AIGWExecutable: executable}
			if _, err := manager.EnableClaude(); err == nil || !strings.Contains(err.Error(), "temporary build executable") {
				t.Fatalf("EnableClaude() error = %v", err)
			}
			if _, err := os.Stat(filepath.Join(dir, "claude")); !os.IsNotExist(err) {
				t.Fatalf("temporary build unexpectedly wrote Claude launcher: %v", err)
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
	manager := claude.Launcher{GOOS: "darwin", Home: home, Shell: "/bin/zsh", BinDir: filepath.Join(home, "Library", "Application Support", "aigw", "bin")}
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
	original := "# >>> AIGW Claude launcher PATH >>>\nexport PATH='" + dedicated + `':$PATH` + "\n# <<< AIGW Claude launcher PATH <<<\n"
	if err := os.WriteFile(profilePath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	manager := claude.Launcher{GOOS: "darwin", Home: home, Shell: "/bin/zsh", BinDir: dedicated}
	if err := manager.DisableClaude(); err == nil || !strings.Contains(err.Error(), "not owned") {
		t.Fatalf("disable error = %v", err)
	}
	got, err := os.ReadFile(profilePath)
	if err != nil || string(got) != original {
		t.Fatalf("activation changed after foreign-launcher refusal: %q, %v", got, err)
	}
}
