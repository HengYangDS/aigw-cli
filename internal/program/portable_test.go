package program

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"aigw-cli/internal/platform"
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
	refused, isDirectory := errors.AsType[*TargetDirectoryError](Install(source, root))
	if !isDirectory || refused.Path != root || !strings.Contains(refused.Error(), "is a directory; name the program file") {
		t.Fatalf("directory target = %#v", refused)
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
	original := writeFileAtomic
	writeFileAtomic = func(path string, data []byte, mode os.FileMode) error {
		if path == RollbackPath(target) {
			return errors.New("backup failed")
		}
		return original(path, data, mode)
	}
	t.Cleanup(func() { writeFileAtomic = original })
	if err := Install(source, target); err == nil || !strings.Contains(err.Error(), "save previous") {
		t.Fatalf("blocked backup = %v", err)
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
	if err := Uninstall(uninstallTarget); err == nil || !strings.Contains(err.Error(), "is a directory; name the program file") {
		t.Fatalf("directory target = %v", err)
	}
}

func TestPlansAndChangesRefuseARollbackDirectoryAlike(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "aigw")
	foreign := filepath.Join(RollbackPath(target), "foreign")
	for path, content := range map[string]string{source: "current", target: "previous", foreign: "preserve"} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, change := range map[string]func() error{
		"install plan":   func() error { _, err := PlanInstall(source, target); return err },
		"install":        func() error { return Install(source, target) },
		"uninstall plan": func() error { _, err := PlanUninstall(target); return err },
		"uninstall":      func() error { return Uninstall(target) },
	} {
		// The refusal names the rollback copy's file, never the private
		// directory that holds the program.
		if err := change(); err == nil || !strings.Contains(err.Error(), "rollback copy path "+filepath.Base(RollbackPath(target))+" beside the program is a directory") ||
			strings.Contains(err.Error(), filepath.Dir(target)) {
			t.Fatalf("%s = %v", name, err)
		}
	}
	for path, want := range map[string]string{target: "previous", foreign: "preserve"} {
		if got, err := os.ReadFile(path); err != nil || string(got) != want {
			t.Fatalf("%s = %q, %v; want %q", path, got, err, want)
		}
	}
	// A fresh install keeps no rollback copy, so the directory is no obstacle.
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := Install(source, target); err != nil {
		t.Fatal(err)
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

// A target whose path runs through a file is refused by planning, on every
// platform, so a preview does not pass where the write would fail. The error
// names the file that stands where a folder is needed.
func TestPlanInstallRefusesAPathThroughAFile(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "aigw")
	blocked := filepath.Join(root, "blocked")
	for _, path := range []string{source, blocked} {
		if err := os.WriteFile(path, []byte("file"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, target := range []string{filepath.Join(blocked, "aigw"), filepath.Join(blocked, "nested", "aigw")} {
		_, err := PlanInstall(source, target)
		failure, classified := errors.AsType[*platform.PathConditionError](err)
		if !classified || failure.Culprit != blocked || !errors.Is(err, syscall.ENOTDIR) {
			t.Fatalf("%s = %v", target, err)
		}
	}
	if _, err := PlanInstall(source, filepath.Join(root, "missing", "aigw")); err != nil {
		t.Fatalf("a target under a folder that does not exist yet = %v", err)
	}
}

// A program file that runs through a regular file is refused before anything
// is written or removed, on Windows too, where such a path reads as a missing
// one and an uninstall would report nothing to remove.
func TestPlanInstallRefusesAProgramThroughAFile(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "aigw")
	if err := os.WriteFile(source, []byte("AIGW fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := PlanInstall(source, filepath.Join(file, "aigw")); err == nil || !errors.Is(err, platform.ErrFileAsFolder) {
		t.Fatalf("an install through a file = %v", err)
	}
	_, err := PlanUninstall(filepath.Join(file, "aigw"))
	if err == nil || !errors.Is(err, syscall.ENOTDIR) {
		t.Fatalf("an uninstall through a file = %v", err)
	}
}

// Windows reports a program file under a regular file as a missing path, so a
// plan that took it for absent would remove nothing and say nothing. Both
// paths are classified from the file system instead, so the plan refuses here
// as on POSIX. The error mapping is Windows', given to this host.
func TestPresentProgramFilesRefusesAProgramUnderAFileAsWindowsReportsIt(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	original := lstatProgramFile
	t.Cleanup(func() { lstatProgramFile = original })
	lstatProgramFile = func(path string) (os.FileInfo, error) {
		return nil, &os.PathError{Op: "lstat", Path: path, Err: fs.ErrNotExist}
	}
	present, err := presentProgramFiles(filepath.Join(file, "aigw"))
	if err == nil || !errors.Is(err, platform.ErrFileAsFolder) || !errors.Is(err, syscall.ENOTDIR) {
		t.Fatalf("a program under a file = %v, %v", present, err)
	}
}
