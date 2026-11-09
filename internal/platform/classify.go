package platform

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// PathKind is what an operation expects at the path that it acted on, which
// decides whether a regular file there stands in its way.
type PathKind int

const (
	// FileExpected is a path that the operation reads, writes or replaces. A
	// regular file there is what it acts on, never in its way.
	FileExpected PathKind = iota
	// FolderExpected is a path that the operation creates or enters.
	FolderExpected
)

// PathCondition is the one condition of an operation that failed at a path.
type PathCondition int

const (
	// PathOther is a failure that no other condition describes. Its error is
	// presented as it is.
	PathOther PathCondition = iota
	// PathPermission is a permission error with no file in the way.
	PathPermission
	// PathFileInTheWay is a regular file that stands where a folder is needed:
	// a strict ancestor of a file path, or a folder path or one of its
	// ancestors.
	PathFileInTheWay
	// PathDanglingLink is a symbolic link on the path that leads to nothing,
	// or, where a folder is needed, to no folder.
	PathDanglingLink
	// PathMissingFolder is a path whose folder does not exist, with nothing
	// above it in the way.
	PathMissingFolder
	// PathMissingFile is a file path whose folder exists while the file does
	// not.
	PathMissingFile
)

// PathConditionError is an operation that failed at Path, classified once for
// every surface that presents it: its Condition, the regular file in the way
// or the link that Culprit names, and the operation and error that failed. A
// file in the way reads as ErrFileAsFolder on every platform, never as the
// missing path that Windows reports, and a dangling link as neither missing
// nor existing. The surface that classified it says whether the operator gave
// Path, as a --path or a client's file is given, which a command may then
// name outside the home directory, and whether the operator creates what is
// missing, as for a client's file that AIGW does not create.
type PathConditionError struct {
	Condition       PathCondition
	Path            string
	Kind            PathKind
	Culprit         string
	Op              string
	Err             error
	Given           bool
	OperatorCreates bool
	// Restore, which the owner of a folder of AIGW's own sets, is the step
	// that brings back what the folder held when no copy of it does, such as
	// the Tokens of AIGW's Token folder.
	Restore string
	// toFile marks a link that leads to a regular file where a folder is
	// needed: no broken link, but no folder either.
	toFile bool
}

// Restoring is the failure of a folder of AIGW's own whose contents restore
// brings back when no copy of the folder does.
func (e *PathConditionError) Restoring(restore string) *PathConditionError {
	restoring := *e
	restoring.Restore = restore
	return &restoring
}

// Owned is the failure as given by the operator, who creates what is missing
// when operatorCreates, as the surface that knows the path's owner says.
func (e *PathConditionError) Owned(operatorCreates bool) *PathConditionError {
	owned := *e
	owned.Given, owned.OperatorCreates = true, operatorCreates
	return &owned
}

// reasonOr is the failure's reason, or fallback for a failure without one.
func (e *PathConditionError) reasonOr(fallback string) string {
	if reason := e.Reason(); reason != "" {
		return reason
	}
	return fallback
}

// ClassifyPath classifies err, the failure of an operation at path, which the
// operation expects to be kind. A file in the way and a dangling link are found
// on the file system, never in the error's text or number: Windows reports a
// path through a regular file as ERROR_PATH_NOT_FOUND, which a missing folder
// returns too. Any other error is PathOther. The result is never nil.
func ClassifyPath(path string, kind PathKind, err error) *PathConditionError {
	// An operation that classified its own failure keeps that classification,
	// which a dangling link, unwrapping to nothing, would otherwise lose.
	if classified, done := errors.AsType[*PathConditionError](err); done {
		return classified
	}
	failure := &PathConditionError{Path: path, Kind: kind, Op: operation(err), Err: err}
	if !aboutAPath(err) {
		return failure
	}
	if culprit, condition, found := culpritOn(path, kind); found {
		failure.Condition, failure.Culprit = condition, culprit
		failure.toFile = condition == PathDanglingLink && exists(culprit)
		return failure
	}
	switch {
	case errors.Is(err, fs.ErrPermission):
		failure.Condition = PathPermission
	case errors.Is(err, fs.ErrNotExist):
		failure.Condition = PathMissingFolder
		if kind == FileExpected && folderExists(filepath.Dir(filepath.Clean(path))) {
			failure.Condition = PathMissingFile
		}
	}
	return failure
}

// BlockedPath is the classification that err holds of a path that a file in
// the way or a link that leads nowhere blocks, and whether it holds one.
func BlockedPath(err error) (*PathConditionError, bool) {
	failure, classified := errors.AsType[*PathConditionError](err)
	return failure, classified && failure.Blocked()
}

