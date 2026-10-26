package configuration

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aigw-cli/internal/transaction"
)

func TestRollbackConfigurationPreservesRawBytesWithoutRetryComponent(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "aigw.toml"))
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
	store := NewStore(filepath.Join(t.TempDir(), "aigw.toml"))
	cfg := validConfig()
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(t.Context(), cfg, []string{ClientCodex}); err != nil {
		t.Fatal(err)
	}
	zero := uint64(0)
	binding := cfg.Clients[ClientCodex]
	binding.RetryPolicy = &RetryPolicy{RequestMaxRetries: &zero}
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
	store := NewStore(filepath.Join(t.TempDir(), "aigw.toml"))
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
	binding.RetryPolicy = &RetryPolicy{RequestMaxRetries: &zero}
	cfg.Clients[ClientCodex] = binding
	remove := removeConfigurationFileIfUnchanged
	t.Cleanup(func() { removeConfigurationFileIfUnchanged = remove })
	failure := errors.New("injected checkpoint invalidation failure")
	removeConfigurationFileIfUnchanged = func(path string, preimage transaction.FileSnapshot) (transaction.FileSnapshot, error) {
		if path == store.Path()+".verified.json" {
			return transaction.FileSnapshot{}, failure
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
	store := NewStore(filepath.Join(t.TempDir(), "aigw.toml"))
	cfg := validConfig()
	zero := uint64(0)
	binding := cfg.Clients[ClientCodex]
	binding.RetryPolicy = &RetryPolicy{RequestMaxRetries: &zero}
	cfg.Clients[ClientCodex] = binding
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	before, err := store.CaptureSnapshot()
	if err != nil || !before.Retries.Exists {
		t.Fatalf("selected policy was not recorded: %v", err)
	}
	admission, err := store.RollbackConfiguration()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeTOMLConfig(admission)
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
	if err != nil || after.Retries.Exists || !bytes.Equal(before.Config.Data, after.Config.Data) {
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
	store := NewStore(filepath.Join(t.TempDir(), "aigw.toml"))
	cfg := validConfig()
	zero := uint64(0)
	binding := cfg.Clients[ClientCodex]
	binding.RetryPolicy = &RetryPolicy{RequestMaxRetries: &zero}
	cfg.Clients[ClientCodex] = binding
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	snapshot, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	valid := string(snapshot.Retries.Data)
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
