package install

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
)

func TestInstallCommandUsesPlatformDefaultTarget(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "download", "aigw")
	target := filepath.Join(root, "bin", "aigw")
	if err := os.MkdirAll(filepath.Dir(source), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := new(bytes.Buffer)
	command := NewInstallCommand(invocation.Context{Executable: source, InstallTarget: target, Out: out})
	command.SetArgs(nil)
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != "current" {
		t.Fatalf("installed program = %q, %v", data, err)
	}
	if !strings.Contains(out.String(), target) || !strings.Contains(out.String(), "aigw setup") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestInstallCommandReportsUnavailableTargetAndCopyFailure(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "aigw")
	if err := os.WriteFile(source, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	command := NewInstallCommand(invocation.Context{Executable: source})
	command.SetArgs(nil)
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "target is unavailable") {
		t.Fatalf("missing target error = %v", err)
	}
	command = NewInstallCommand(invocation.Context{Executable: filepath.Join(root, "missing"), InstallTarget: filepath.Join(root, "bin", "aigw")})
	command.SetArgs(nil)
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "read portable") {
		t.Fatalf("copy failure = %v", err)
	}
}

func TestUninstallCommandDefaultsToRunningExecutableAndReportsFailure(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "aigw")
	if err := os.WriteFile(target, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := new(bytes.Buffer)
	command := NewUninstallCommand(invocation.Context{Executable: target, Out: out})
	command.SetArgs(nil)
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target remains: %v", err)
	}
	if !strings.Contains(out.String(), "credential-store secrets were preserved") {
		t.Fatalf("output = %q", out.String())
	}
	command = NewUninstallCommand(invocation.Context{Executable: ""})
	command.SetArgs(nil)
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "target is empty") {
		t.Fatalf("empty target error = %v", err)
	}
}

func TestInstallCopiesCurrentExecutableAndPreservesOnePredecessor(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source-aigw")
	target := filepath.Join(root, "bin", "aigw")
	if err := os.WriteFile(source, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("previous"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Install(source, target); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{target: "current", filepath.Join(root, "bin", ".aigw.previous"): "previous"} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatalf("%s=%q,%v want %q", path, got, err, want)
		}
	}
}

func TestUninstallRemovesOnlyOwnedProgramFiles(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "aigw")
	backup := filepath.Join(root, ".aigw.previous")
	foreign := filepath.Join(root, "foreign")
	for path := range map[string]bool{target: true, backup: true, foreign: true} {
		if err := os.WriteFile(path, []byte(path), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := Uninstall(target); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{target, backup} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("owned file retained: %s", path)
		}
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatalf("foreign file removed: %v", err)
	}
}

func TestInstallRejectsInvalidSourceAndSamePath(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "missing")
	if err := Install(missing, filepath.Join(root, "target")); err == nil {
		t.Fatal("missing source accepted")
	}
	source := filepath.Join(root, "aigw")
	if err := os.WriteFile(source, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Install(source, source); err == nil || !strings.Contains(err.Error(), "same path") {
		t.Fatalf("same path=%v", err)
	}
}

func TestInstallAcceptsPortableFileAndRejectsBlockedDestinations(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "aigw")
	if err := os.WriteFile(source, []byte("binary"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "bin", "aigw")
	if err := Install(source, target); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != "binary" {
		t.Fatalf("portable target = %q, %v", got, err)
	}
	blocked := filepath.Join(root, "blocked")
	if err := os.WriteFile(blocked, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Install(source, filepath.Join(blocked, "aigw")); err == nil || !strings.Contains(err.Error(), "installation directory") {
		t.Fatalf("blocked parent = %v", err)
	}
	if err := Install(source, root); err == nil || !strings.Contains(err.Error(), "read installed") {
		t.Fatalf("directory target = %v", err)
	}
}

func TestInstallRejectsDirectorySource(t *testing.T) {
	root := t.TempDir()
	if err := Install(root, filepath.Join(root, "target")); err == nil || !strings.Contains(err.Error(), "read portable") {
		t.Fatalf("directory source = %v", err)
	}
}

func TestInstallAndUninstallReportOwnedFileFailures(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "aigw")
	if err := os.WriteFile(source, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("previous"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(backupPath(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := Install(source, target); err == nil || !strings.Contains(err.Error(), "save previous") {
		t.Fatalf("blocked backup = %v", err)
	}
	if err := os.RemoveAll(backupPath(target)); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(target, 0o700); err == nil {
		t.Fatal("target file unexpectedly replaced by directory")
	}
	uninstallTarget := filepath.Join(root, "owned-directory")
	if err := os.Mkdir(uninstallTarget, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(uninstallTarget, "child"), []byte("foreign"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Uninstall(uninstallTarget); err == nil || !strings.Contains(err.Error(), "remove portable") {
		t.Fatalf("non-empty owned path = %v", err)
	}
}

func TestInstallReportsAtomicTargetReplacementFailure(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.WriteFile(source, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	original := writeFileAtomic
	writeFileAtomic = func(path string, data []byte, mode os.FileMode) error {
		if path == filepath.Join(root, "aigw") {
			return errors.New("replace failed")
		}
		return original(path, data, mode)
	}
	t.Cleanup(func() { writeFileAtomic = original })
	if err := Install(source, filepath.Join(root, "aigw")); err == nil || !strings.Contains(err.Error(), "replace failed") {
		t.Fatalf("atomic replacement error = %v", err)
	}
}

func TestBackupPathUsesWindowsExecutableSuffix(t *testing.T) {
	if got := backupPath(filepath.Join("root", "aigw.exe")); filepath.Base(got) != ".aigw.previous.exe" {
		t.Fatalf("backup path = %q", got)
	}
}
