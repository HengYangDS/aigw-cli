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
