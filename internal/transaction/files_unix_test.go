//go:build !windows

package transaction_test

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"aigw-cli/internal/transaction"
)

func TestCaptureFileSnapshotSurfacesSymlinkLoop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "loop")
	if err := os.Symlink(path, path); err != nil {
		t.Fatal(err)
	}
	if _, err := transaction.CaptureFileSnapshot(path); err == nil || !strings.Contains(err.Error(), "read") {
		t.Fatalf("CaptureFileSnapshot() error = %v, want a symlink-loop read error", err)
	}
}

func TestCaptureFileSnapshotRemainsConsistentWhenPathDisappears(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot-fifo")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	type snapshotResult struct {
		snapshot transaction.FileSnapshot
		err      error
	}
	result := make(chan snapshotResult, 1)
	go func() {
		snapshot, err := transaction.CaptureFileSnapshot(path)
		result <- snapshotResult{snapshot: snapshot, err: err}
	}()

	writer, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = writer.Close() }()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("snapshot")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	got := <-result
	if got.err != nil || !got.snapshot.Exists || string(got.snapshot.Data) != "snapshot" {
		t.Fatalf("CaptureFileSnapshot() = %#v, %v", got.snapshot, got.err)
	}
}
