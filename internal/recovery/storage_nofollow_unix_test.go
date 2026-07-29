//go:build darwin || linux

package recovery

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

func TestReadRecoveryDirectoryNoFollowMaterializesEntryMetadata(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "inventory")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	original := []byte("original")
	if err := os.WriteFile(filepath.Join(directory, "config.toml"), original, 0o600); err != nil {
		t.Fatal(err)
	}
	entries, _, err := readRecoveryDirectoryNoFollow(root, directory)
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries = %#v, err = %v", entries, err)
	}

	moved := filepath.Join(root, "inventory-original")
	if err := os.Rename(directory, moved); err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(root, "inventory-replacement")
	if err := os.Mkdir(replacement, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(replacement, "config.toml"), bytes.Repeat([]byte("x"), 31), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(replacement, directory); err != nil {
		t.Fatal(err)
	}

	info := entries[0]
	if info.Size() != int64(len(original)) || info.Mode().Perm() != 0o600 {
		t.Fatalf("entry metadata followed replacement tree: size=%d mode=%o", info.Size(), info.Mode().Perm())
	}
}

func TestReadRecoveryDirectoryNoFollowRejectsUnreadableDirectory(t *testing.T) {
	root := t.TempDir()
	unreadable := filepath.Join(root, "unreadable")
	if err := os.Mkdir(unreadable, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unreadable, "config.toml"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(unreadable, 0o100); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o700) })
	if _, _, err := readRecoveryDirectoryNoFollow(root, unreadable); err == nil {
		t.Fatal("listed the entries of a directory without read permission")
	}
}

func TestCaptureRecoveryFileNoFollowRejectsFIFOWithoutBlocking(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "ledger.json")
	if err := unix.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := captureRecoveryFileNoFollow(root, path)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("FIFO was accepted as a regular recovery file")
		}
	case <-time.After(250 * time.Millisecond):
		fd, _ := unix.Open(path, unix.O_WRONLY|unix.O_NONBLOCK, 0)
		if fd >= 0 {
			_ = unix.Close(fd)
		}
		select {
		case <-done:
		case <-time.After(time.Second):
		}
		t.Fatal("recovery capture blocked while opening a FIFO")
	}
}

func TestRecoveryOwnedMutationsRejectSymlinkedParent(t *testing.T) {
	for _, operation := range []string{"write", "remove", "restore"} {
		t.Run(operation, func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, "recovery")
			outside := filepath.Join(base, "outside")
			if err := os.Mkdir(root, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(outside, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, filepath.Join(root, "air")); err != nil {
				t.Fatal(err)
			}
			store := NewStore(root)
			path := filepath.Join(root, "air", "ledger.json")
			outsidePath := filepath.Join(outside, "ledger.json")
			before := []byte("before\n")
			after := []byte("after\n")

			switch operation {
			case "write":
				if _, err := store.write(path, transaction.FileSnapshot{}, after, 0o600); err == nil {
					t.Fatal("recovery write followed a symlinked parent")
				}
				if _, err := os.Lstat(outsidePath); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("recovery write escaped its root: %v", err)
				}
			case "remove":
				if err := os.WriteFile(outsidePath, after, 0o600); err != nil {
					t.Fatal(err)
				}
				if _, err := store.remove(path, desiredSnapshot(after, 0o600)); err == nil {
					t.Fatal("recovery remove followed a symlinked parent")
				}
				got, err := os.ReadFile(outsidePath)
				if err != nil || !bytes.Equal(got, after) {
					t.Fatalf("recovery remove escaped its root: %q, %v", got, err)
				}
			case "restore":
				if err := os.WriteFile(outsidePath, after, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := store.restore(path, desiredSnapshot(before, 0o600), desiredSnapshot(after, 0o600)); err == nil {
					t.Fatal("recovery restore followed a symlinked parent")
				}
				got, err := os.ReadFile(outsidePath)
				if err != nil || !bytes.Equal(got, after) {
					t.Fatalf("recovery restore escaped its root: %q, %v", got, err)
				}
			}
		})
	}
}

func TestRecoveryOwnedMutationRejectsSymlinkInConfiguredRoot(t *testing.T) {
	base := t.TempDir()
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(filepath.Join(outside, "recovery"), 0o700); err != nil {
		t.Fatal(err)
	}
	linkedParent := filepath.Join(base, "linked-parent")
	if err := os.Symlink(outside, linkedParent); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(linkedParent, "recovery")
	store := NewStore(root)
	path := filepath.Join(root, "air", "ledger.json")
	if _, err := store.write(path, transaction.FileSnapshot{}, []byte("outside\n"), 0o600); err == nil {
		t.Fatal("recovery write followed a symlink in the configured root")
	}
	outsidePath := filepath.Join(outside, "recovery", "air", "ledger.json")
	if _, err := os.Lstat(outsidePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("recovery write escaped through configured-root symlink: %v", err)
	}
}

func TestCaptureRecoveryFileNoFollowRejectsSymlinkInConfiguredRoot(t *testing.T) {
	base := t.TempDir()
	outsideRoot := filepath.Join(base, "outside", "recovery")
	if err := os.MkdirAll(outsideRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outsideRoot, "ledger.json"), []byte("outside\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	linkedParent := filepath.Join(base, "linked-parent")
	if err := os.Symlink(filepath.Join(base, "outside"), linkedParent); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(linkedParent, "recovery")
	if _, err := captureRecoveryFileNoFollow(root, filepath.Join(root, "ledger.json")); err == nil {
		t.Fatal("recovery capture followed a symlink in the configured root")
	}
}

func TestInspectAirLifecycleRejectsDanglingSymlinkInConfiguredRoot(t *testing.T) {
	base := t.TempDir()
	linkedParent := filepath.Join(base, "linked-parent")
	if err := os.Symlink(filepath.Join(base, "missing-target"), linkedParent); err != nil {
		t.Fatal(err)
	}
	f := newAirRecoveryFixture(t)
	f.store = NewStore(filepath.Join(linkedParent, "recovery"))
	status, err := f.store.InspectAirLifecycle(f.air, f.standalone)
	if err != nil {
		t.Fatal(err)
	}
	if status.RecoveryState != "none" || status.RecoveryHealth != AirRecoveryHealthInvalid || status.RecoveryReasonCode != AirRecoveryReasonStoragePermission {
		t.Fatalf("status = %#v, want none invalid storage-permission-invalid", status)
	}
}
