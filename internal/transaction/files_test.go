package transaction_test

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"aigw-cli/internal/transaction"
)

func TestCaptureFileSnapshotOfMissingFileIsEmptyNotError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent")
	snapshot, err := transaction.CaptureFileSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Exists || len(snapshot.Data) != 0 || snapshot.SHA256 != "" {
		t.Fatalf("snapshot of missing file = %#v", snapshot)
	}
}

func TestCaptureFileSnapshotSurfacesReadErrors(t *testing.T) {
	if runtime.GOOS != "windows" {
		dir := t.TempDir()
		if _, err := transaction.CaptureFileSnapshot(dir); err == nil || strings.Contains(err.Error(), "not exist") {
			t.Fatalf("CaptureFileSnapshot(directory) error = %v", err)
		}
	}
}

func TestWriteFileAtomicPreservesMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := transaction.WriteFileAtomic(path, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	info, _ := os.Stat(path)
	if string(data) != "new" {
		t.Fatalf("content=%q", data)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o640 {
		t.Fatalf("mode=%o", info.Mode().Perm())
	}
}

func TestWriteFileAtomicIfUnchangedRejectsChangedPreimage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("before"), 0o640); err != nil {
		t.Fatal(err)
	}
	expected, err := transaction.CaptureFileSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("newer"), 0o640); err != nil {
		t.Fatal(err)
	}
	_, err = transaction.WriteFileAtomicIfUnchanged(path, expected, []byte("desired"), 0o600)
	if err == nil || !strings.Contains(err.Error(), "preimage changed") {
		t.Fatalf("WriteFileAtomicIfUnchanged() error = %v, want preimage changed", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil || string(got) != "newer" {
		t.Fatalf("file after rejected write = %q, %v; want newer", got, readErr)
	}
}

func TestRestoreFileAtomicIfPostimageRestoresExactPreimage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("before\n\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	before, err := transaction.CaptureFileSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	postimage, err := transaction.WriteFileAtomicIfUnchanged(path, before, []byte("projected"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(path, 0o755); err != nil {
			t.Fatal(err)
		}
		postimage, err = transaction.CaptureFileSnapshot(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := transaction.RestoreFileAtomicIfPostimage(path, before, postimage); err != nil {
		t.Fatal(err)
	}
	restored, err := transaction.CaptureFileSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored, before) {
		t.Fatalf("restored snapshot = %#v, want %#v", restored, before)
	}
}

