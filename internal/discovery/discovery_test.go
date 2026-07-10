package discovery_test

import (
	"os"
	"path/filepath"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/discovery"
)

func TestDiscoverFindsClientsAndExistingCodexTargets(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"claude", "codex"} {
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
	d := discovery.System{GOOS: "linux", Home: home, Path: bin}
	got := d.Discover()
	if got.ClaudeExecutable != filepath.Join(bin, "claude") || got.CodexExecutable != filepath.Join(bin, "codex") {
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
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\n# AIGW managed Claude shim\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := (discovery.System{GOOS: "linux", Home: home, Path: bin}).Discover()
	if got.ClaudeExecutable != "" {
		t.Fatalf("managed shim rediscovered as real Claude: %#v", got)
	}
}
