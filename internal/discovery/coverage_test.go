package discovery_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/discovery"
)

func TestCurrentReflectsProcessPlatformAndPath(t *testing.T) {
	wantPath := filepath.Join(t.TempDir(), "bin")
	t.Setenv("PATH", wantPath)

	system := discovery.Current()
	if system.GOOS != runtime.GOOS || system.Path != wantPath {
		t.Fatalf("Current() = %#v, want GOOS %q and PATH %q", system, runtime.GOOS, wantPath)
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

func TestLinuxDiscoveryContainsOnlyStandaloneSurface(t *testing.T) {
	home := t.TempDir()
	result := (discovery.System{GOOS: "linux", Home: home}).Discover()
	if len(result.Surfaces) != 1 {
		t.Fatalf("Linux surfaces = %#v", result.Surfaces)
	}
	surface := result.Surfaces[0]
	wantConfig := filepath.Join(home, ".codex", "config.toml")
	if surface.ID != discovery.SurfaceCodexCLIStandalone || surface.ConfigPath != wantConfig || surface.Present {
		t.Fatalf("standalone surface = %#v", surface)
	}
}
