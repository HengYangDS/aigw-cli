package discovery_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/discovery"
)

func TestDiscoverFindsClientsAndExistingCodexTargets(t *testing.T) {
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
	if got.ClaudeExecutable != wantClaude || got.CodexExecutable != wantCodex {
		t.Fatalf("executables = %#v", got)
	}
	if len(got.CodexTargets) != 1 || got.CodexTargets[0] != target {
		t.Fatalf("targets = %#v", got.CodexTargets)
	}
}

func TestDiscoverSkipsAIGWOwnedClaudeShim(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	goos, name := "linux", "claude"
	if runtime.GOOS == "windows" {
		goos, name = "windows", "claude.cmd"
	}
	if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n# AIGW managed Claude shim\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := (discovery.System{GOOS: goos, Home: home, Path: bin}).Discover()
	if got.ClaudeExecutable != "" {
		t.Fatalf("managed shim rediscovered as real Claude: %#v", got)
	}
}
