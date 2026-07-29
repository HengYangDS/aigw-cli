//go:build linux

package recovery

import "testing"

func TestCanonicalRecoveryRootPathIsIdentityOnLinux(t *testing.T) {
	for _, path := range []string{
		"/var",
		"/var/lib/aigw-recovery",
		"/tmp/aigw-recovery",
		"/etc/aigw",
		"/home/example/.local/state/aigw/recovery",
	} {
		if got := canonicalRecoveryRootPath(path); got != path {
			t.Fatalf("canonicalRecoveryRootPath(%q) = %q, want the unmodified path", path, got)
		}
	}
}
