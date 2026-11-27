package store

import (
	"aigw-cli/internal/domain"
	"path/filepath"
	"testing"
)

// A snapshot matches only the configuration that it holds: an absent file
// the empty configuration, and a saved file the configuration that was saved.
func TestSnapshotMatchesOnlyItsOwnConfiguration(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "aigw.toml"), testAdmission())
	absent, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if matches, err := absent.MatchesConfiguration(domain.NewConfig(store.Admission()), nil); err != nil || !matches {
		t.Fatalf("an absent file does not match the empty configuration: %v", err)
	}
	cfg := validConfig()
	if matches, err := absent.MatchesConfiguration(cfg, nil); err != nil || matches {
		t.Fatalf("an absent file matches a configuration: %v", err)
	}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	saved, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if matches, err := saved.MatchesConfiguration(cfg, nil); err != nil || !matches {
		t.Fatalf("a saved file does not match its configuration: %v", err)
	}
	changed := cfg.Clone()
	provider := changed.Providers["dmx"]
	provider.Label = "Changed"
	changed.Providers["dmx"] = provider
	if matches, err := saved.MatchesConfiguration(changed, nil); err != nil || matches {
		t.Fatalf("a saved file matches a changed configuration: %v", err)
	}
	saved.Config.Data = []byte("version = 9\n[[providers]\n")
	if _, err := saved.MatchesConfiguration(cfg, nil); err == nil {
		t.Fatal("a file that does not parse matched")
	}
}

// A snapshot decodes the configuration that it holds, says that an absent
// file holds none, and refuses one that does not parse.
func TestSnapshotDecodesItsOwnConfiguration(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "aigw.toml"), testAdmission())
	absent, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if _, exists, err := absent.Configuration(store.Admission()); err != nil || exists {
		t.Fatalf("an absent file holds a configuration: exists %t, %v", exists, err)
	}
	cfg := validConfig()
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	saved, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	decoded, exists, err := saved.Configuration(store.Admission())
	if err != nil || !exists {
		t.Fatalf("a saved file holds no configuration: exists %t, %v", exists, err)
	}
	if same, err := configurationsEqual(decoded, cfg); err != nil || !same {
		t.Fatalf("a saved file decodes to another configuration: %v", err)
	}
	saved.Config.Data = []byte("version = 9\n[[providers]\n")
	if _, exists, err := saved.Configuration(store.Admission()); err == nil || exists {
		t.Fatalf("a file that does not parse decoded: exists %t", exists)
	}
}
