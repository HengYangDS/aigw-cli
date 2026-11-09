//go:build darwin

package platform

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

// lock sets macOS's uchg flag on path and clears it when the test ends, so
// that its temporary folder can be removed.
func lock(t *testing.T, path string) {
	t.Helper()
	if err := unix.Chflags(path, unix.UF_IMMUTABLE); err != nil {
		t.Skipf("the uchg flag cannot be set here: %v", err)
	}
	t.Cleanup(func() { _ = unix.Chflags(path, 0) })
}

// A write that a flag refuses names every entry that carries one: the file,
// read through a link to the file that it leads to, and the folder that holds
// it, whose lock refuses the rename that replaces the file. A write that no
// flag refuses, and any read, carries none.
func TestAWriteNamesTheEntriesThatAFlagLocks(t *testing.T) {
	root := t.TempDir()
	folder := filepath.Join(root, "work")
	if err := os.Mkdir(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(folder, "config.toml")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	denied := &os.PathError{Op: "open", Path: file, Err: syscall.EPERM}
	locks := func(path string) []LockedEntry {
		return ClassifyPath(path, FileExpected, &os.PathError{Op: "open", Path: path, Err: syscall.EPERM}).Written().Locked
	}
	if got := locks(file); len(got) != 0 {
		t.Fatalf("a write that no flag refused names %v", got)
	}
	if read := ClassifyPath(file, FileExpected, denied); len(read.Locked) != 0 {
		t.Fatalf("a read names %v", read.Locked)
	}
	lock(t, file)
	if got := locks(file); len(got) != 1 || got[0] != (LockedEntry{Path: file}) {
		t.Fatalf("a locked file = %v", got)
	}
	lock(t, folder)
	if got := locks(file); len(got) != 2 || got[0] != (LockedEntry{Path: file}) || got[1] != (LockedEntry{Path: folder}) {
		t.Fatalf("a locked file and folder = %v", got)
	}
	_ = unix.Chflags(folder, 0)
	link := filepath.Join(root, "link.toml")
	if err := os.Symlink(file, link); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}
	resolved, err := filepath.EvalSymlinks(file)
	if err != nil {
		t.Fatal(err)
	}
	if got := locks(link); len(got) != 1 || got[0] != (LockedEntry{Path: link, Through: resolved}) {
		t.Fatalf("a link to a locked file = %v", got)
	}
}
