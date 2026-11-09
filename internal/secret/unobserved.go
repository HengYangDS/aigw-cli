package secret

import (
	"errors"

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
	if file, unsafe := errors.AsType[unsafeTokenFileError](err); unsafe {
		return file.Error(), "Make " + file.name + " in AIGW's Token directory " + tokenFileRequirement + ", then run the same command again."
	}
	if directory, unsafe := errors.AsType[unsafeTokenDirectoryError](err); unsafe {
		return directory.Error(), "Make AIGW's Token directory " + tokenDirectoryRequirement + ", then run the same command again."
	}
	return "the credential backend is unavailable", BackendUnavailableFix
}
