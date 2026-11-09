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

// A write that a flag refuses names the entry that carries it: the file, or
// the folder holding it, whose lock refuses the rename that replaces the file.
// A write that no flag refuses, and any read, carries none.
func TestAWriteNamesTheEntryThatAFlagLocks(t *testing.T) {
	folder := filepath.Join(t.TempDir(), "work")
	if err := os.Mkdir(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(folder, "config.toml")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	denied := &os.PathError{Op: "open", Path: file, Err: syscall.EPERM}
	classify := func() *PathConditionError { return ClassifyPath(file, FileExpected, denied) }
	if written := classify().Written(); written.Locked != "" {
		t.Fatalf("a write that no flag refused names %q", written.Locked)
	}
	lock(t, file)
	if written := classify().Written(); written.Locked != file {
		t.Fatalf("a locked file = %q", written.Locked)
	}
	if read := classify(); read.Locked != "" {
		t.Fatalf("a read names the flag of %q", read.Locked)
	}
	_ = unix.Chflags(file, 0)
	lock(t, folder)
	if written := classify().Written(); written.Locked != folder {
		t.Fatalf("a locked folder = %q", written.Locked)
	}
}
