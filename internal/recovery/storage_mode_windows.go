//go:build windows

package recovery

import "os"

func platformSnapshotMode(mode os.FileMode) os.FileMode {
	if mode.Perm()&0o200 == 0 {
		return 0o444
	}
	return 0o666
}

func recoveryFileModeIsPrivate(os.FileMode) bool {
	return true
}

func recoveryDirectoryModeIsPrivate(os.FileMode) bool {
	return true
}
