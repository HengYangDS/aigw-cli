package claude_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aigw-cli/internal/claude"
)

func TestIntegrationRequiresRealExecutable(t *testing.T) {
	for _, test := range []struct {
		name string
		path string
		want bool
	}{
		{name: "empty"},
		{name: "missing", path: filepath.Join(t.TempDir(), "missing")},
		{name: "directory", path: t.TempDir()},
	} {
		t.Run(test.name, func(t *testing.T) {
			ready, err := claude.Ready(test.path)
			if err != nil || ready != test.want {
				t.Fatalf("Ready(%q) = %v, %v; want %v", test.path, ready, err, test.want)
			}
		})
	}
}

func TestIntegrationRejectsARegularNonExecutableFileOnPOSIX(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows executability is extension-based")
	}
	path := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	ready, err := claude.Ready(path)
	if err != nil || ready {
		t.Fatalf("Ready()=%v, %v", ready, err)
	}
}

func TestIntegrationAcceptsNativeExecutableWithoutWritingAnything(t *testing.T) {
	root := t.TempDir()
	extension := ""
	mode := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		extension = ".exe"
		mode = 0o600
	}
	executable := filepath.Join(root, "claude"+extension)
	if err := os.WriteFile(executable, []byte("fixture"), mode); err != nil {
		t.Fatal(err)
	}
	ready, err := claude.Ready(executable)
	if err != nil || !ready {
		t.Fatalf("Ready() = %v, %v", ready, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(executable) {
		t.Fatalf("integration wrote unexpected files: %#v", entries)
	}
}

func TestIntegrationReportsInspectionFailure(t *testing.T) {
	root := t.TempDir()
	loop := filepath.Join(root, "loop")
	if err := os.Symlink(loop, loop); err != nil {
		t.Skipf("symbolic link unavailable: %v", err)
	}
	ready, err := claude.Ready(loop)
	if ready || err == nil || !strings.Contains(err.Error(), "inspect Claude executable") {
		t.Fatalf("Ready() = %v, %v", ready, err)
	}
}
