package transaction

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

type failingSnapshotFile struct {
	readErr error
	statErr error
	closed  bool
}

type failingTemporaryFile struct {
	name     string
	writeErr error
	chmodErr error
	syncErr  error
	closeErr error
}

func (file *failingTemporaryFile) Name() string { return file.name }

func (file *failingTemporaryFile) Write(value []byte) (int, error) {
	if file.writeErr != nil {
		return 0, file.writeErr
	}
	return len(value), nil
}

func (file *failingTemporaryFile) Chmod(os.FileMode) error { return file.chmodErr }

func (file *failingTemporaryFile) Sync() error { return file.syncErr }

func (file *failingTemporaryFile) Close() error { return file.closeErr }

func (file *failingSnapshotFile) Read([]byte) (int, error) {
	if file.readErr != nil {
		return 0, file.readErr
	}
	return 0, io.EOF
}

func (file *failingSnapshotFile) Stat() (os.FileInfo, error) {
	return nil, file.statErr
}

func (file *failingSnapshotFile) Close() error {
	file.closed = true
	return nil
}

func TestCaptureOpenedFileSurfacesReadAndStatErrors(t *testing.T) {
	tests := []struct {
		name    string
		file    *failingSnapshotFile
		message string
	}{
		{name: "read", file: &failingSnapshotFile{readErr: errors.New("read failure")}, message: "read fixture"},
		{name: "stat", file: &failingSnapshotFile{statErr: errors.New("stat failure")}, message: "inspect fixture"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := captureOpenedFile("fixture", test.file); err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("captureOpenedFile() error = %v, want %q", err, test.message)
			}
			if !test.file.closed {
				t.Fatal("captureOpenedFile did not close its file")
			}
		})
	}
}

func TestCommitTemporaryFileSurfacesDeterministicStageErrors(t *testing.T) {
	tests := []struct {
		name    string
		file    *failingTemporaryFile
		message string
	}{
		{name: "write", file: &failingTemporaryFile{writeErr: errors.New("write failure")}, message: "write temporary file"},
		{name: "chmod", file: &failingTemporaryFile{chmodErr: errors.New("chmod failure")}, message: "set temporary mode"},
		{name: "sync", file: &failingTemporaryFile{syncErr: errors.New("sync failure")}, message: "sync temporary file"},
		{name: "close", file: &failingTemporaryFile{closeErr: errors.New("close failure")}, message: "close temporary file"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := commitTemporaryFile(test.file, "target", []byte("data"), 0o600); err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("commitTemporaryFile() error = %v, want %q", err, test.message)
			}
		})
	}
}

func TestGuardedRemovalSurfacesDeterministicRemoveErrors(t *testing.T) {
	path := t.TempDir() + "/file"
	if err := os.WriteFile(path, []byte("prepared"), 0o600); err != nil {
		t.Fatal(err)
	}
	prepared, err := CaptureFileSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	remove := func(string) error { return errors.New("remove failure") }
	if _, err := removeFileIfUnchanged(path, prepared, remove); err == nil || !strings.Contains(err.Error(), "remove failure") {
		t.Fatalf("removeFileIfUnchanged() error = %v, want remove failure", err)
	}

	absent := FileSnapshot{}
	if err := restoreFileAtomicIfPostimage(path, absent, prepared, remove); err == nil || !strings.Contains(err.Error(), "remove failure") {
		t.Fatalf("restoreFileAtomicIfPostimage() error = %v, want remove failure", err)
	}
}

func TestGuardedRemovalAcceptsAlreadyAbsentState(t *testing.T) {
	path := t.TempDir() + "/absent"
	removeCalled := false
	remove := func(string) error {
		removeCalled = true
		return nil
	}
	if _, err := removeFileIfUnchanged(path, FileSnapshot{}, remove); err != nil {
		t.Fatal(err)
	}
	if removeCalled {
		t.Fatal("removeFileIfUnchanged called remove for an already absent file")
	}
	if err := restoreFileAtomicIfPostimage(path, FileSnapshot{}, FileSnapshot{}, remove); err != nil {
		t.Fatal(err)
	}
	if removeCalled {
		t.Fatal("restoreFileAtomicIfPostimage called remove for an already absent file")
	}
}

func TestGuardedWriteSurfacesDeterministicWriteError(t *testing.T) {
	path := t.TempDir() + "/file"
	expected, err := CaptureFileSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	write := func(string, []byte, os.FileMode) error { return errors.New("write failure") }
	if _, err := writeFileAtomicIfUnchanged(path, expected, []byte("desired"), 0o600, write); err == nil || !strings.Contains(err.Error(), "write failure") {
		t.Fatalf("writeFileAtomicIfUnchanged() error = %v, want write failure", err)
	}
}
