package discovery

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"aigw-cli/internal/platform"
)

// An executable path that runs through a regular file is a failure to inspect,
// not an absent client: on Windows such a path reads as missing, and discovery
// would report the client as not installed.
func TestAnExecutableThroughAFileIsNotAbsent(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, []byte("x"), 0o700); err != nil {
		t.Fatal(err)
	}
	available, err := executableAvailable("darwin", filepath.Join(file, "codex"))
	if available || err == nil || !errors.Is(err, platform.ErrFileAsFolder) {
		t.Fatalf("an executable through a file = %v, %v", available, err)
	}
}