func TestRestoreFileAtomicIfPostimageDoesNotOverwriteNewerContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("before"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := transaction.CaptureFileSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	postimage, err := transaction.WriteFileAtomicIfUnchanged(path, before, []byte("projected"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("newer"), 0o600); err != nil {
		t.Fatal(err)
	}
	err = transaction.RestoreFileAtomicIfPostimage(path, before, postimage)
	if err == nil || !strings.Contains(err.Error(), "postimage changed") {
		t.Fatalf("RestoreFileAtomicIfPostimage() error = %v, want postimage changed", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil || string(got) != "newer" {
		t.Fatalf("file after rejected restore = %q, %v; want newer", got, readErr)
	}
}

func TestRemoveFileIfUnchangedRemovesOnlyPreparedContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("prepared"), 0o600); err != nil {
		t.Fatal(err)
	}
	expected, err := transaction.CaptureFileSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	removed, err := transaction.RemoveFileIfUnchanged(path, expected)
	if err != nil {
		t.Fatal(err)
	}
	if removed.Exists {
		t.Fatalf("removed snapshot = %#v, want absent", removed)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file remains after removal: %v", err)
	}
	if err := os.WriteFile(path, []byte("newer"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = transaction.RemoveFileIfUnchanged(path, expected)
	if err == nil || !strings.Contains(err.Error(), "preimage changed") {
		t.Fatalf("RemoveFileIfUnchanged() error = %v, want preimage changed", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil || string(got) != "newer" {
		t.Fatalf("file after rejected removal = %q, %v; want newer", got, readErr)
	}
}

func TestRestoreFileAtomicIfPostimageRemovesFileCreatedByTransaction(t *testing.T) {
	path := filepath.Join(t.TempDir(), "created-by-transaction")
	before, err := transaction.CaptureFileSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if before.Exists {
		t.Fatalf("preimage of never-created file must be absent: %#v", before)
	}
	postimage, err := transaction.WriteFileAtomicIfUnchanged(path, before, []byte("projected"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := transaction.RestoreFileAtomicIfPostimage(path, before, postimage); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("rollback of a creation must remove the file: %v", err)
	}
}

// TestWriteFileAtomicExactModeIfUnchangedCorrectsADriftedMode pins the one
// difference from the ordinary guarded write: the mode is written rather than
// inherited, which is what lets an artifact treat its permissions as a contract.
func TestWriteFileAtomicExactModeIfUnchangedCorrectsADriftedMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not represent owner-only permissions in a file mode")
	}
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	expected, err := transaction.CaptureFileSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	post, err := transaction.WriteFileAtomicExactModeIfUnchanged(path, expected, []byte("desired"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if post.Mode.Perm() != 0o600 || string(post.Data) != "desired" {
		t.Fatalf("postimage = %#v", post)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode on disk = %v, want 0600", info.Mode().Perm())
	}
	// The preimage guard is not weakened by writing the mode exactly.
	if err := os.WriteFile(path, []byte("newer"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := transaction.WriteFileAtomicExactModeIfUnchanged(path, expected, []byte("x"), 0o600); err == nil ||
		!strings.Contains(err.Error(), "preimage changed") {
		t.Fatalf("WriteFileAtomicExactModeIfUnchanged() error = %v, want preimage changed", err)
	}
}

func TestWriteFileAtomicIfUnchangedRejectsUnreadableCurrentState(t *testing.T) {
	if runtime.GOOS == "windows" {
		return
	}
	path := filepath.Join(t.TempDir(), "dir-as-file")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := transaction.WriteFileAtomicIfUnchanged(path, transaction.FileSnapshot{}, []byte("x"), 0o600)
	if err == nil || strings.Contains(err.Error(), "preimage changed") {
		t.Fatalf("WriteFileAtomicIfUnchanged() error = %v, want a snapshot read failure", err)
	}
}

func TestRemoveFileIfUnchangedRejectsUnreadableCurrentState(t *testing.T) {
	if runtime.GOOS == "windows" {
		return
	}
	path := filepath.Join(t.TempDir(), "dir-as-file")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := transaction.RemoveFileIfUnchanged(path, transaction.FileSnapshot{})
	if err == nil || strings.Contains(err.Error(), "preimage changed") {
		t.Fatalf("RemoveFileIfUnchanged() error = %v, want a snapshot read failure", err)
	}
}

func TestRestoreFileAtomicIfPostimageRejectsUnreadableCurrentState(t *testing.T) {
	if runtime.GOOS == "windows" {
		return
	}
	path := filepath.Join(t.TempDir(), "dir-as-file")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	err := transaction.RestoreFileAtomicIfPostimage(path, transaction.FileSnapshot{}, transaction.FileSnapshot{})
	if err == nil || strings.Contains(err.Error(), "postimage changed") {
		t.Fatalf("RestoreFileAtomicIfPostimage() error = %v, want a snapshot read failure", err)
	}
}

func TestWriteFileAtomicSurfacesBlockedParentPath(t *testing.T) {
	base := t.TempDir()
	blocked := filepath.Join(base, "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(blocked, "child", "file")
	if err := transaction.WriteFileAtomicExactMode(path, []byte("x"), 0o600); err == nil {
		t.Fatal("WriteFileAtomicExactMode succeeded despite a file blocking its parent path")
	}
}

func TestWriteFileAtomicSurfacesStatFailureBeyondMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid\x00name")
	if err := transaction.WriteFileAtomic(path, []byte("x"), 0o600); err == nil {
		t.Fatal("WriteFileAtomic succeeded despite an invalid path")
	}
}

func TestWriteFileAtomicSurfacesRenameFailureOverExistingDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "target-is-a-directory")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := transaction.WriteFileAtomic(path, []byte("x"), 0o600); err == nil || !strings.Contains(err.Error(), "replace ") {
		t.Fatalf("WriteFileAtomic() error = %v, want a rename failure over an existing directory", err)
	}
}
