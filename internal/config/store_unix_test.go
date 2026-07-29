//go:build !windows

package config_test

import (
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/config"
)

func TestSaveSurfacesWriteFailureBeyondFileSizeLimit(t *testing.T) {
	signal.Ignore(syscall.SIGXFSZ)
	var original syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &original); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		signal.Reset(syscall.SIGXFSZ)
		_ = syscall.Setrlimit(syscall.RLIMIT_FSIZE, &original)
	})
	tiny := syscall.Rlimit{Cur: 1, Max: original.Max}
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &tiny); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := config.NewStore(path).Save(convergenceConfig("current")); err == nil || !strings.Contains(err.Error(), "write temporary file") {
		t.Fatalf("Save() error = %v, want a write failure once the file-size rlimit is exceeded", err)
	}
}
