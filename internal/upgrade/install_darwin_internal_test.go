//go:build darwin

package upgrade

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

func TestRollbackPropagatesBackupReplacementFailure(t *testing.T) {
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
	if err == nil || !strings.Contains(err.Error(), "restore previous AIGW executable") {
		t.Fatalf("error = %v", err)
	}
	got, readErr := os.ReadFile(executable)
	if readErr != nil || string(got) != "current" {
		t.Fatalf("current executable was not restored: got=%q err=%v", got, readErr)
	}
}
