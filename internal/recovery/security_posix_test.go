//go:build !windows

package recovery

import (
	"os"
	"sync"
	"testing"
)

const (
	ledgerSecurityFailureReasonForTest     = AirRecoveryReasonLedgerPermission
	quarantineSecurityFailureReasonForTest = AirRecoveryReasonQuarantinePermission
)

func makeRecoveryFileSecurityInvalidForTest(t *testing.T, path string) {
	t.Helper()
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
}

func makeRecoveryDirectorySecurityInvalidForTest(t *testing.T, path string) {
	t.Helper()
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func makeRecoveryPathUnreadableForTest(t *testing.T, path string) func() {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	restore := func() {
		once.Do(func() {
			if err := os.Chmod(path, info.Mode().Perm()); err != nil && !os.IsNotExist(err) {
				t.Errorf("restore permissions for %s: %v", path, err)
			}
		})
	}
	t.Cleanup(restore)
	return restore
}

func assertRecoveryDirectorySecurityFaultForTest(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("read-only inspection removed unsafe directory: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("read-only inspection changed unsafe directory mode: %v", info.Mode().Perm())
	}
}

func assertRecoveryFileModeForTest(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != want.Perm() {
		t.Fatalf("mode for %s = %o, want %o", path, info.Mode().Perm(), want.Perm())
	}
}

func createRecoveryLinkForTest(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

func createRecoveryCaptureFailureForTest(t *testing.T, path string) {
	t.Helper()
	createRecoveryLinkForTest(t, path+".missing", path)
}
