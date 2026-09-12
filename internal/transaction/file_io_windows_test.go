//go:build windows

package transaction

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommitTemporaryFileSurfacesWriteFailureFromReadOnlyHandle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "read-only-handle")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()

	err = commitTemporaryFile(file, filepath.Join(t.TempDir(), "target"), []byte("replacement"), 0o600)
	if err == nil || !strings.Contains(err.Error(), "write temporary file") {
		t.Fatalf("commitTemporaryFile() error = %v, want a real read-only-handle write failure", err)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil || string(data) != "original" {
		t.Fatalf("source after rejected write = %q, %v; want original", data, readErr)
	}
}

func TestCommitTemporaryFileSurfacesReplaceFailureForWindowsPipe(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	defer func() { _ = writer.Close() }()

	want := []byte("buffered")
	type readResult struct {
		data []byte
		err  error
	}
	result := make(chan readResult, 1)
	go func() {
		data := make([]byte, len(want))
		_, err := io.ReadFull(reader, data)
		result <- readResult{data: data, err: err}
	}()
	err = commitTemporaryFile(writer, filepath.Join(t.TempDir(), "target"), want, 0o400)
	_ = writer.Close()
	got := <-result
	if err == nil || !strings.Contains(err.Error(), "replace") {
		t.Fatalf("commitTemporaryFile() error = %v, want a real Windows pipe replace failure", err)
	}
	if got.err != nil || string(got.data) != string(want) {
		t.Fatalf("pipe data after metadata failure = %q, %v; want %q", got.data, got.err, want)
	}
}
