//go:build !windows

package transaction

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCaptureFileSnapshotSurfacesSymlinkLoop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "loop")
	if err := os.Symlink(path, path); err != nil {
		t.Fatal(err)
	}
	if _, err := CaptureFileSnapshot(path); err == nil || !strings.Contains(err.Error(), "read") {
		t.Fatalf("CaptureFileSnapshot() error = %v, want a symlink-loop read error", err)
	}
}

func TestCaptureFileSnapshotRemainsConsistentWhenPathDisappears(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot")
	if err := os.WriteFile(path, []byte("snapshot"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	got, err := captureOpenedFile(path, file)
	if err != nil || !got.Exists || string(got.Data) != "snapshot" {
		t.Fatalf("captureOpenedFile() = %#v, %v", got, err)
	}
}
