//go:build windows

package recovery

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"golang.org/x/sys/windows"
)

const (
	ledgerSecurityFailureReasonForTest     = AirRecoveryReasonLedgerUnreadable
	quarantineSecurityFailureReasonForTest = AirRecoveryReasonQuarantineUnreadable
)

func makeRecoveryFileSecurityInvalidForTest(t *testing.T, path string) {
	t.Helper()
	lockRecoveryPathForTest(t, path)
}

func makeRecoveryDirectorySecurityInvalidForTest(t *testing.T, path string) {
	t.Helper()
	lockRecoveryPathForTest(t, path)
}

func makeRecoveryPathUnreadableForTest(t *testing.T, path string) func() {
	t.Helper()
	return lockRecoveryPathForTest(t, path)
}

func lockRecoveryPathForTest(t *testing.T, path string) func() {
	t.Helper()
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(
		name,
		windows.GENERIC_READ,
		0,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		t.Fatalf("exclusively open %s: %v", path, err)
	}
	var once sync.Once
	release := func() {
		once.Do(func() {
			if err := windows.CloseHandle(handle); err != nil {
				t.Errorf("close exclusive handle for %s: %v", path, err)
			}
		})
	}
	t.Cleanup(release)
	return release
}

func assertRecoveryDirectorySecurityFaultForTest(t *testing.T, path string) {
	t.Helper()
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(
		name,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if handle != windows.InvalidHandle {
		_ = windows.CloseHandle(handle)
	}
	if !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
		t.Fatalf("exclusive directory protection was not preserved: %v", err)
	}
}

func assertRecoveryFileModeForTest(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	want = platformSnapshotMode(want)
	if info.Mode().Perm() != want {
		t.Fatalf("Windows mode for %s = %o, want %o", path, info.Mode().Perm(), want)
	}
}

func createRecoveryLinkForTest(t *testing.T, target, link string) {
	t.Helper()
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.IsDir() {
		createRecoveryJunctionForTest(t, target, link)
		return
	}
	payload, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	linkParent := filepath.Dir(link)
	if err := os.Remove(linkParent); err != nil {
		t.Fatalf("remove empty recovery link parent: %v", err)
	}
	targetDirectory := target + ".junction-target"
	if err := os.Mkdir(targetDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetDirectory, filepath.Base(link)), payload, 0o600); err != nil {
		t.Fatal(err)
	}
	createRecoveryJunctionForTest(t, targetDirectory, linkParent)
}

func createRecoveryJunctionForTest(t *testing.T, target, link string) {
	t.Helper()
	output, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput()
	if err != nil {
		t.Fatalf("create recovery junction: %v: %s", err, output)
	}
}

func createRecoveryCaptureFailureForTest(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("locked ledger"), 0o600); err != nil {
		t.Fatal(err)
	}
	lockRecoveryPathForTest(t, path)
}
