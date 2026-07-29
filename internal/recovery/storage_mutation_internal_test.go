//go:build darwin || linux

package recovery

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

func privateRecoveryRootForTest(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "recovery")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRecoveryMutationsEnforceMissingAndChangedSnapshots(t *testing.T) {
	root := filepath.Join(t.TempDir(), "recovery")
	path := filepath.Join(root, "air", "ledger.json")
	missing := transaction.FileSnapshot{}
	desired := desiredSnapshot([]byte("ledger\n"), 0o600)

	if got, err := removeRecoveryFileIfUnchanged(root, path, missing); err != nil || got.Exists {
		t.Fatalf("remove missing file = %#v, %v", got, err)
	}
	if _, err := removeRecoveryFileIfUnchanged(root, path, desired); err == nil {
		t.Fatal("remove accepted a missing file as the expected postimage")
	}
	if err := restoreRecoveryFileAtomicIfPostimage(root, path, missing, missing); err != nil {
		t.Fatalf("restore missing to missing: %v", err)
	}
	if err := restoreRecoveryFileAtomicIfPostimage(root, path, missing, desired); err == nil {
		t.Fatal("restore accepted a missing file as a present postimage")
	}

	written, err := writeRecoveryFileAtomicIfUnchanged(root, path, missing, desired.Data, desired.Mode)
	if err != nil || !sameRecoverySnapshot(written, desired) {
		t.Fatalf("initial write = %#v, %v", written, err)
	}
	if _, err := writeRecoveryFileAtomicIfUnchanged(root, path, missing, []byte("new\n"), 0o600); err == nil {
		t.Fatal("write accepted a changed preimage")
	}
	if _, err := removeRecoveryFileIfUnchanged(root, path, missing); err == nil {
		t.Fatal("remove accepted a changed preimage")
	}
	if err := restoreRecoveryFileAtomicIfPostimage(root, path, missing, desiredSnapshot([]byte("other\n"), 0o600)); err == nil {
		t.Fatal("restore accepted a changed postimage")
	}
}

func TestRecoveryMutationsRejectNonRegularTargets(t *testing.T) {
	for _, operation := range []string{"write", "remove", "restore"} {
		t.Run(operation, func(t *testing.T) {
			root := privateRecoveryRootForTest(t)
			path := filepath.Join(root, "air", "ledger.json")
			if err := os.MkdirAll(path, 0o700); err != nil {
				t.Fatal(err)
			}
			var err error
			switch operation {
			case "write":
				_, err = writeRecoveryFileAtomicIfUnchanged(root, path, transaction.FileSnapshot{}, []byte("data"), 0o600)
			case "remove":
				_, err = removeRecoveryFileIfUnchanged(root, path, transaction.FileSnapshot{})
			case "restore":
				err = restoreRecoveryFileAtomicIfPostimage(root, path, transaction.FileSnapshot{}, transaction.FileSnapshot{})
			}
			if err == nil {
				t.Fatalf("%s accepted a directory target", operation)
			}
		})
	}
}

