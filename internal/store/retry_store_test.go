package store

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
)

func TestRollbackConfigurationPreservesRawBytesWithoutRetryComponent(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "aigw.toml"), testAdmission())
	original := []byte("version = 3\n# The retained reader owns schema admission.\n")
	if err := os.WriteFile(store.Path(), original, 0o600); err != nil {
		t.Fatal(err)
	}
	admission, err := store.RollbackConfiguration()
	if err != nil || !bytes.Equal(admission, original) {
		t.Fatalf("configuration without retry policy did not reach the retained reader unchanged: %q (%v)", admission, err)
	}
	retained, err := os.ReadFile(store.Path())
	if err != nil || !bytes.Equal(retained, original) {
		t.Fatalf("rollback admission changed persistent bytes: %v", err)
	}
}

func TestRetryCheckpointDoesNotAdoptUnverifiedPolicy(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "aigw.toml"), testAdmission())
	cfg := validConfig()
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(t.Context(), cfg, []string{ClientCodex}); err != nil {
		t.Fatal(err)
	}
	zero := uint64(0)
	binding := cfg.Clients[ClientCodex]
	binding.RetryPolicy = &domain.RetryPolicy{RequestMaxRetries: &zero}
	cfg.Clients[ClientCodex] = binding
	data, err := encodeRetryPolicies(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.retriesPath(), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CaptureVerifiedBackupState(); err == nil {
		t.Fatal("unverified retry policy was accepted as the checkpoint")
	}
	checkpoint, err := store.LoadVerifiedCheckpoint()
	if err != nil || checkpoint.Config.Clients[ClientCodex].RetryPolicy != nil {
		t.Fatalf("checkpoint adopted current retry policy: %v", err)
	}
}

func TestRetryPolicyCommitCompensatesLaterFailure(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "aigw.toml"), testAdmission())
	cfg := validConfig()
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(t.Context(), cfg, []string{ClientCodex}); err != nil {
		t.Fatal(err)
	}
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	zero := uint64(0)
	binding := cfg.Clients[ClientCodex]
	binding.RetryPolicy = &domain.RetryPolicy{RequestMaxRetries: &zero}
	cfg.Clients[ClientCodex] = binding
	remove := removeConfigurationFileIfUnchanged
	t.Cleanup(func() { removeConfigurationFileIfUnchanged = remove })
	failure := errors.New("injected checkpoint invalidation failure")
	removeConfigurationFileIfUnchanged = func(path string, preimage platform.FileSnapshot) (platform.FileSnapshot, error) {
		if path == store.Path()+".verified.json" {
			return platform.FileSnapshot{}, failure
		}
		return remove(path, preimage)
	}
	if _, err := store.Commit(before, cfg); !errors.Is(err, failure) {
		t.Fatalf("injected failure was lost: %v", err)
	}
	after, err := store.CaptureSnapshot()
	if err != nil || !before.equal(after) {
		t.Fatalf("failed policy commit changed preimages: %v", err)
	}
}

func TestRetryPolicyBackupAdmissionAndNativeReset(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "aigw.toml"), testAdmission())
	cfg := validConfig()
	zero := uint64(0)
	binding := cfg.Clients[ClientCodex]
	binding.RetryPolicy = &domain.RetryPolicy{RequestMaxRetries: &zero}
	cfg.Clients[ClientCodex] = binding
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	before, err := store.CaptureSnapshot()
	if err != nil || !before.retries.Exists {
		t.Fatalf("selected policy was not recorded: %v", err)
	}
	admission, err := store.RollbackConfiguration()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeTOMLConfig(admission, testAdmission())
	if err != nil || !reflect.DeepEqual(decoded.Clients[ClientCodex].RetryPolicy, binding.RetryPolicy) {
		t.Fatalf("admission document lost policy: %v", err)
	}
	afterAdmission, err := store.CaptureSnapshot()
	if err != nil || !before.equal(afterAdmission) {
		t.Fatalf("capability admission changed persistent state: %v", err)
	}
	binding.RetryPolicy = nil
	cfg.Clients[ClientCodex] = binding
	after, err := store.Commit(before, cfg)
	if err != nil || after.retries.Exists || !bytes.Equal(before.Config.Data, after.Config.Data) {
		t.Fatalf("native reset changed canonical bytes or retained policy: %v", err)
	}
	backup, err := store.LoadBackup()
	if err != nil || backup.Clients[ClientCodex].RetryPolicy == nil {
		t.Fatalf("backup lost prior policy: %v", err)
	}
	if err := store.RestoreSnapshot(before, after); err != nil {
		t.Fatal(err)
	}
	restored, err := store.CaptureSnapshot()
	if err != nil || !restored.equal(before) {
		t.Fatalf("compensation did not restore every preimage: %v", err)
	}
}

