//go:build !windows

package config_test

import (
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/config"
)

// denyDirectoryWrite makes dir refuse new files while remaining a directory
// that can still be traversed and removed. This is the exact permission
// model Store.Save's temporary-file creation checks: on POSIX, clearing the
// owner write bit is honored for directories, unlike on Windows (see
// store_windows_test.go and https://github.com/golang/go/issues/35042).
func denyDirectoryWrite(t *testing.T, dir string) {
	t.Helper()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
}

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
