//go:build windows

package configuration

import (
	"os/exec"
	"testing"
)

// denyDirectoryWrite makes dir refuse new files while remaining a directory
// that can still be traversed and removed. Windows does not honor the
// read-only file attribute for directories (see
// https://github.com/golang/go/issues/35042), so clearing the owner write
// bit via os.Chmod, as store_unix_test.go does, has no effect here. Reaching
// the same production failure therefore requires an explicit ACL deny entry
// for the well-known Everyone SID, scoped to this directory only (no object
// or container inheritance), which blocks the FILE_ADD_FILE right that
// os.CreateTemp needs.
func denyDirectoryWrite(t *testing.T, dir string) {
	t.Helper()
	if out, err := exec.Command("icacls", dir, "/deny", "*S-1-1-0:(WD,AD)").CombinedOutput(); err != nil {
		t.Fatalf("icacls deny: %v: %s", err, out)
	}
	t.Cleanup(func() {
		if out, err := exec.Command("icacls", dir, "/remove:d", "*S-1-1-0").CombinedOutput(); err != nil {
			t.Errorf("remove temporary deny ACL: %v: %s", err, out)
		}
	})
}
