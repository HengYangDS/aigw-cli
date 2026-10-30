package claudedesktop_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"aigw-cli/internal/adapter/claudedesktop"
	"aigw-cli/internal/adapter/discovery"
)

// desktopHost returns a supported Claude Desktop platform for this test host:
// Linux has no official application, so a Linux host observes macOS paths.
func desktopHost(t *testing.T) (string, string, map[string]string) {
	t.Helper()
	goos := runtime.GOOS
	if goos == "linux" {
		goos = "darwin"
	}
	if goos != "darwin" && goos != "windows" {
		t.Skip("unsupported test host")
	}
	root := t.TempDir()
	return goos, root, map[string]string{"HOME": root, "LOCALAPPDATA": root}
}

func TestDiscoverReturnsClaudeDesktopApplicationAndConfigurationLibrary(t *testing.T) {
	goos, root, env := desktopHost(t)
	executable := claudedesktop.ApplicationPathsFor(goos, env)[0]
	if err := os.MkdirAll(filepath.Dir(executable), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	library, err := claudedesktop.LibraryPathFor(goos, env)
	if err != nil {
		t.Fatal(err)
	}
	result := claudedesktop.New().Discover(discovery.System{GOOS: goos, Home: root, Env: map[string]string{"LOCALAPPDATA": root}})
	if got := result.Executable(claudedesktop.ID); got != executable {
		t.Fatalf("Claude Desktop executable = %q, want %q", got, executable)
	}
	surface, ok := result.Surface(string(claudedesktop.LibrarySurface))
	if !ok || surface.ConfigPath != library || surface.Executable != executable || !surface.AutoManaged || !surface.Present {
		t.Fatalf("Claude Desktop surface = %#v, %t", surface, ok)
	}
}

func TestClaudeDesktopDiscoveryUsesNativeApplicationPath(t *testing.T) {
	goos, root, env := desktopHost(t)
	candidate := claudedesktop.ApplicationPathsFor(goos, env)[0]
	if err := os.MkdirAll(filepath.Dir(candidate), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(candidate, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	result := claudedesktop.New().Discover(discovery.System{GOOS: goos, Home: root, Env: map[string]string{"LOCALAPPDATA": root}})
	if got := result.Executable(claudedesktop.ID); got != candidate {
		t.Fatalf("Claude Desktop executable = %q, want %q", got, candidate)
	}
}

func TestClaudeDesktopDiscoveryRejectsUnofficialLinuxPathExecutable(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "claude-desktop")
	if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}

	result := claudedesktop.New().Discover(discovery.System{GOOS: "linux", Home: root, Path: root})
	if got := result.Executable(claudedesktop.ID); got != "" {
		t.Fatalf("Claude Desktop executable = %q, want unavailable on Linux", got)
	}
}
