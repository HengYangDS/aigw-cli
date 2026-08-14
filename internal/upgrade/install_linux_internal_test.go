//go:build linux

package upgrade

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func blockPortableStagingPath(t *testing.T, executable string) {
	t.Helper()
	staging := filepath.Join(filepath.Dir(executable), "."+filepath.Base(executable)+".new")
	if err := os.Mkdir(staging, 0o700); err != nil {
		t.Fatal(err)
	}
}

func TestReplacePortableBinaryPropagatesLinuxWriteFailure(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(executable, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	blockPortableStagingPath(t, executable)
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
	blockPortableStagingPath(t, executable)
	if _, err := (Updater{Executable: executable}).Rollback(context.Background()); err == nil || !strings.Contains(err.Error(), "restore previous AIGW executable") {
		t.Fatalf("error = %v", err)
	}
}
