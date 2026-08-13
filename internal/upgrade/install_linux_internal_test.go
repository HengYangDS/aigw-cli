//go:build linux

package upgrade

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeParentReadOnly(t *testing.T, path string) {
	t.Helper()
	parent := filepath.Dir(path)
	if err := os.Chmod(parent, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o700) })
}

func TestReplacePortableBinaryPropagatesLinuxWriteFailure(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(executable, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	makeParentReadOnly(t, executable)
	if err := (Updater{Executable: executable}).replacePortableBinary([]byte("next")); err == nil || !strings.Contains(err.Error(), "replace AIGW executable") {
		t.Fatalf("error = %v", err)
	}
}

func TestRollbackPropagatesLinuxWriteFailure(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(executable, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rollbackPath(executable), []byte("previous"), 0o755); err != nil {
		t.Fatal(err)
	}
	makeParentReadOnly(t, executable)
	if _, err := (Updater{Executable: executable}).Rollback(context.Background()); err == nil || !strings.Contains(err.Error(), "restore previous AIGW executable") {
		t.Fatalf("error = %v", err)
	}
}
