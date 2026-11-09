package codex

import (
	"io/fs"

	"aigw-cli/internal/platform"
)

// missingTargetError refuses a Codex configuration file that the
// configuration keeps but that does not exist. AIGW does not create a Codex
// home, so the operator creates the file and the folder that holds it: the
// classification that it wraps says which is missing, and every surface
// renders its one fix from it.
type missingTargetError struct{ failure *platform.PathConditionError }

// newMissingTargetError classifies the configured file at path, which a
// snapshot found absent, as a file that the operator creates.
func newMissingTargetError(path string) missingTargetError {
	return missingTargetError{failure: platform.ClassifyPath(path, platform.FileExpected, fs.ErrNotExist).Owned(true)}
}

func (missingTargetError) Error() string {
	return "Codex config does not exist, and AIGW does not create a Codex home"
}

// Unwrap returns the file's classification, from which the fix is rendered.
func (e missingTargetError) Unwrap() error { return e.failure }

// recordedMissingTargetError refuses to drop a Codex configuration file that
// no longer exists while AIGW's record of it, which can restore what AIGW
// changed there, remains in its folder.
type recordedMissingTargetError struct{ record string }

func (e recordedMissingTargetError) Error() string {
	return "Codex config does not exist, but AIGW's record of it, " + e.record + ", remains in its folder"
}

// NextAction names the record to inspect: restoring the file lets AIGW undo
// its changes, and deleting the record says that the file is gone for good.
func (e recordedMissingTargetError) NextAction() string {
	return "Restore the Codex config, or delete " + e.record + " in its folder if the config is gone for good, then run the same command again."
}
