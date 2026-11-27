package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

// A surface owns its configuration file in every spelling that names the
// file: through a link to it, and, before the file exists, through a link to
// its folder. Another file, and an empty path, are not the surface's.
func TestASurfaceOwnsItsFileInEverySpelling(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	if err := os.Mkdir(home, 0o700); err != nil {
		t.Fatal(err)
	}
	missing := Result{Surfaces: []Surface{{ID: "missing", ConfigPath: filepath.Join(home, "missing.toml")}}}
	linkedHome := filepath.Join(dir, "linked")
	target := filepath.Join(home, "target.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "linked.toml")
	if err := os.Symlink(home, linkedHome); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if surface, ok := missing.SurfaceForConfigPath(filepath.Join(linkedHome, "missing.toml")); !ok || surface.ID != "missing" {
		t.Fatalf("a missing file through a link to its folder = %#v, %t", surface, ok)
	}
	existing := Result{Surfaces: []Surface{{ID: "target", ConfigPath: target}}}
	if surface, ok := existing.SurfaceForConfigPath(link); !ok || surface.ID != "target" {
		t.Fatalf("a link to the file = %#v, %t", surface, ok)
	}
	for _, other := range []string{filepath.Join(home, "other.toml"), ""} {
		if surface, ok := existing.SurfaceForConfigPath(other); ok {
			t.Fatalf("%q was taken for the surface's file: %#v", other, surface)
		}
	}
}

func TestFindTreatsClaudeAsTheNativeClientExecutable(t *testing.T) {
	dir := t.TempDir()
	goos, name := "linux", "claude"
	target := filepath.Join(dir, name)
	if err := os.WriteFile(target, []byte("#!/bin/sh\nnative Claude fixture\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	system := System{GOOS: goos, Path: dir}
	want, err := filepath.Abs(target)
	if err != nil {
		t.Fatal(err)
	}
	if got := system.Executable("claude"); got != want {
		t.Fatalf("Executable(claude) = %q, want %q", got, want)
	}
}

func TestFindRejectsDirectoriesAndNonExecutableUnixFiles(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T, string)
	}{
		{
			name: "directory",
			setup: func(t *testing.T, path string) {
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "non-executable file",
			setup: func(t *testing.T, path string) {
				if err := os.WriteFile(path, []byte("not executable"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(t, filepath.Join(dir, "codex"))
			if got := (System{GOOS: "linux", Path: dir}).Executable("codex"); got != "" {
				t.Fatalf("Executable(codex) = %q", got)
			}
		})
	}
}

func TestExecutableAvailableReportsEmptyMissingAndInspectionFailure(t *testing.T) {
	if available, err := ExecutableAvailable(""); available || err != nil {
		t.Fatalf("ExecutableAvailable(empty) = %v, %v", available, err)
	}
	if available, err := ExecutableAvailable(filepath.Join(t.TempDir(), "missing")); available || err != nil {
		t.Fatalf("ExecutableAvailable(missing) = %v, %v", available, err)
	}
	loop := filepath.Join(t.TempDir(), "loop")
	if err := os.Symlink(loop, loop); err != nil {
		t.Skipf("symbolic link unavailable: %v", err)
	}
	if available, err := ExecutableAvailable(loop); available || err == nil {
		t.Fatalf("ExecutableAvailable(loop) = %v, %v", available, err)
	}
}
