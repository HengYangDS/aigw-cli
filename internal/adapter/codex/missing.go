package codex

// missingTargetError refuses a Codex configuration file that the
// configuration keeps but that does not exist. AIGW does not create a Codex
// home, so Codex moves to a file that exists.
type missingTargetError struct{}

func (missingTargetError) Error() string {
	return "Codex config does not exist, and AIGW does not create a Codex home"
}

// NextAction moves Codex to a configuration file that the user chooses.
func (missingTargetError) NextAction() string { return "aigw client edit codex --target <file>" }

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
