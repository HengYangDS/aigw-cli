package platform

import (
	"errors"
	"io/fs"
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

// MissingAt classifies err, the failure of an operation at path, which the
// operation expects to be kind, for a check that takes a missing path for an
// absent one: absent reports that path holds nothing yet, and refusal, when a
// regular file stands in its way, is that classification. Windows reports a
// path through a regular file as ERROR_PATH_NOT_FOUND, which a missing path
// returns too, so only ClassifyPath's look at the file system tells them
// apart. A link that leads nowhere reads as absent here, as before; the
// operation that follows it reports it. Any other error returns both false.
func MissingAt(path string, kind PathKind, err error) (absent bool, refusal error) {
	failure := ClassifyPath(path, kind, err)
	if failure.Condition == PathFileInTheWay {
		return false, failure
	}
	return errors.Is(err, fs.ErrNotExist), nil
}
