//go:build !darwin && !linux

package recovery

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCaptureRecoveryFileNoFollowFallbackMaterializesSnapshot(t *testing.T) {
	root := privateRecoveryRootForTest(t)
	path := filepath.Join(root, "ledger.json")
	data := []byte("ledger contents")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := captureRecoveryFileNoFollow(root, path)
	if err != nil || !snapshot.Exists || string(snapshot.Data) != string(data) {
		t.Fatalf("snapshot = %#v, err = %v", snapshot, err)
	}
}

func TestCaptureRecoveryFileNoFollowFallbackReportsMissingFile(t *testing.T) {
	root := privateRecoveryRootForTest(t)
	snapshot, err := captureRecoveryFileNoFollow(root, filepath.Join(root, "missing.json"))
	if err != nil || snapshot.Exists {
		t.Fatalf("missing file snapshot = %#v, err = %v", snapshot, err)
	}
}

func TestCaptureRecoveryFileNoFollowFallbackRejectsDirectory(t *testing.T) {
	root := privateRecoveryRootForTest(t)
	path := filepath.Join(root, "ledger.json")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := captureRecoveryFileNoFollow(root, path); err == nil {
		t.Fatal("captured a directory as a regular recovery file")
	}
}

func TestCaptureRecoveryFileNoFollowFallbackRejectsPathEscape(t *testing.T) {
	root := privateRecoveryRootForTest(t)
	outside := filepath.Join(filepath.Dir(root), "outside.json")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := captureRecoveryFileNoFollow(root, filepath.Join(root, "..", "outside.json")); err == nil {
		t.Fatal("captured a file that escapes its root")
	}
}

func TestReadRecoveryDirectoryNoFollowFallbackListsEntries(t *testing.T) {
	root := privateRecoveryRootForTest(t)
	directory := filepath.Join(root, "inventory")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "config.toml"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, info, err := readRecoveryDirectoryNoFollow(root, directory)
	if err != nil || len(entries) != 1 || !info.IsDir() {
		t.Fatalf("entries = %#v, info = %v, err = %v", entries, info, err)
	}
}

func TestReadRecoveryDirectoryNoFollowFallbackRejectsRegularFile(t *testing.T) {
	root := privateRecoveryRootForTest(t)
	path := filepath.Join(root, "ledger.json")
	if err := os.WriteFile(path, []byte("ledger"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readRecoveryDirectoryNoFollow(root, path); err == nil {
		t.Fatal("read a regular file as a recovery directory")
	}
}

func TestReadRecoveryDirectoryNoFollowFallbackRejectsPathEscape(t *testing.T) {
	root := privateRecoveryRootForTest(t)
	outside := filepath.Join(filepath.Dir(root), "outside-directory")
	if err := os.Mkdir(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readRecoveryDirectoryNoFollow(root, filepath.Join(root, "..", "outside-directory")); err == nil {
		t.Fatal("read a directory that escapes its root")
	}
}

func TestLstatRecoveryPathRejectsEscapesAndAbsolutePaths(t *testing.T) {
	root := privateRecoveryRootForTest(t)
	for _, path := range []string{
		filepath.Join(root, "..", "outside.json"),
		filepath.Dir(root),
	} {
		if _, err := lstatRecoveryPath(root, path); err == nil {
			t.Fatalf("lstatRecoveryPath(%q) escaped its root", path)
		}
	}
	if _, err := lstatRecoveryPath(root, filepath.Join(root, "missing.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lstatRecoveryPath missing file = %v", err)
	}
}
