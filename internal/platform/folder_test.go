package platform

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// A path through a file is refused, naming the file; a path under a folder
// that does not exist yet is not.
func TestRefuseFileAsFolderNamesTheFile(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(file, "a"), filepath.Join(file, "a", "b")} {
		failure, atFile := errors.AsType[*os.PathError](RefuseFileAsFolder(path))
		if !atFile || failure.Path != file || !errors.Is(failure, syscall.ENOTDIR) {
			t.Fatalf("%s = %#v", path, failure)
		}
	}
	if err := RefuseFileAsFolder(filepath.Join(root, "missing", "a")); err != nil {
		t.Fatalf("a folder that does not exist yet = %v", err)
	}
}
