package discovery_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"aigw-cli/internal/client"
	"aigw-cli/internal/discovery"
	surfacepkg "aigw-cli/internal/surface"
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
	d := client.NewDiscoverer(client.DefaultRegistry(), discovery.System{GOOS: goos, Home: home, Path: bin})
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
	got := client.NewDiscoverer(client.DefaultRegistry(), discovery.System{GOOS: goos, Home: home, Path: bin}).Discover()
	want, err := filepath.Abs(filepath.Join(bin, name))
	if err != nil {
		t.Fatal(err)
	}
	if got.Executable("claude") != want {
		t.Fatalf("Claude executable = %q, want %q", got.Executable("claude"), want)
	}
}

func TestCurrentReflectsProcessPlatformAndPath(t *testing.T) {
	wantPath := filepath.Join(t.TempDir(), "bin")
	t.Setenv("PATH", wantPath)

	system := discovery.Current()
	if system.GOOS != runtime.GOOS || system.Path != wantPath {
		t.Fatalf("Current() = %#v, want GOOS %q and PATH %q", system, runtime.GOOS, wantPath)
	}
}

func TestResultIndexesExecutablesByAdmittedClient(t *testing.T) {
	result := discovery.Result{Executables: map[string]string{
		"claude": "/portable/claude",
		"codex":  "/portable/codex",
	}}
	if got := result.Executable("claude"); got != "/portable/claude" {
		t.Fatalf("Claude executable = %q", got)
	}
	if got := result.Executable("codex"); got != "/portable/codex" {
		t.Fatalf("Codex executable = %q", got)
	}
	if got := result.Executable("unknown"); got != "" {
		t.Fatalf("unknown executable = %q", got)
	}
}

func TestResultRejectsUnknownSurfacePaths(t *testing.T) {
	result := discovery.Result{Surfaces: []discovery.Surface{{ID: "known", ConfigPath: "/known/config", Executable: "/known/executable"}}}
	if surface, ok := result.Surface("unknown"); ok {
		t.Fatalf("Surface(unknown) = %#v, true", surface)
	}
	if surface, ok := result.SurfaceForConfigPath("/unknown/config"); ok {
		t.Fatalf("SurfaceForConfigPath(unknown) = %#v, true", surface)
	}
	if surface, ok := result.SurfaceForExecutablePath("/unknown/executable"); ok {
		t.Fatalf("SurfaceForExecutablePath(unknown) = %#v, true", surface)
	}
}

func TestResultFindsKnownSurfaceByEachIdentity(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "configuration.toml")
	executablePath := filepath.Join(dir, "codex")
	known := discovery.Surface{ID: "known", ConfigPath: configPath, Executable: executablePath}
	result := discovery.Result{Surfaces: []discovery.Surface{known}}

	lookups := []struct {
		name string
		find func() (discovery.Surface, bool)
	}{
		{name: "ID", find: func() (discovery.Surface, bool) { return result.Surface("known") }},
		{name: "config path", find: func() (discovery.Surface, bool) { return result.SurfaceForConfigPath(configPath) }},
		{name: "executable path", find: func() (discovery.Surface, bool) { return result.SurfaceForExecutablePath(executablePath) }},
	}
	for _, lookup := range lookups {
		t.Run(lookup.name, func(t *testing.T) {
			got, ok := lookup.find()
			if !ok || got != known {
				t.Fatalf("lookup = %#v, %t; want %#v, true", got, ok, known)
			}
		})
	}

	if _, ok := result.SurfaceForConfigPath(""); ok {
		t.Fatal("blank config path unexpectedly matched")
	}
}

func TestLinuxDiscoveryContainsOnlyDefaultCodexHome(t *testing.T) {
	home := t.TempDir()
	result := client.NewDiscoverer(client.DefaultRegistry(), discovery.System{GOOS: "linux", Home: home}).Discover()
	if len(result.Surfaces) != 1 {
		t.Fatalf("Linux surfaces = %#v", result.Surfaces)
	}
	surface := result.Surfaces[0]
	wantConfig := filepath.Join(home, ".codex", "config.toml")
	if surface.ID != string(surfacepkg.CodexHomeDefault) || surface.ConfigPath != wantConfig || surface.Present {
		t.Fatalf("default Codex Home = %#v", surface)
	}
	if targets := result.AutoManagedCodexTargets(); len(targets) != 1 || targets[0] != wantConfig {
		t.Fatalf("auto-managed targets = %#v, want absent but creatable default %q", targets, wantConfig)
	}
}
