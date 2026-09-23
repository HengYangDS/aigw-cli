package configuration

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublishedMigrationRejectsAmbiguousOrInvalidState(t *testing.T) {
	tests := []struct {
		name, original, replacement, reason string
	}{
		{"unpublished client", `client = "codex"`, `client = "hermes"`, "no admitted client protocol"},
		{"missing account", "[profiles.codex]\nlabel = \"Codex\"\naccount = \"gateway\"", "[profiles.codex]\nlabel = \"Codex\"\naccount = \"missing\"", "references unknown account"},
		{"missing model", `model = "gpt-test"`, `model = ""`, "has no model"},
		{"missing recommendation target", "[recommended_routes]\ncodex = \"codex\"", "[recommended_routes]\ncodex = \"missing\"", "published recommendation"},
		{"missing selected target", "[routes]\nclaude = \"claude\"", "[routes]\nclaude = \"missing\"", "published selection"},
		{"unowned client options", "[adapters.claude]", "[profiles.orphan]\naccount = \"gateway\"\nclient = \"codex\"\nmodel = \"gpt-orphan\"\nmodel_provider = \"amazon-bedrock\"\n\n[adapters.claude]", "client-specific options without a selected or recommended owner"},
		{"unknown published field", "version = 3\n", "version = 3\nunknown = true\n", "strict mode"},
		{"incompatible selected client", "[routes]\nclaude = \"claude\"", "[routes]\nclaude = \"codex\"", "migrated published configuration is invalid"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store, predecessor, _, _, _ := publishedV010Store(t)
			if count := strings.Count(string(predecessor), test.original); count != 1 {
				t.Fatalf("published fixture has %d matches for %q", count, test.original)
			}
			invalid := []byte(strings.Replace(string(predecessor), test.original, test.replacement, 1))
			if err := os.WriteFile(store.Path(), invalid, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := store.PrepareMigration(false); err == nil || !strings.Contains(err.Error(), test.reason) {
				t.Fatalf("unsafe published migration was admitted: %v", err)
			}
			if actual, err := os.ReadFile(store.Path()); err != nil || !bytes.Equal(actual, invalid) {
				t.Fatalf("rejected migration changed configuration: %v", err)
			}
			if _, err := os.Stat(store.Path() + ".bak"); !os.IsNotExist(err) {
				t.Fatalf("rejected migration created backup: %v", err)
			}
		})
	}
}

func TestMigrationPreviewRejectsAbsentOrUnsupportedConfiguration(t *testing.T) {
	for _, test := range []struct {
		name, data, reason string
	}{
		{"absent", "", "configuration is unavailable"},
		{"malformed TOML", "version = [\n", "parse config"},
		{"unsupported schema", "version = 4\n", "unsupported config version"},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := NewStore(filepath.Join(t.TempDir(), "config.toml"))
			if test.data != "" {
				if err := os.WriteFile(store.Path(), []byte(test.data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := store.PrepareMigration(false); err == nil || !strings.Contains(err.Error(), test.reason) {
				t.Fatalf("invalid configuration was admitted: %v", err)
			}
		})
	}
}

func TestMigrationRollbackRejectsUnsafePreimages(t *testing.T) {
	for _, test := range []struct {
		name, current, backup, reason string
		removeBackup                  bool
	}{
		{"missing backup", "", "", "no retained predecessor", true},
		{"invalid current configuration", "version = 4\n", "", "requires a valid current configuration", false},
		{"malformed predecessor", "", "version = [\n", "rollback is unavailable", false},
		{"unsupported predecessor", "", "version = 4\n", "rollback is unavailable", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			store, _, _ := appliedPublishedV010Store(t)
			if test.removeBackup {
				if err := os.Remove(store.Path() + ".bak"); err != nil {
					t.Fatal(err)
				}
			}
			if test.current != "" {
				if err := os.WriteFile(store.Path(), []byte(test.current), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if test.backup != "" {
				if err := os.WriteFile(store.Path()+".bak", []byte(test.backup), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := store.PrepareMigration(true); err == nil || !strings.Contains(err.Error(), test.reason) {
				t.Fatalf("unsafe rollback was admitted: %v", err)
			}
		})
	}
}

func TestMigrationPlanRejectsAnotherConfigurationStore(t *testing.T) {
	store, predecessor, _, _, _ := publishedV010Store(t)
	plan, err := store.PrepareMigration(false)
	if err != nil {
		t.Fatal(err)
	}
	other := NewStore(filepath.Join(t.TempDir(), "config.toml"))
	if err := other.ApplyMigration(plan); err == nil || !strings.Contains(err.Error(), "different configuration store") {
		t.Fatalf("foreign migration plan was admitted: %v", err)
	}
	if actual, err := os.ReadFile(store.Path()); err != nil || !bytes.Equal(actual, predecessor) {
		t.Fatalf("foreign plan changed original configuration: %v", err)
	}
	if _, err := os.Stat(other.Path()); !os.IsNotExist(err) {
		t.Fatalf("foreign plan created another configuration: %v", err)
	}
}

func TestImmediatePredecessorMigrationRejectsInvalidProfiles(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*legacyConfig)
		reason string
	}{
		{"missing model", func(config *legacyConfig) {
			profile := config.Profiles[ClientCodex]
			profile.Model = ""
			config.Profiles[ClientCodex] = profile
		}, "has no model"},
		{"missing account", func(config *legacyConfig) {
			profile := config.Profiles[ClientCodex]
			profile.Account = "missing"
			config.Profiles[ClientCodex] = profile
		}, "references unknown account"},
		{"incompatible binding", func(config *legacyConfig) {
			binding := config.Clients[ClientCodex]
			binding.Protocol = ProtocolAnthropic
			config.Clients[ClientCodex] = binding
		}, "migrated configuration is invalid"},
	} {
		t.Run(test.name, func(t *testing.T) {
			store, original := legacyStoreWith(t, test.mutate)
			if _, err := store.PrepareMigration(false); err == nil || !strings.Contains(err.Error(), test.reason) {
				t.Fatalf("invalid predecessor was admitted: %v", err)
			}
			if actual, err := os.ReadFile(store.Path()); err != nil || !bytes.Equal(actual, original) {
				t.Fatalf("rejected predecessor changed configuration: %v", err)
			}
		})
	}
}
