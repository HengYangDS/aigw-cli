package configuration

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/transaction"
)

func TestUnreadableCapabilityComponentStopsOnlyFullReads(t *testing.T) {
	for _, suffix := range []string{"", ".bak"} {
		t.Run("component"+suffix, func(t *testing.T) {
			store := NewStore(filepath.Join(t.TempDir(), "config.toml"), testAdmission())
			cfg := toolSearchConfig()
			if err := store.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := store.SaveVerifiedCheckpoint(t.Context(), cfg, []string{ClientClaude}); err != nil {
				t.Fatal(err)
			}
			state, err := store.CaptureVerifiedBackupState()
			if err != nil {
				t.Fatal(err)
			}
			if err := store.ConvergeVerifiedBackup(state.Snapshot); err != nil {
				t.Fatal(err)
			}
			path := capabilityComponentPath(store) + suffix
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
			if _, err := store.CaptureSnapshot(); err == nil {
				t.Fatal("snapshot capture ignored an unreadable capability component")
			}
			if _, err := store.LoadCredentialConfig(); err != nil {
				t.Fatalf("credential configuration depends on the capability component: %v", err)
			}
			_, loadErr := store.Load()
			_, backupErr := store.LoadBackup()
			_, checkpointErr := store.LoadVerifiedCheckpoint()
			saveErr := store.SaveVerifiedCheckpoint(t.Context(), cfg, []string{ClientClaude})
			current := suffix == ""
			if (loadErr != nil) != current || (checkpointErr != nil) != current || (saveErr != nil) != current || (backupErr != nil) == current {
				t.Fatalf("unreadable %q: load=%v checkpoint=%v save=%v backup=%v", suffix, loadErr, checkpointErr, saveErr, backupErr)
			}
		})
	}
}

func TestCapabilityComponentWriteFailureRestoresOwnedFiles(t *testing.T) {
	for _, suffix := range []string{"", ".bak"} {
		t.Run("component"+suffix, func(t *testing.T) {
			store := NewStore(filepath.Join(t.TempDir(), "config.toml"), testAdmission())
			cfg := toolSearchConfig()
			if err := store.Save(cfg); err != nil {
				t.Fatal(err)
			}
			before, err := store.CaptureSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			failure := failCapabilityWrites(t, capabilityComponentPath(store)+suffix)
			changed := cfg.Clone()
			account := changed.Accounts["ucloud"]
			account.Label = "UCloud Updated"
			changed.Accounts["ucloud"] = account
			if _, err := store.Commit(before, changed); !errors.Is(err, failure) {
				t.Fatalf("failed capability write = %v", err)
			}
			if after, err := store.CaptureSnapshot(); err != nil || !before.equal(after) {
				t.Fatalf("failed capability write did not restore every owned file: %v", err)
			}
		})
	}
}

func TestVerifiedBackupConvergenceGuardsTheCapabilityComponent(t *testing.T) {
	for _, change := range []string{"preimage", "write"} {
		t.Run(change, func(t *testing.T) {
			store := NewStore(filepath.Join(t.TempDir(), "config.toml"), testAdmission())
			if err := store.Save(toolSearchConfig()); err != nil {
				t.Fatal(err)
			}
			state, err := store.CaptureVerifiedBackupState()
			if err != nil {
				t.Fatal(err)
			}
			want := "route capability preimage changed"
			if change == "preimage" {
				if err := os.WriteFile(capabilityComponentPath(store), []byte("# newer operator content\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			} else {
				want = failCapabilityWrites(t, capabilityComponentPath(store)+".bak").Error()
			}
			if err := store.ConvergeVerifiedBackup(state.Snapshot); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("backup convergence error = %v, want %q", err, want)
			}
			if _, err := os.Stat(capabilityComponentPath(store) + ".bak"); !os.IsNotExist(err) {
				t.Fatalf("refused convergence left a capability backup: %v", err)
			}
		})
	}
}

func TestRestoreSnapshotReportsCapabilityComponentConflicts(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "config.toml"), testAdmission())
	cfg := toolSearchConfig()
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	changed := cfg.Clone()
	route := changed.Routes["claude"]
	route.Interfaces = map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {domain.CapabilityText}}
	changed.Routes["claude"] = route
	after, err := store.Commit(before, changed)
	if err != nil {
		t.Fatal(err)
	}
	newer := []byte("# newer operator content\n")
	for _, path := range []string{capabilityComponentPath(store), capabilityComponentPath(store) + ".bak"} {
		if err := os.WriteFile(path, newer, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	err = store.RestoreSnapshot(before, after)
	for _, want := range []string{"restore route capability snapshot", "restore route capability backup"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("restore error = %v, want %q", err, want)
		}
	}
	for _, path := range []string{capabilityComponentPath(store), capabilityComponentPath(store) + ".bak"} {
		if got := readStoreFile(t, path); string(got) != string(newer) {
			t.Fatalf("restore replaced newer content at %s", filepath.Base(path))
		}
	}
}

// failCapabilityWrites makes every write or removal of path fail until the
// test ends, and returns the injected failure.
func failCapabilityWrites(t *testing.T, path string) error {
	t.Helper()
	failure := errors.New("capability component mutation failed")
	originalWrite := writeConfigurationFileIfUnchanged
	originalRemove := removeConfigurationFileIfUnchanged
	t.Cleanup(func() {
		writeConfigurationFileIfUnchanged = originalWrite
		removeConfigurationFileIfUnchanged = originalRemove
	})
	writeConfigurationFileIfUnchanged = func(target string, expected transaction.FileSnapshot, data []byte, mode os.FileMode) (transaction.FileSnapshot, error) {
		if target == path {
			return transaction.FileSnapshot{}, failure
		}
		return originalWrite(target, expected, data, mode)
	}
	removeConfigurationFileIfUnchanged = func(target string, expected transaction.FileSnapshot) (transaction.FileSnapshot, error) {
		if target == path {
			return transaction.FileSnapshot{}, failure
		}
		return originalRemove(target, expected)
	}
	return failure
}