func TestRecoveryPathOpeningRejectsEscapesAndUnsafeDirectories(t *testing.T) {
	t.Run("path escape", func(t *testing.T) {
		root := t.TempDir()
		for _, path := range []string{root, filepath.Join(root, "..", "outside")} {
			if file, _, err := openRecoveryParentNoFollow(root, path, true); err == nil {
				_ = file.Close()
				t.Fatalf("opened escaping path %q", path)
			}
		}
		if file, err := openRecoveryPathNoFollow(root, filepath.Join(root, "..", "outside"), false); err == nil {
			_ = file.Close()
			t.Fatal("read path escaped its root")
		}
		if file, err := openRecoveryPathNoFollow(root, root, false); err == nil {
			_ = file.Close()
			t.Fatal("recovery root was accepted as a regular file")
		}
	})

	t.Run("unsafe root", func(t *testing.T) {
		root := t.TempDir()
		if err := os.Chmod(root, 0o755); err != nil {
			t.Fatal(err)
		}
		if file, err := openRecoveryRootNoFollow(root, false, true); err == nil {
			_ = file.Close()
			t.Fatal("opened a recovery root with unsafe permissions")
		}
	})

	t.Run("unsafe parent", func(t *testing.T) {
		root := privateRecoveryRootForTest(t)
		parent := filepath.Join(root, "air")
		if err := os.Mkdir(parent, 0o755); err != nil {
			t.Fatal(err)
		}
		if file, _, err := openRecoveryParentNoFollow(root, filepath.Join(parent, "ledger.json"), false); err == nil {
			_ = file.Close()
			t.Fatal("opened a recovery parent with unsafe permissions")
		}
	})

	t.Run("missing parent", func(t *testing.T) {
		root := privateRecoveryRootForTest(t)
		if file, _, err := openRecoveryParentNoFollow(root, filepath.Join(root, "missing", "ledger.json"), false); !errors.Is(err, os.ErrNotExist) {
			if file != nil {
				_ = file.Close()
			}
			t.Fatalf("missing parent error = %v", err)
		}
	})

	t.Run("regular parent component", func(t *testing.T) {
		root := privateRecoveryRootForTest(t)
		blocked := filepath.Join(root, "blocked")
		if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
			t.Fatal(err)
		}
		if file, _, err := openRecoveryParentNoFollow(root, filepath.Join(blocked, "ledger.json"), false); err == nil {
			_ = file.Close()
			t.Fatal("opened a regular file as a recovery parent")
		}
	})
}

func TestRecoveryDescriptorOperationsRejectClosedDirectory(t *testing.T) {
	directory, err := os.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := directory.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := createRecoveryTempAt(directory); err == nil {
		t.Fatal("created a temporary file through a closed directory")
	}
	if _, err := captureRecoveryFileAt(directory, "ledger.json"); err == nil {
		t.Fatal("captured a file through a closed directory")
	}
	if err := writeRecoveryFileAt(directory, "ledger.json", []byte("data"), 0o600); err == nil {
		t.Fatal("wrote a file through a closed directory")
	}
}

func TestReadRecoveryDirectoryRejectsRegularFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "ledger.json")
	if err := os.WriteFile(path, []byte("ledger"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readRecoveryDirectoryNoFollow(root, path); err == nil {
		t.Fatal("read a regular file as a recovery directory")
	}
	if _, err := captureRecoveryFileNoFollow(root, filepath.Join(root, "missing")); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("capture missing file: %v", err)
	}
}

func TestCaptureRecoveryFileAtRejectsSymlink(t *testing.T) {
	root := privateRecoveryRootForTest(t)
	target := filepath.Join(root, "target")
	if err := os.WriteFile(target, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	directory, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := directory.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err := captureRecoveryFileAt(directory, "link"); err == nil {
		t.Fatal("captured a recovery file through a symlink")
	}
}

func TestOpenRecoveryRootRejectsNonDirectoryComponent(t *testing.T) {
	base := t.TempDir()
	component := filepath.Join(base, "component")
	if err := os.WriteFile(component, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if file, err := openRecoveryRootNoFollow(filepath.Join(component, "recovery"), false, false); err == nil {
		_ = file.Close()
		t.Fatal("opened a recovery root through a regular file")
	}
	if got := canonicalRecoveryRootPath("/Users/example/recovery"); got != "/Users/example/recovery" {
		t.Fatalf("canonical path = %q", got)
	}
}

func TestOpenRecoveryRootHandlesFilesystemRoot(t *testing.T) {
	root, err := openRecoveryRootNoFollow(string(os.PathSeparator), false, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenRecoveryRootRejectsRelativePathFromRemovedWorkingDirectory(t *testing.T) {
	originalWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	workingDirectory := filepath.Join(t.TempDir(), "working")
	if err := os.Mkdir(workingDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workingDirectory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalWorkingDirectory) })
	if err := os.Remove(workingDirectory); err != nil {
		_ = os.Chdir(originalWorkingDirectory)
		t.Fatal(err)
	}
	root, openErr := openRecoveryRootNoFollow("relative-recovery", false, false)
	if root != nil {
		_ = root.Close()
	}
	if err := os.Chdir(originalWorkingDirectory); err != nil {
		t.Fatal(err)
	}
	if openErr == nil {
		t.Fatal("resolved a relative recovery root from a removed working directory")
	}
}
