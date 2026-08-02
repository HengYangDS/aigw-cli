package cli

import (
	"aigw-cli/internal/cli/adapter"
	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	surfaceidentity "aigw-cli/internal/surface"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandBoundaryTargetAndNamingBranches(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "codex")
	configPath := filepath.Join(t.TempDir(), "configuration.toml")
	discovered := discovery.Result{Surfaces: []discovery.Surface{
		{ID: string(surfaceidentity.CodexHomeDefault), Executable: executable},
		{ID: "future-surface", ConfigPath: configPath},
	}}
	if err := adapter.ValidateCodexTarget(discovered, executable); err == nil || !strings.Contains(err.Error(), "executable") {
		t.Fatalf("executable error = %v", err)
	}
	if err := adapter.ValidateCodexTarget(discovered, configPath); err == nil || !strings.Contains(err.Error(), "future-surface") {
		t.Fatalf("surface error = %v", err)
	}
	if names := configuration.ManifestAccountNames(configuration.Manifest{Profiles: map[string]configuration.Profile{"implicit": {}}}); len(names) != 1 || names[0] != "implicit" {
		t.Fatalf("account names = %#v", names)
	}
	if got := invocation.Title(""); got != "" {
		t.Fatalf("empty title = %q", got)
	}
}
