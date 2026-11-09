package platform

import (
	"os"
	"path/filepath"
	"syscall"
)

// ErrFileAsFolder is a regular file where a path needs a folder. POSIX
// reports one as ENOTDIR, but Windows reports ERROR_PATH_NOT_FOUND, "The
// system cannot find the path specified", which Go takes for a missing path
// and whose Errno even ENOTDIR is there. This error says the same thing on
// every platform, and [errors.Is] accepts [syscall.ENOTDIR] for it.
var ErrFileAsFolder = fileAsFolderError{}

type fileAsFolderError struct{}

func (fileAsFolderError) Error() string { return "not a directory" }

// Is lets callers that test for ENOTDIR find this error.
func (fileAsFolderError) Is(target error) bool { return target == syscall.ENOTDIR }

// RefuseFileAsFolder refuses a path that runs through a regular file, which
// can hold nothing, naming the file where a folder is needed with
// ErrFileAsFolder. A check that only looks for the path would take one
// through a file for a path that does not exist yet on Windows. A symbolic
// link is left to the operation that follows it.
func RefuseFileAsFolder(path string) error {
	for folder := filepath.Dir(filepath.Clean(path)); ; folder = filepath.Dir(folder) {
		if info, err := os.Lstat(folder); err == nil {
			if info.Mode().IsRegular() {
				return &os.PathError{Op: "mkdir", Path: folder, Err: ErrFileAsFolder}
			}
			return nil
		}
		if parent := filepath.Dir(folder); parent == folder {
			return nil
		}
	}
}
