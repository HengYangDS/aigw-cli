package discovery

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveSurfacePathHandlesMissingAndSymbolicPaths(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing", "configuration.toml")
	wantMissing, err := filepath.Abs(missing)
	if err != nil {
		t.Fatal(err)
	}
	if got := resolveSurfacePath(missing); got != wantMissing {
		t.Fatalf("resolveSurfacePath(missing) = %q, want %q", got, wantMissing)
	}

	target := filepath.Join(dir, "target.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "linked.toml")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	wantTarget, err := filepath.Abs(resolvedTarget)
	if err != nil {
		t.Fatal(err)
	}
	if got := resolveSurfacePath(link); got != wantTarget {
		t.Fatalf("resolveSurfacePath(link) = %q, want %q", got, wantTarget)
	}
}

func TestFindUsesWindowsCommandSuffixWithoutExecutableBit(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "claude.cmd")
	if err := os.WriteFile(target, []byte("@echo off\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(target)
	if err != nil {
		t.Fatal(err)
	}
	if got := (System{GOOS: "windows", Path: dir}).find("claude", false); got != want {
		t.Fatalf("find(claude) = %q, want %q", got, want)
	}
}

func TestFindSkipsManagedClaudeOnlyWhenRequested(t *testing.T) {
	dir := t.TempDir()
	// A real Windows filesystem never reports a Unix executable bit
	// (https://github.com/golang/go/issues/41809), so the non-Windows
	// exec-bit branch of find cannot be exercised there regardless of the
	// permission requested at creation. Exercise the Windows lookup
	// semantics instead: its plain-name fallback still reaches this file
	// and bypasses the exec-bit check entirely, which is the only find
	// path a real native-Windows host can ever take in production.
	goos, name := "linux", "claude"
	if runtime.GOOS == "windows" {
		goos, name = "windows", "claude.cmd"
	}
	target := filepath.Join(dir, name)
	if err := os.WriteFile(target, []byte("#!/bin/sh\n# AIGW managed Claude launcher\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	system := System{GOOS: goos, Path: dir}
	if got := system.find("claude", true); got != "" {
		t.Fatalf("find(managed Claude, skip=true) = %q", got)
	}
	want, err := filepath.Abs(target)
	if err != nil {
		t.Fatal(err)
	}
	if got := system.find("claude", false); got != want {
		t.Fatalf("find(managed Claude, skip=false) = %q, want %q", got, want)
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
			if got := (System{GOOS: "linux", Path: dir}).find("codex", false); got != "" {
				t.Fatalf("find(codex) = %q", got)
			}
		})
	}
}
