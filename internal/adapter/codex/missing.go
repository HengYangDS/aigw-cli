package codex

import (
	"errors"
	"os"
	"path/filepath"

	"aigw-cli/internal/platform"
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
// On Windows a path through a regular file reads as a missing file, so a file
// that stands where the folder should be is named to move, never to create.
func (e missingTargetError) NextAction() string {
	if e.path == "" {
		return "aigw client edit codex --target <file>"
	}
	if failure, atFile := errors.AsType[*os.PathError](platform.RefuseFileAsFolder(e.path)); atFile {
		return "Move the file " + failure.Path + ", or choose another path, then run the same command again."
	}
	if folder := filepath.Dir(e.path); !folderExists(folder) {
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
