//go:build linux

package recovery

// canonicalRecoveryRootPath is the identity function on Linux. Unlike
// macOS, Linux does not alias /var, /tmp, or /etc into a separate /private
// tree, so a configured recovery root already names the real directory the
// filesystem-root NOFOLLOW walk must match component-by-component.
func canonicalRecoveryRootPath(path string) string {
	return path
}
