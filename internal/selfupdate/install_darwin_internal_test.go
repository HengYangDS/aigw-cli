//go:build darwin

package selfupdate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func makeImmutable(t *testing.T, path string) {
	t.Helper()
	if err := unix.Chflags(path, unix.UF_IMMUTABLE); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = unix.Chflags(path, 0) })
}

func TestDetectChannelDarwinPathHeuristic(t *testing.T) {
	previous := InstallChannel
	t.Cleanup(func() { InstallChannel = previous })
	InstallChannel = "bogus"
	t.Setenv("AIGW_INSTALL_CHANNEL", "")
	if got := detectChannel("/usr/local/bin/aigw"); got != ChannelPKG {
		t.Fatalf("channel = %q, want pkg", got)
	}
	if got := detectChannel("/opt/aigw/aigw"); got != ChannelPortable {
		t.Fatalf("channel = %q, want portable", got)
	}
}

func TestReplacePortableBinaryPropagatesWriteFailure(t *testing.T) {
	directory := t.TempDir()
	executable := filepath.Join(directory, "aigw")
	if err := os.WriteFile(executable, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	makeImmutable(t, executable)
	u := Updater{Executable: executable}
	if err := u.replacePortableBinary([]byte("new-binary")); err == nil || !strings.Contains(err.Error(), "replace AIGW executable") {
		t.Fatalf("error = %v", err)
	}
}

func TestRollbackPropagatesRestoreWriteFailure(t *testing.T) {
	directory := t.TempDir()
	executable := filepath.Join(directory, "aigw")
	if err := os.WriteFile(executable, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rollbackPath(executable), []byte("previous"), 0o755); err != nil {
		t.Fatal(err)
	}
	makeImmutable(t, executable)
	u := Updater{Channel: ChannelPortable, Executable: executable}
	if _, err := u.Rollback(context.Background()); err == nil || !strings.Contains(err.Error(), "restore previous AIGW executable") {
		t.Fatalf("error = %v", err)
	}
}

func TestRollbackReportsBackupSaveFailureAndRestoresCurrentBinary(t *testing.T) {
	directory := t.TempDir()
	executable := filepath.Join(directory, "aigw")
	if err := os.WriteFile(executable, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	backup := rollbackPath(executable)
	if err := os.WriteFile(backup, []byte("previous"), 0o755); err != nil {
		t.Fatal(err)
	}
	makeImmutable(t, backup)
	u := Updater{Channel: ChannelPortable, Executable: executable}
	_, err := u.Rollback(context.Background())
	if err == nil || !strings.Contains(err.Error(), "save reversible AIGW rollback copy") {
		t.Fatalf("error = %v", err)
	}
	got, readErr := os.ReadFile(executable)
	if readErr != nil || string(got) != "current" {
		t.Fatalf("current executable was not restored: got=%q err=%v", got, readErr)
	}
}

func TestScheduleWindowsRollbackPropagatesStagingWriteFailure(t *testing.T) {
	withFakeCmd(t, true)
	executable := filepath.Join(t.TempDir(), "aigw.exe")
	if err := os.WriteFile(executable, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rollbackPath(executable), []byte("previous"), 0o755); err != nil {
		t.Fatal(err)
	}
	staged := windowsRollbackStagePath(executable)
	if err := os.WriteFile(staged, []byte("placeholder"), 0o755); err != nil {
		t.Fatal(err)
	}
	makeImmutable(t, staged)
	u := Updater{Executable: executable}
	if _, err := u.scheduleWindowsRollback(); err == nil || !strings.Contains(err.Error(), "stage Windows AIGW rollback") {
		t.Fatalf("error = %v", err)
	}
}

func TestScheduleWindowsRollbackPropagatesScriptWriteFailure(t *testing.T) {
	withFakeCmd(t, true)
	executable := filepath.Join(t.TempDir(), "aigw.exe")
	if err := os.WriteFile(executable, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rollbackPath(executable), []byte("previous"), 0o755); err != nil {
		t.Fatal(err)
	}
	script := executable + ".rollback.cmd"
	if err := os.WriteFile(script, []byte("placeholder"), 0o600); err != nil {
		t.Fatal(err)
	}
	makeImmutable(t, script)
	u := Updater{Executable: executable}
	_, err := u.scheduleWindowsRollback()
	if err == nil || !strings.Contains(err.Error(), "write Windows AIGW rollback helper") {
		t.Fatalf("error = %v", err)
	}
	if _, statErr := os.Stat(windowsRollbackStagePath(executable)); !os.IsNotExist(statErr) {
		t.Fatalf("staged rollback binary was not cleaned up: %v", statErr)
	}
}