func TestRetryPolicyComponentsRefuseInvalidIdentityOrContent(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "aigw.toml"), testAdmission())
	cfg := validConfig()
	zero := uint64(0)
	binding := cfg.Clients[ClientCodex]
	binding.RetryPolicy = &domain.RetryPolicy{RequestMaxRetries: &zero}
	cfg.Clients[ClientCodex] = binding
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	snapshot, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	valid := string(snapshot.retries.Data)
	runtime, err := cfg.ResolveRuntime(ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	for name, invalid := range map[string]string{
		"empty":                "",
		"syntax":               "[",
		"unknown":              valid + "unknown = true\n",
		"identity":             strings.Replace(valid, "connection_identity =", "obsolete_identity =", 1),
		"different connection": strings.Replace(valid, runtime.ConnectionFingerprint(), "different", 1),
		"stale":                strings.ReplaceAll(valid, "[clients.codex", "[clients.missing"),
		"missing budget":       strings.ReplaceAll(valid, "request_max_retries = 0", ""),
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(store.retriesPath(), []byte(invalid), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Load(); err == nil {
				t.Fatal("invalid policy component was accepted")
			}
			if _, err := store.LoadCredentialConfig(); err != nil {
				t.Fatalf("policy corruption interrupted credential authority: %v", err)
			}
			retained, err := os.ReadFile(store.retriesPath())
			if err != nil || string(retained) != invalid {
				t.Fatalf("refused read rewrote the component: %v", err)
			}
		})
	}
}

// A retry declaration in the canonical file must agree with the component that
// recorded it, and rollback admission, snapshots and loads refuse a component
// or catalog that cannot be read as written.
func TestRetryPolicyAdmissionRefusesConflictingOrUnreadableComponents(t *testing.T) {
	zero, two := uint64(0), uint64(2)
	declared := func(t *testing.T) (Store, domain.Config) {
		t.Helper()
		store := NewStore(filepath.Join(t.TempDir(), "aigw.toml"), testAdmission())
		cfg := validConfig()
		binding := cfg.Clients[ClientCodex]
		binding.RetryPolicy = &domain.RetryPolicy{RequestMaxRetries: &zero}
		cfg.Clients[ClientCodex] = binding
		if err := store.Save(cfg); err != nil {
			t.Fatal(err)
		}
		return store, cfg
	}
	t.Run("conflicting declaration", func(t *testing.T) {
		store, cfg := declared(t)
		admission, err := store.RollbackConfiguration()
		if err != nil {
			t.Fatal(err)
		}
		// The canonical file now declares zero while the component records two.
		if err := os.WriteFile(store.Path(), admission, 0o600); err != nil {
			t.Fatal(err)
		}
		binding := cfg.Clients[ClientCodex]
		binding.RetryPolicy = &domain.RetryPolicy{RequestMaxRetries: &two}
		cfg.Clients[ClientCodex] = binding
		data, err := encodeRetryPolicies(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(store.retriesPath(), data, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Load(); err == nil || !strings.Contains(err.Error(), "conflicts with its recorded declaration") {
			t.Fatalf("conflicting retry declaration = %v", err)
		}
	})
	t.Run("corrupt component", func(t *testing.T) {
		store, _ := declared(t)
		if err := os.WriteFile(store.retriesPath(), []byte("["), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := store.RollbackConfiguration(); err == nil || !strings.Contains(err.Error(), "parse client retry policies") {
			t.Fatalf("rollback admission with a corrupt retry component = %v", err)
		}
	})
	t.Run("invalid catalog", func(t *testing.T) {
		store, _ := declared(t)
		if err := os.WriteFile(store.catalogPath(), []byte("["), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := store.RollbackConfiguration(); err == nil {
			t.Fatal("rollback admission accepted an invalid catalog")
		}
	})
	t.Run("unreadable component", func(t *testing.T) {
		store, _ := declared(t)
		if err := os.Remove(store.retriesPath()); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(store.retriesPath(), 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := store.CaptureSnapshot(); err == nil {
			t.Fatal("snapshot read a directory as the retry component")
		}
		if _, err := store.Load(); err == nil {
			t.Fatal("load read a directory as the retry component")
		}
	})
}
