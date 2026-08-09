package discovery_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"aigw-cli/internal/discovery"
)

func TestDiscoverFindsClientsAndAutoManagedCodexTargets(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	goos := "linux"
	names := []string{"claude", "codex"}
	if runtime.GOOS == "windows" {
		goos = "windows"
		names = []string{"claude.exe", "codex.exe"}
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	target := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	d := discovery.System{GOOS: goos, Home: home, Path: bin}
	got := d.Discover()
	wantClaude, wantCodex := filepath.Join(bin, names[0]), filepath.Join(bin, names[1])
	if got.Executable("claude") != wantClaude || got.Executable("codex") != wantCodex {
		t.Fatalf("executables = %#v", got)
	}
	if targets := got.AutoManagedCodexTargets(); len(targets) != 1 || targets[0] != target {
		t.Fatalf("targets = %#v", targets)
	}
}

func TestDiscoverReturnsClaudeExecutableWithoutPrivateMarkers(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	goos, name := "linux", "claude"
	if runtime.GOOS == "windows" {
		goos, name = "windows", "claude.cmd"
	}
	if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\nnative Claude fixture\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := (discovery.System{GOOS: goos, Home: home, Path: bin}).Discover()
	want, err := filepath.Abs(filepath.Join(bin, name))
	if err != nil {
		t.Fatal(err)
	}
	if got.Executable("claude") != want {
		t.Fatalf("Claude executable = %q, want %q", got.Executable("claude"), want)
	}
}
