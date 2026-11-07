package store

import (
	"os"
	"path/filepath"
	"testing"

	"aigw-cli/internal/domain"
)

// TestConfigurationsWithoutModelsRoundTrip shows that the store saves and
// loads a configuration whose providers have no models, and one without
// anything, through both of its readers, and that the credential reader's
// view of them selects nothing.
func TestConfigurationsWithoutModelsRoundTrip(t *testing.T) {
	providersOnly := admitted(domain.Config{Version: domain.ConfigVersion, Providers: map[string]domain.Provider{
		"team": {Label: "Team", Endpoints: domain.Endpoints{Anthropic: "https://team.test"}},
	}})
	for name, cfg := range map[string]domain.Config{
		"providers without models": providersOnly,
		"nothing":                  admitted(domain.Config{Version: domain.ConfigVersion}),
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			store := NewStore(path, testAdmission())
			if err := store.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("no configuration file was written: %v", err)
			}
			loaded, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			credential, err := store.LoadCredentialConfig()
			if err != nil {
				t.Fatal(err)
			}
			for _, got := range []domain.Config{loaded, credential} {
				if len(got.Providers) != len(cfg.Providers) || len(got.Models) != 0 || len(got.EnabledClientIDs()) != 0 {
					t.Fatalf("loaded configuration = %+v", got)
				}
			}
			// Saving what was loaded changes nothing.
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.Save(loaded); err != nil {
				t.Fatal(err)
			}
			if after, err := os.ReadFile(path); err != nil || string(after) != string(before) {
				t.Fatalf("a second save changed the file:\n%s\n%s", before, after)
			}
		})
	}
}
