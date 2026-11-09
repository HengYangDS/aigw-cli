package platform

import (
	"os"
	"path/filepath"
	"syscall"
)

// RefuseFileAsFolder refuses a path that runs through a regular file, which
// can hold nothing. POSIX reports one as ENOTDIR, but Windows reports
// ERROR_PATH_NOT_FOUND, which Go maps to a missing path, so a check that only
// looks for the file would take it for one that does not exist yet. The
// refusal names the file where a folder is needed, with ENOTDIR on every
// platform. A symbolic link is left to the operation that follows it.
func RefuseFileAsFolder(path string) error {
	for folder := filepath.Dir(filepath.Clean(path)); ; folder = filepath.Dir(folder) {
		if info, err := os.Lstat(folder); err == nil {
			if info.Mode().IsRegular() {
				return &os.PathError{Op: "mkdir", Path: folder, Err: syscall.ENOTDIR}
			}
			return nil
		}
		if parent := filepath.Dir(folder); parent == folder {
			return nil
		}
	}
}
