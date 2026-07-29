package discovery_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/discovery"
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
	if got.ClaudeExecutable != wantClaude || got.CodexExecutable != wantCodex {
		t.Fatalf("executables = %#v", got)
	}
	if targets := got.AutoManagedCodexTargets(); len(targets) != 1 || targets[0] != target {
		t.Fatalf("targets = %#v", targets)
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

func TestDiscoverClassifiesJetBrainsSurfacesWithoutExecutingJunie(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "codex"), []byte("#!/bin/sh\nexit 99\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	standalone := filepath.Join(home, ".codex", "config.toml")
	pycharm := filepath.Join(home, "Library", "Caches", "JetBrains", "PyCharm2026.1", "aia", "codex", "config.toml")
	air := filepath.Join(home, "Library", "Application Support", "JetBrains", "Air", ".codex", "config.toml")
	for path, body := range map[string]string{
		standalone: "model_provider = \"native\"\n",
		pycharm:    "model_provider = \"jetbrains\"\n",
		air:        "model_provider = \"jetbrains\"\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	sentinel := filepath.Join(home, "junie-executed")
	junie := filepath.Join(home, ".local", "bin", "junie")
	if err := os.MkdirAll(filepath.Dir(junie), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(junie, []byte("#!/bin/sh\ntouch "+sentinel+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := (discovery.System{GOOS: "darwin", Home: home, Path: bin}).Discover()
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatalf("Junie was executed during discovery: %v", err)
	}
	wantCodex := filepath.Join(bin, "codex")
	if runtime.GOOS == "windows" {
		// A real Windows filesystem never reports a Unix executable bit
		// (https://github.com/golang/go/issues/41809), so a GOOS-darwin
		// System can never classify this file as executable on this host;
		// that combination never occurs in production, since Current()
		// always sets GOOS to the real runtime.GOOS. The Windows-target
		// executable lookup path is covered separately by
		// TestDiscoverFindsClientsAndAutoManagedCodexTargets.
		wantCodex = ""
	}
	if got.CodexExecutable != wantCodex {
		t.Fatalf("CodexExecutable = %q, want %q", got.CodexExecutable, wantCodex)
	}
	if targets := got.AutoManagedCodexTargets(); len(targets) != 1 || targets[0] != standalone {
		t.Fatalf("AutoManagedCodexTargets() = %#v", targets)
	}
	airSurface, ok := got.Surface(discovery.SurfaceAirCodex)
	if !ok || airSurface.Authority != discovery.AuthorityJetBrainsAI || !airSurface.ManualFallbackAllowed || airSurface.AutoManaged || !airSurface.Present {
		t.Fatalf("Air surface = %#v", airSurface)
	}
	pycharmSurface, ok := got.Surface(discovery.SurfacePyCharmCodex)
	if !ok || pycharmSurface.Authority != discovery.AuthorityJetBrainsAI || pycharmSurface.AutoManaged || !pycharmSurface.Present {
		t.Fatalf("PyCharm surface = %#v", pycharmSurface)
	}
	junieSurface, ok := got.Surface(discovery.SurfaceJunieCLI)
	if !ok || junieSurface.Authority != discovery.AuthorityJetBrainsAI || junieSurface.ConfigPath != "" || !junieSurface.Present {
		t.Fatalf("Junie surface = %#v", junieSurface)
	}
	if surface, ok := got.SurfaceForConfigPath(air); !ok || surface.ID != discovery.SurfaceAirCodex {
		t.Fatalf("SurfaceForConfigPath(Air) = %#v, %v", surface, ok)
	}
	if surface, ok := got.SurfaceForExecutablePath(junie); !ok || surface.ID != discovery.SurfaceJunieCLI {
		t.Fatalf("SurfaceForExecutablePath(Junie) = %#v, %v", surface, ok)
	}
}
