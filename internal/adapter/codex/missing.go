package codex

import (
	"aigw-cli/internal/adapter"
)

// missingTargetError refuses a Codex configuration file that the
// configuration keeps but that does not exist. AIGW does not create a Codex
// home, so the fix names the file and the folder that holds it to create,
// because creating the folder alone leaves the same command refused. home
// names a file inside it as ~/…. A caller that knows the home directory
// renders the same fix through TargetFix.
type missingTargetError struct{ path, home string }

func (missingTargetError) Error() string {
	return "Codex config does not exist, and AIGW does not create a Codex home"
}

// NextAction creates the missing file and the folder that holds it, or moves
// Codex to a file that exists. On Windows a path through a regular file reads
// as a missing file, so a file that stands where the folder should be is named
// to move, never to create.
func (e missingTargetError) NextAction() string { return e.TargetFix(e.home) }

// TargetFix renders NextAction against home, which names a file inside it as
// ~/… and a file outside it as no path at all.
func (e missingTargetError) TargetFix(home string) string {
	if e.path == "" {
		return "aigw client edit codex --target <file>"
	}
	return adapter.MissingTargetFix(e.path, home)
}

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
