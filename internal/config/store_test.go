package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/config"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

func TestSaveLoadRoundTripAndSecurePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.toml")
	store := config.NewStore(path)
	want := domain.Config{
		Version: 1,
		Profiles: map[string]domain.Profile{"dmx": {
			Label: "DMXAPI", Endpoints: domain.Endpoints{Anthropic: "https://example.test"},
		}},
		Routes: domain.Routes{Default: "dmx", Overrides: map[string]string{}},
	}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o, want 600", got)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Profiles["dmx"].Endpoints.Anthropic != "https://example.test" || got.Routes.Default != "dmx" {
		t.Fatalf("round trip = %#v", got)
	}
}

func TestSaveRefusesInvalidConfigWithoutReplacingExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("sentinel"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := config.NewStore(path)
	if err := store.Save(domain.Config{Version: 1}); err == nil {
		t.Fatal("expected validation error")
	}
	got, _ := os.ReadFile(path)
	if string(got) != "sentinel" {
		t.Fatalf("existing config was replaced: %q", got)
	}
}
