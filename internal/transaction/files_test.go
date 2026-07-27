package transaction_test

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

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