// RefuseBlockedFile refuses, before an operation, a file path that a file in
// the way or a dangling link blocks, with its classification. A file that is
// there, or merely missing, is not refused.
func RefuseBlockedFile(path string) error {
	_, err := os.Lstat(path)
	if err == nil {
		return nil
	}
	if failure := ClassifyPath(path, FileExpected, err); failure.Blocked() {
		return failure
	}
	return nil
}

// Error names the path and why it failed: a file in the way and a dangling
// link by their condition, and any other failure by its own error.
func (e *PathConditionError) Error() string {
	if e.Blocked() {
		if e.Op != "" {
			return e.Op + " " + e.Path + ": " + e.Reason()
		}
		return e.Path + ": " + e.Reason()
	}
	if e.Err == nil {
		return e.Path + ": unexpected error"
	}
	return e.Err.Error()
}

// Unwrap returns ErrFileAsFolder for a file in the way, nothing for a
// dangling link, and the operation's own error otherwise, so that neither
// reads as a missing path.
func (e *PathConditionError) Unwrap() error {
	switch e.Condition {
	case PathFileInTheWay:
		return ErrFileAsFolder
	case PathDanglingLink:
		if e.toFile {
			return ErrFileAsFolder
		}
		return nil
	case PathOther, PathPermission, PathMissingFolder, PathMissingFile:
		return e.Err
	}
	return e.Err
}

// Reason says why the operation failed, the same on every platform, naming no
// path: its condition's reason, or the operating system's number for any other
// failure, which otherwise has no reason of its own.
func (e *PathConditionError) Reason() string {
	switch e.Condition {
	case PathPermission:
		return "permission denied"
	case PathFileInTheWay:
		return ErrFileAsFolder.Error()
	case PathDanglingLink:
		// A link to a regular file is not broken; it leads to no folder.
		if e.toFile {
			return ErrFileAsFolder.Error()
		}
		return "broken symbolic link"
	case PathMissingFolder, PathMissingFile:
		return "no such file or directory"
	case PathOther:
		// The operating system's own number, which names no path; any other
		// error's text may, and has no reason here.
		if number, ok := errors.AsType[syscall.Errno](e.Err); ok {
			return number.Error()
		}
		if errors.Is(e.Err, fs.ErrExist) {
			return "file exists"
		}
	}
	return ""
}

// Blocked reports a file in the way or a dangling link, which no operation at
// the path gets past.
func (e *PathConditionError) Blocked() bool {
	return e.Condition == PathFileInTheWay || e.Condition == PathDanglingLink
}

// Absent reports a path that holds nothing yet, with nothing in its way.
func (e *PathConditionError) Absent() bool {
	return e.Condition == PathMissingFolder || e.Condition == PathMissingFile
}

// aboutAPath reports an error that a path's state can explain.
func aboutAPath(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission) ||
		errors.Is(err, fs.ErrExist) || errors.Is(err, syscall.ENOTDIR)
}

// culpritOn finds the regular file in the way or the dangling link on path:
// for a file path, a link at the file itself that leads to nothing, and then
// its ancestors; for a folder path, the path itself and its ancestors. The
// deepest entry that exists decides, as the operating system resolves the
// path.
func culpritOn(path string, kind PathKind) (string, PathCondition, bool) {
	folder := filepath.Clean(path)
	if kind == FileExpected {
		if info, err := os.Lstat(folder); err == nil {
			if info.Mode()&os.ModeSymlink != 0 && !exists(folder) {
				return folder, PathDanglingLink, true
			}
			return "", PathOther, false
		}
		folder = filepath.Dir(folder)
	}
	for {
		info, err := os.Lstat(folder)
		switch {
		case err != nil:
			// Missing, or not readable: the entry above decides.
		case info.Mode().IsRegular():
			return folder, PathFileInTheWay, true
		case info.Mode()&os.ModeSymlink != 0 && !folderExists(folder):
			return folder, PathDanglingLink, true
		default:
			return "", PathOther, false
		}
		parent := filepath.Dir(folder)
		if parent == folder {
			return "", PathOther, false
		}
		folder = parent
	}
}

// operation is the operation that err names, when it names one.
func operation(err error) string {
	if failure, atPath := errors.AsType[*fs.PathError](err); atPath {
		return failure.Op
	}
	if failure, atLink := errors.AsType[*os.LinkError](err); atLink {
		return failure.Op
	}
	return ""
}

// exists reports whether path, followed through its links, names an entry.
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// folderExists reports whether path, followed through its links, names a
// folder.
func folderExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
