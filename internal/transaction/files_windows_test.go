//go:build windows

package transaction_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"

	"aigw-cli/internal/transaction"
)

// These are the Windows equivalents of files_unix_test.go. Native path and
// sharing semantics exercise the platform-specific production branches.

// lockExclusive opens path with no sharing at all, so any concurrent
// os.Open (which Go always issues with at least FILE_SHARE_READ|WRITE) fails
// with a genuine sharing violation until the returned func closes the lock.
func lockExclusive(t *testing.T, path string) func() {
	t.Helper()
	namePtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(namePtr, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatalf("CreateFile with an exclusive share mode: %v", err)
	}
	return func() { _ = windows.CloseHandle(handle) }
}

// lockDeletion keeps the file readable while withholding delete sharing. This
// lets the guarded operation observe the expected snapshot before Windows
// rejects the actual removal with a sharing violation.
func lockDeletion(t *testing.T, path string) func() {
	t.Helper()
	namePtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(
		namePtr,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		t.Fatalf("CreateFile without delete sharing: %v", err)
	}
	return func() { _ = windows.CloseHandle(handle) }
}

func TestCaptureFileSnapshotSurfacesReadErrorsWhenExclusivelyLocked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "locked")
	if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	unlock := lockExclusive(t, path)
	defer unlock()
	if _, err := transaction.CaptureFileSnapshot(path); err == nil || strings.Contains(err.Error(), "not exist") {
		t.Fatalf("CaptureFileSnapshot() error = %v, want a read error while the file is exclusively locked", err)
	}
}

func TestWriteFileAtomicIfUnchangedRejectsUnreadableCurrentStateWhenLocked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	unlock := lockExclusive(t, path)
	defer unlock()
	_, err := transaction.WriteFileAtomicIfUnchanged(path, transaction.FileSnapshot{}, []byte("x"), 0o600)
	if err == nil || strings.Contains(err.Error(), "preimage changed") {
		t.Fatalf("WriteFileAtomicIfUnchanged() error = %v, want the underlying read failure", err)
	}
}

func TestRemoveFileIfUnchangedRejectsUnreadableCurrentStateWhenLocked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	unlock := lockExclusive(t, path)
	defer unlock()
	_, err := transaction.RemoveFileIfUnchanged(path, transaction.FileSnapshot{})
	if err == nil || strings.Contains(err.Error(), "preimage changed") {
		t.Fatalf("RemoveFileIfUnchanged() error = %v, want the underlying read failure", err)
	}
}

func TestRemoveFileIfUnchangedSurfacesRemoveFailureWhenDeletionIsLocked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("prepared"), 0o600); err != nil {
		t.Fatal(err)
	}
	expected, err := transaction.CaptureFileSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	unlock := lockDeletion(t, path)
	defer unlock()
	if _, err := transaction.RemoveFileIfUnchanged(path, expected); err == nil || !strings.Contains(err.Error(), "remove ") {
		t.Fatalf("RemoveFileIfUnchanged() error = %v, want the removal failure", err)
	}
}

func TestRemoveFileIfUnchangedAcceptsAlreadyAbsentFileOnWindows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent")
	absent, err := transaction.CaptureFileSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := transaction.RemoveFileIfUnchanged(path, absent); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveFileIfUnchangedRemovesPreparedFileOnWindows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prepared")
	if err := os.WriteFile(path, []byte("prepared"), 0o600); err != nil {
		t.Fatal(err)
	}
	expected, err := transaction.CaptureFileSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	removed, err := transaction.RemoveFileIfUnchanged(path, expected)
	if err != nil || removed.Exists {
		t.Fatalf("RemoveFileIfUnchanged() = %#v, %v", removed, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("prepared file remains: %v", err)
	}
}

func TestRestoreFileAtomicIfPostimageRejectsUnreadableCurrentStateWhenLocked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	unlock := lockExclusive(t, path)
	defer unlock()
	err := transaction.RestoreFileAtomicIfPostimage(path, transaction.FileSnapshot{}, transaction.FileSnapshot{})
	if err == nil || strings.Contains(err.Error(), "postimage changed") {
		t.Fatalf("RestoreFileAtomicIfPostimage() error = %v, want the underlying read failure", err)
	}
}

func TestRestoreFileAtomicIfPostimageSurfacesRemoveFailureWhenDeletionIsLocked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	before, err := transaction.CaptureFileSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	postimage, err := transaction.WriteFileAtomicIfUnchanged(path, before, []byte("projected"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	unlock := lockDeletion(t, path)
	defer unlock()
	if err := transaction.RestoreFileAtomicIfPostimage(path, before, postimage); err == nil || !strings.Contains(err.Error(), "remove ") {
		t.Fatalf("RestoreFileAtomicIfPostimage() error = %v, want the removal failure", err)
	}
}

func TestRestoreFileAtomicIfPostimageAcceptsAlreadyAbsentFileOnWindows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent")
	absent, err := transaction.CaptureFileSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := transaction.RestoreFileAtomicIfPostimage(path, absent, absent); err != nil {
		t.Fatal(err)
	}
}

func TestRestoreFileAtomicIfPostimageRemovesCreatedFileOnWindows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "created")
	absent, err := transaction.CaptureFileSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	postimage, err := transaction.WriteFileAtomicIfUnchanged(path, absent, []byte("projected"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := transaction.RestoreFileAtomicIfPostimage(path, absent, postimage); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("created file remains after restore: %v", err)
	}
}

func TestWriteFileAtomicSurfacesInvalidWindowsPathDuringStat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid\x00name")
	if err := transaction.WriteFileAtomic(path, []byte("x"), 0o600); err == nil || !strings.Contains(err.Error(), "inspect ") {
		t.Fatalf("WriteFileAtomic() error = %v, want an invalid-path stat failure", err)
	}
}

func TestWriteFileAtomicSurfacesParentPathThatIsAFile(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "parent-is-a-file")
	if err := os.WriteFile(parent, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "child", "file")
	if err := transaction.WriteFileAtomicExactMode(path, []byte("x"), 0o600); err == nil || !strings.Contains(err.Error(), "create parent directory") {
		t.Fatalf("WriteFileAtomicExactMode() error = %v, want a parent-directory creation failure", err)
	}
}

func TestWriteFileAtomicExactModeAppliesReadOnlyModeAfterWriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "read-only")
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
	if err := transaction.WriteFileAtomicExactMode(path, []byte("content"), 0o400); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "content" || info.Mode().Perm()&0o200 != 0 {
		t.Fatalf("written file = %q mode %o, want content with a read-only mode", data, info.Mode().Perm())
	}
}
