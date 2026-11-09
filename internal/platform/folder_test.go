package platform

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// A missing path is absent only when no regular file stands on it, whatever
// error the platform reports for one: Windows reports a file in the way as a
// missing path, and its own missing-path error is the same.
func TestMissingAtNamesTheFileInTheWay(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	under, itself := filepath.Join(file, "config.toml"), filepath.Join(root, "new")
	missing := func(path string) error { return &os.PathError{Op: "open", Path: path, Err: fs.ErrNotExist} }
	for _, test := range []struct {
		name, path string
		kind       PathKind
		err        error
		absent     bool
		inTheWay   string
	}{
		{name: "a file in the way", path: under, err: missing(under), inTheWay: file},
		{name: "a file in the way reported as ENOTDIR", path: under, err: &os.PathError{Op: "open", Path: under, Err: syscall.ENOTDIR}, inTheWay: file},
		{name: "the folder path is itself the file", path: file, kind: FolderExpected, err: &os.PathError{Op: "mkdir", Path: file, Err: syscall.ENOTDIR}, inTheWay: file},
		{name: "nothing in the way", path: itself, err: missing(itself), absent: true},
		{name: "nothing in the way of a path through a missing folder", path: filepath.Join(root, "new", "deeper", "a"), err: missing(filepath.Join(root, "new", "deeper", "a")), absent: true},
		{name: "another error", path: under, err: &os.PathError{Op: "open", Path: under, Err: syscall.EIO}},
	} {
		absent, refusal := MissingAt(test.path, test.kind, test.err)
		inTheWay := ""
		if failure, atFile := errors.AsType[*PathConditionError](refusal); atFile {
			inTheWay = failure.Culprit
			if !errors.Is(refusal, syscall.ENOTDIR) {
				t.Fatalf("%s: the refusal does not read as not a directory: %v", test.name, refusal)
			}
		}
		if absent != test.absent || inTheWay != test.inTheWay {
			t.Fatalf("%s = absent %v, in the way %q; want absent %v, in the way %q", test.name, absent, inTheWay, test.absent, test.inTheWay)
		}
	}
}

// A symlink to a folder is not a file in the way: an operation through it
// succeeds. A dangling one is absent on this host, which the operation itself
// reports.
func TestMissingAtFollowsASymlinkToAFolder(t *testing.T) {
	root := t.TempDir()
	folder := filepath.Join(root, "folder")
	if err := os.Mkdir(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(folder, link); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	path := filepath.Join(link, "config.toml")
	absent, refusal := MissingAt(path, FileExpected, &os.PathError{Op: "open", Path: path, Err: fs.ErrNotExist})
	if !absent || refusal != nil {
		t.Fatalf("a path through a folder link = absent %v, %v", absent, refusal)
	}
}
