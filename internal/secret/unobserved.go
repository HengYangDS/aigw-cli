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

// tokenFileError is a failure at one Token file inside AIGW's Token folder,
// as the store reports it: its path is the file's, not the folder's.
type tokenFileError struct{ err error }

func (e tokenFileError) Error() string { return e.err.Error() }
func (e tokenFileError) Unwrap() error { return e.err }

// tokenFileFailure marks err as a failure at a Token file.
func tokenFileFailure(err error) error {
	if err == nil {
		return nil
	}
	return tokenFileError{err: err}
}

// NamesTokenFolder reports a failure that ObservationFailure tells by a path
// of AIGW's Token store: a file in the way of the Token folder or a link that
// leads nowhere, or a permission error that the operating system gave at an
// absolute path of the folder or of a Token file in it.
func NamesTokenFolder(err error) bool {
	if _, blocked := platform.BlockedPath(err); blocked {
		return true
	}
	_, denied := deniedFolder(err)
	return denied
}

// deniedFolder classifies a permission error at an absolute path, which a
// sandbox, system protection or the folder's mode gives. The path is a Token
// file's when the store marked the failure as one, and the folder's otherwise.
func deniedFolder(err error) (*platform.PathConditionError, bool) {
	failure, atPath := errors.AsType[*fs.PathError](err)
	if !atPath || !filepath.IsAbs(failure.Path) {
		return nil, false
	}
	kind := platform.FolderExpected
	if _, file := errors.AsType[tokenFileError](err); file {
		kind = platform.FileExpected
	}
	classified := platform.ClassifyPath(failure.Path, kind, err)
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
		subject, verb := "AIGW's Token folder", " cannot be read: "
		if denied.Kind == platform.FileExpected {
			subject, verb = "AIGW's Token file", " cannot be used: "
		}
		if named, _ := platform.HomeTarget(denied.Path, at.Home); named != "" {
			subject += " " + named
		}
		return subject + verb + denied.Reason(), platform.PathFix(denied, at)
	}
	if file, unsafe := errors.AsType[unsafeTokenFileError](err); unsafe {
		return file.Error(), "Make " + file.name + " in AIGW's Token directory " + tokenFileRequirement + ", then run the same command again."
	}
	if directory, unsafe := errors.AsType[unsafeTokenDirectoryError](err); unsafe {
		return directory.Error(), "Make AIGW's Token directory " + tokenDirectoryRequirement + ", then run the same command again."
	}
	return "the credential backend is unavailable", BackendUnavailableFix
}
