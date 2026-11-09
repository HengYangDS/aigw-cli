//go:build linux

package platform

import "golang.org/x/sys/unix"

// lockFlags are the attributes that forbid changing a file whatever its mode
// allows: immutable and append-only, as chattr +i and +a set them.
const lockFlags = unix.STATX_ATTR_IMMUTABLE | unix.STATX_ATTR_APPEND

// lockFlagged reports whether path's own entry carries an attribute that
// forbids changing it, read without following a link or opening the file. A
// file system that does not report an attribute leaves it unset.
func lockFlagged(path string) bool {
	var stat unix.Statx_t
	if err := unix.Statx(unix.AT_FDCWD, path, unix.AT_SYMLINK_NOFOLLOW, unix.STATX_BASIC_STATS, &stat); err != nil {
		return false
	}
	return stat.Attributes&stat.Attributes_mask&lockFlags != 0
}
