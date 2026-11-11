package secret

import (
	"errors"
	"io/fs"
	"path/filepath"

	"aigw-cli/internal/platform"
)

// BackendUnavailableFix makes a credential store that AIGW cannot observe
// available before the same command runs again.
const BackendUnavailableFix = "Make the credential store available to this session, such as by unlocking it, then run the same command again."

// unsafeTokenFileError is a Token file whose type or mode AIGW refuses. It keeps
// the file's name, which is the slot and no path.
type unsafeTokenFileError struct{ name, message string }

func (e unsafeTokenFileError) Error() string { return e.message }

// unsafeTokenDirectoryError is a Token directory whose type or mode AIGW refuses.
type unsafeTokenDirectoryError struct{ message string }

func (e unsafeTokenDirectoryError) Error() string { return e.message }

// NamesTokenFolder reports a failure that ObservationFailure tells by the
// path of AIGW's Token folder: a file in the way or a link that leads nowhere,
// or a permission error that the operating system gave at an absolute path.
func NamesTokenFolder(err error) bool {
	if _, blocked := platform.BlockedPath(err); blocked {
		return true
	}
	_, denied := deniedFolder(err)
	return denied
}

// deniedFolder classifies a permission error at an absolute path, which a
// sandbox, system protection or the folder's mode gives.
func deniedFolder(err error) (*platform.PathConditionError, bool) {
	failure, atPath := errors.AsType[*fs.PathError](err)
	if !atPath || !filepath.IsAbs(failure.Path) {
		return nil, false
	}
	classified := platform.ClassifyPath(failure.Path, platform.FolderExpected, err)
	return classified, classified.Condition == platform.PathPermission
}

// ObservationFailure says why a store could not tell whether it holds a slot,
// and the step that addresses it in at. A file in the way of AIGW's Token
// folder, or a link that leads nowhere, is named with the one fix of its
// path's condition; any other cause comes from a closed set, so it names no
// path and none of a backend's own diagnostics.
func ObservationFailure(err error, at platform.PathContext) (cause, fix string) {
	if path, blocked := platform.BlockedPath(err); blocked {
		folder := "AIGW's Token folder"
		if named, _ := platform.HomeTarget(path.Path, at.Home); named != "" {
			folder += " " + named
		}
		return folder + " cannot be used: " + path.Reason(), platform.PathFix(path, at)
	}
	if denied, isDenied := deniedFolder(err); isDenied {
		folder := "AIGW's Token folder"
		if named, _ := platform.HomeTarget(denied.Path, at.Home); named != "" {
			folder += " " + named
		}
		return folder + " cannot be read: " + denied.Reason(), platform.PathFix(denied, at)
	}
	if file, unsafe := errors.AsType[unsafeTokenFileError](err); unsafe {
		return file.Error(), "Make " + file.name + " in AIGW's Token directory " + tokenFileRequirement + ", then run the same command again."
	}
	if directory, unsafe := errors.AsType[unsafeTokenDirectoryError](err); unsafe {
		return directory.Error(), "Make AIGW's Token directory " + tokenDirectoryRequirement + ", then run the same command again."
	}
	return "the credential backend is unavailable", BackendUnavailableFix
}
