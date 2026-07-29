//go:build !windows

package transaction_test

import (
	"bytes"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

func TestCaptureFileSnapshotSurfacesInspectRace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot-fifo")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		_, err := transaction.CaptureFileSnapshot(path)
		result <- err
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
	if err := <-result; err == nil || !strings.Contains(err.Error(), "inspect") {
		t.Fatalf("CaptureFileSnapshot() error = %v, want an inspect error after the path disappears", err)
	}
}

func TestWriteFileAtomicSurfacesWriteFailureBeyondFileSizeLimit(t *testing.T) {
	signal.Ignore(syscall.SIGXFSZ)
	var original syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &original); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		signal.Reset(syscall.SIGXFSZ)
		_ = syscall.Setrlimit(syscall.RLIMIT_FSIZE, &original)
	})
	tiny := syscall.Rlimit{Cur: 1, Max: original.Max}
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &tiny); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "file")
	data := bytes.Repeat([]byte("x"), 4096)
	if err := transaction.WriteFileAtomic(path, data, 0o600); err == nil || !strings.Contains(err.Error(), "write temporary file") {
		t.Fatalf("WriteFileAtomic() error = %v, want a write failure once the file-size rlimit is exceeded", err)
	}
}
