package adapters

import (
	"os"
	"runtime"
)

// expectedMode returns the effective os.FileMode that the current platform
// will actually use when requested to set a certain mode.
func expectedMode(mode os.FileMode) os.FileMode {
	if runtime.GOOS == "windows" {
		// On Windows, Go's os.Chmod only sets the read-only attribute.
		// If any write bit is set, it becomes 0666. Otherwise 0444.
		if mode&0222 != 0 {
			return 0666
		}
		return 0444
	}
	return mode
}
