package discovery_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"aigw-cli/internal/discovery"
	surfacepkg "aigw-cli/internal/surface"
)

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
	result := (discovery.System{GOOS: "linux", Home: home}).Discover()
	if len(result.Surfaces) != 1 {
		t.Fatalf("Linux surfaces = %#v", result.Surfaces)
	}
	surface := result.Surfaces[0]
	wantConfig := filepath.Join(home, ".codex", "config.toml")
	if surface.ID != string(surfacepkg.CodexHomeDefault) || surface.ConfigPath != wantConfig || surface.Present {
		t.Fatalf("default Codex Home = %#v", surface)
	}
}
