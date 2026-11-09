package terminal

import (
	"errors"
	"io/fs"
	"os"
	"syscall"

	"aigw-cli/internal/platform"
)

// pathReason is why an operation failed at a path: its condition's reason,
// the same on every platform, or the closed-set reason of any other error.
func pathReason(failure *platform.PathConditionError) string {
	if reason := failure.Reason(); reason != "" {
		return reason
	}
	return fileErrorReason(failure.Err)
}

// pathFailureIn is the path failure that err reports: the classification that
// the failed operation made, or else the operating-system error at a path,
// classified here as a folder that an operation creates and otherwise a file.
// An error at no path has none.
func pathFailureIn(err error) *platform.PathConditionError {
	if classified, done := errors.AsType[*platform.PathConditionError](err); done {
		return classified
	}
	if failure, atPath := errors.AsType[*fs.PathError](err); atPath {
		kind := platform.FileExpected
		if failure.Op == "mkdir" {
			kind = platform.FolderExpected
		}
		return platform.ClassifyPath(failure.Path, kind, err)
	}
	if failure, atLink := errors.AsType[*os.LinkError](err); atLink {
		return platform.ClassifyPath(failure.New, platform.FileExpected, err)
	}
	return nil
}

// atOperatingSystem reports whether err holds the operating system's own
// failure at a path, directly or inside the classification made of it, as
// opposed to a refusal that names a path's condition and says why itself.
func atOperatingSystem(err error) bool {
	if path, classified := errors.AsType[*platform.PathConditionError](err); classified {
		err = path.Err
	}
	// A path resolution reports a component that is no folder as the bare
	// number, with no path around it.
	_, atPath := errors.AsType[*fs.PathError](err)
	_, atLink := errors.AsType[*os.LinkError](err)
	_, number := errors.AsType[syscall.Errno](err)
	return atPath || atLink || number
}
