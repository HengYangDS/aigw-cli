package platform

import (
	"errors"
	"io/fs"
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
	return fileOnPath(path, false)
}

// MissingAt classifies err, the failure of an operation at path, as a missing
// path: absent reports that path holds nothing yet, and refusal, when err reads
// as missing but a regular file stands on the path, names that file with
// ErrFileAsFolder. An error that is not about a missing path returns both
// false. Windows reports a path through a regular file as ERROR_PATH_NOT_FOUND
// and a missing folder the same way, so only the file system tells them apart,
// and the answer is right on a host whose error text differs from its files.
func MissingAt(path string, err error) (absent bool, refusal error) {
	if !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, syscall.ENOTDIR) {
		return false, nil
	}
	if failure := fileOnPath(path, true); failure != nil {
		return false, failure
	}
	return errors.Is(err, fs.ErrNotExist), nil
}

// FileInTheWay names the regular file that stands where a folder is needed on
// path, reporting it as a refusal with ErrFileAsFolder, or returns nil. Unlike
// RefuseFileAsFolder it includes path itself, because an operation that
// creates a directory reports the blocking file as the path.
func FileInTheWay(path string) error {
	return fileOnPath(path, true)
}

// DanglingLinkInPath names the symbolic link on path, which is path itself or
// one of its folders, that leads to nothing or to no folder, and reports
// whether there is one. A create through such a link fails with "file exists"
// on POSIX and with a missing path on Windows, so a check that only asks
// whether the folder exists reads the link as a folder to create.
func DanglingLinkInPath(path string) (string, bool) {
	for folder := filepath.Clean(path); ; folder = filepath.Dir(folder) {
		info, err := os.Lstat(folder)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			target, statErr := os.Stat(folder)
			if statErr != nil || !target.IsDir() {
				return folder, true
			}
		}
		if err == nil || filepath.Dir(folder) == folder {
			return "", false
		}
	}
}

// fileOnPath finds the regular file that stands where a folder is needed on
// path, naming it with ErrFileAsFolder, or returns nil. withPath includes path
// itself, which an operation that creates it reports, while a path that only
// names a file to read or replace starts at its folder.
func fileOnPath(path string, withPath bool) error {
	path = filepath.Clean(path)
	if !withPath {
		path = filepath.Dir(path)
	}
	for folder := path; ; folder = filepath.Dir(folder) {
		info, err := os.Lstat(folder)
		if err == nil && info.Mode().IsRegular() {
			return &os.PathError{Op: "mkdir", Path: folder, Err: ErrFileAsFolder}
		}
		if err == nil || filepath.Dir(folder) == folder {
			return nil
		}
	}
}
