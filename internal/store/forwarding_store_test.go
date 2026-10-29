package store

import (
	"aigw-cli/internal/domain"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Ported from the maintenance branch (aedfefcb): a damaged forwarding sidecar,
// current or backup, is refused at its load phase without changing the
// configuration or the inputs a recovery reads.
func TestStoreForwardingReadFailuresPreserveRecoveryInputs(t *testing.T) {
	cfg := validConfig()
	cfg.Normalize()
	store := NewStore(filepath.Join(t.TempDir(), "config.toml"), testAdmission())
	for _, endpoint := range []string{"http://127.0.0.1:8792/previous/v1", "http://127.0.0.1:8792/v1"} {
		if err := cfg.SetForwardingEndpoint(ClientCodex, endpoint); err != nil {
			t.Fatal(err)
		}
		if err := store.Save(cfg); err != nil {
			t.Fatal(err)
		}
	}
	accepted, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	for _, reader := range []struct {
		name, suffix string
		read         func(Store) (domain.Config, error)
	}{
		{"current", "", Store.Load},
		{"backup", ".bak", Store.LoadBackup},
	} {
		t.Run(reader.name, func(t *testing.T) {
			want, err := reader.read(store)
			if err != nil || want.Clients[ClientCodex].ForwardingEndpoint == "" {
				t.Fatalf("valid forwarding read: %v", err)
			}
			path := store.forwardingPath() + reader.suffix
			valid, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, invalid := range []struct {
				content string
				phase   LoadPhase
			}{
				{"[clients.codex", LoadPhaseParse},
				{"", LoadPhaseValidate},
				{strings.ReplaceAll(string(valid), "clients.codex", "clients.hermes"), LoadPhaseValidate},
			} {
				if err := os.WriteFile(path, []byte(invalid.content), 0o600); err != nil {
					t.Fatal(err)
				}
				before, err := store.CaptureSnapshot()
				if err != nil {
					t.Fatal(err)
				}
				_, err = reader.read(store)
				var loadError *LoadError
				if !errors.As(err, &loadError) || loadError.Phase != invalid.phase {
					t.Fatalf("forwarding refusal phase: %v; want %s", err, invalid.phase)
				}
				after, err := store.CaptureSnapshot()
				if err != nil || !before.equal(after) {
					t.Fatalf("forwarding refusal changed configuration or recovery inputs: %v", err)
				}
			}
			damaged, err := store.CaptureSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			if err := store.RestoreSnapshot(accepted, damaged); err != nil {
				t.Fatal(err)
			}
			if got, err := reader.read(store); err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("restored forwarding read: %v", err)
			}
		})
	}
}

// Ported from the maintenance branch (aedfefcb): a sidecar whose configuration
// is gone is refused at read, and restoring the configuration recovers both.
func TestStoreRejectsOrphanForwardingWithoutChangingRecoveryInputs(t *testing.T) {
	cfg := validConfig()
	cfg.Normalize()
	if err := cfg.SetForwardingEndpoint(ClientCodex, "http://127.0.0.1:8792/v1"); err != nil {
		t.Fatal(err)
	}
	store := NewStore(filepath.Join(t.TempDir(), "config.toml"), testAdmission())
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	accepted, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(store.Path()); err != nil {
		t.Fatal(err)
	}
	orphaned, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Load()
	var loadError *LoadError
	if !errors.As(err, &loadError) || loadError.Phase != LoadPhaseRead {
		t.Fatalf("orphan forwarding refusal: %v", err)
	}
	after, err := store.CaptureSnapshot()
	if err != nil || !after.equal(orphaned) {
		t.Fatalf("orphan forwarding refusal changed recovery inputs: %v", err)
	}
	if err := store.RestoreSnapshot(accepted, after); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Load(); err != nil || !reflect.DeepEqual(got, cfg) {
		t.Fatalf("restored canonical forwarding configuration: %v", err)
	}
}

// Ported from the maintenance branch (aedfefcb): forwarding is selected only for
// a valid destination of a client whose Route reaches an existing Account, and
// a refused selection changes nothing.
func TestForwardingRequiresAValidSelectedUpstream(t *testing.T) {
	for _, test := range []struct {
		name, endpoint, want string
	}{
		{"unselected", "http://127.0.0.1:8792/v1", "no Route selected"},
		{"invalid destination", "not-an-endpoint", "forwarding endpoint"},
		{"missing upstream", "http://127.0.0.1:8792/v1", "account"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Normalize()
			switch test.name {
			case "unselected":
				delete(cfg.Clients, ClientCodex)
			case "missing upstream":
				delete(cfg.Accounts, "backup")
			}
			before := cfg.Clone()
			if err := cfg.SetForwardingEndpoint(ClientCodex, test.endpoint); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("invalid forwarding selection: %v", err)
			}
			if !reflect.DeepEqual(cfg, before) {
				t.Fatal("rejected forwarding selection changed configuration")
			}
		})
	}
}
