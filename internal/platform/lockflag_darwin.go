//go:build darwin

package platform

import "golang.org/x/sys/unix"

// lockFlags are the flags that forbid changing a file whatever its mode
// allows: the owner's and the system's immutable and append-only flags.
const lockFlags = unix.UF_IMMUTABLE | unix.SF_IMMUTABLE | unix.UF_APPEND | unix.SF_APPEND

// lockFlagged reports whether path's own entry carries a flag that forbids
// changing it, read without following a link.
func lockFlagged(path string) bool {
	var stat unix.Stat_t
	if err := unix.Lstat(path, &stat); err != nil {
		return false
	}
	return stat.Flags&lockFlags != 0
}
