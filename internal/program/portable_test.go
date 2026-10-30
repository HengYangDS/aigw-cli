package program

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallCopiesCurrentExecutableAndPreservesOnePredecessor(t *testing.T) {
	for _, previous := range []string{"", "previous"} {
		t.Run("predecessor="+previous, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source-aigw")
			target := filepath.Join(root, "bin", "aigw")
			backup := filepath.Join(root, "bin", ".aigw.previous")
			if err := os.WriteFile(source, []byte("current"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				t.Fatal(err)
			}
			wantFiles := map[string]string{target: "current"}
			if previous != "" {
				if err := os.WriteFile(target, []byte(previous), 0o755); err != nil {
					t.Fatal(err)
				}
				wantFiles[backup] = previous
			}
			for attempt := range 2 {
				if err := Install(source, target); err != nil {
					t.Fatal(err)
				}
				for path, want := range wantFiles {
					got, err := os.ReadFile(path)
					if err != nil || string(got) != want {
						t.Fatalf("install %d: %s=%q,%v want %q", attempt+1, path, got, err, want)
					}
				}
				if previous == "" {
					if _, err := os.Stat(backup); !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("fresh install created a predecessor: %v", err)
					}
				}
			}
		})
	}
}

func TestInstallKeepsIdenticalExecutableInPlace(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source-aigw")
	target := filepath.Join(root, "aigw")
	for _, path := range []string{source, target} {
		if err := os.WriteFile(path, []byte("current"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	before, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := Install(source, target); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(target)
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("identical executable was replaced: %v", err)
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
	if err := Install(source, filepath.Join(blocked, "aigw")); err == nil {
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
	if err := os.Mkdir(RollbackPath(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := Install(source, target); err == nil || !strings.Contains(err.Error(), "save previous") {
		t.Fatalf("blocked backup = %v", err)
	}
	if err := os.RemoveAll(RollbackPath(target)); err != nil {
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
	if got := RollbackPath(filepath.Join("root", "aigw.exe")); filepath.Base(got) != ".aigw.previous.exe" {
		t.Fatalf("backup path = %q", got)
	}
}
