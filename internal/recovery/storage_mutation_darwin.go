//go:build darwin

package recovery

import (
	"os"
	"path/filepath"
	"strings"
)

// canonicalRecoveryRootPath resolves the macOS-only alias where /var, /tmp,
// and /etc are symlinks into /private. Recovery roots configured under one
// of those aliases must be canonicalized before the filesystem-root NOFOLLOW
// walk, otherwise every path component comparison would mismatch the real,
// non-symlinked directory tree and the recovery root would never resolve.
func canonicalRecoveryRootPath(path string) string {
	for _, alias := range []string{"/var", "/tmp", "/etc"} {
		if path == alias || strings.HasPrefix(path, alias+string(os.PathSeparator)) {
			return filepath.Join("/private", path)
		}
	}
	return path
}
