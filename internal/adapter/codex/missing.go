package codex

import (
	"os"
	"path/filepath"
)

// missingTargetError refuses a Codex configuration file that the
// configuration keeps but that does not exist. AIGW does not create a Codex
// home. When the file's folder is missing too, another target that names a
// file in it would be refused the same way, so the fix names the folder.
type missingTargetError struct{ path string }

func (missingTargetError) Error() string {
	return "Codex config does not exist, and AIGW does not create a Codex home"
}

// NextAction creates the missing folder, or moves Codex to a file that exists.
func (e missingTargetError) NextAction() string {
	if folder := filepath.Dir(e.path); e.path != "" && !folderExists(folder) {
		return "Create " + folder + ", or choose another path, then run the same command again."
	}
	return "aigw client edit codex --target <file>"
}

// folderExists reports whether path names a folder.
func folderExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
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
