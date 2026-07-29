//go:build !windows

package recovery

import "os"

func platformSnapshotMode(mode os.FileMode) os.FileMode {
	return mode.Perm()
}

func recoveryFileModeIsPrivate(mode os.FileMode) bool {
	return mode.Perm() == 0o600
}

func recoveryDirectoryModeIsPrivate(mode os.FileMode) bool {
	return mode.Perm() == 0o700
}
