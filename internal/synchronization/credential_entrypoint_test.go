package synchronization

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	configuration "aigw-cli/internal/configuration"
)

func TestFailedSynchronizationRemovesOnlyItsNewCredentialEntrypoint(t *testing.T) {
	for _, phase := range []string{"commit", "apply"} {
		t.Run(phase, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "aigw")
			helper := filepath.Join(root, "data", "credential", "aigw")
			target := filepath.Join(root, "codex.toml")
			if err := os.WriteFile(source, []byte("AIGW executable fixture"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			store := &configStoreStub{}
			if phase == "commit" {
				store.commitErr = errors.New("commit failed")
			}
			if phase == "apply" {
				store.onCommit = func() {
					if err := os.WriteFile(target, []byte("invalid TOML {"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			}
			syncer := Synchronizer{Config: store, Discovery: targetDiscovery(target), AIGWExecutable: source, CredentialPath: helper}
			cfg := testConfig(target)
			err := syncer.CommitProjection(t.Context(), configuration.NewConfig(), cfg, "test")
			if err == nil {
				t.Fatal("injected failure did not fail synchronization")
			}
			for _, path := range []string{helper, helper + ".sha256"} {
				if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
					t.Fatalf("failed %s left an unconsumed credential entrypoint at %s: %v", phase, path, statErr)
				}
			}
		})
	}
}

func TestFailedReconcileRemovesNewEntrypointAfterProjectionAdmission(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "aigw")
	helper := filepath.Join(root, "data", "credential", "aigw")
	target := filepath.Join(root, "codex.toml")
	if err := os.WriteFile(source, []byte("AIGW executable fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	discovered := targetDiscovery(target)
	var discoveryCalls int
	discovered.onDiscover = func() {
		discoveryCalls++
		if discoveryCalls != 3 {
			return
		}
		if _, err := os.Lstat(helper); err != nil {
			t.Fatalf("reconcile failed before credential entrypoint creation: %v", err)
		}
		if err := os.WriteFile(target, []byte("invalid TOML {"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	syncer := Synchronizer{Discovery: discovered, AIGWExecutable: source, CredentialPath: helper}
	if err := syncer.ReconcileClient(t.Context(), testConfig(target), configuration.ClientCodex); err == nil {
		t.Fatal("post-entrypoint projection failure was not reported")
	}
	if discoveryCalls < 3 {
		t.Fatalf("reconcile did not reach post-entrypoint application: %d discovery calls", discoveryCalls)
	}
	for _, path := range []string{helper, helper + ".sha256"} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("failed reconcile retained unconsumed entrypoint %s: %v", path, err)
		}
	}
}

func TestCredentialEntrypointPlanScopesDefaultTokenClients(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "codex.toml")
	helper := filepath.Join(root, "data", "credential", "aigw")
	cfg := testConfig(target)
	syncer := Synchronizer{CredentialPath: helper}
	if needed, err := syncer.CredentialEntrypointPlan(cfg, configuration.ClientCodex); err != nil || !needed {
		t.Fatalf("default Token client plan = %v, %v; want helper creation", needed, err)
	}
	disabled := cfg.Clone()
	disabled.SetClientActivation(configuration.ClientCodex, false, "", nil)
	if needed, err := syncer.CredentialEntrypointPlan(disabled, configuration.ClientCodex); err != nil || needed {
		t.Fatalf("disabled client plan = %v, %v; want no helper", needed, err)
	}
	external := cfg.Clone()
	binding := external.Clients[configuration.ClientCodex]
	binding.CredentialCommand = filepath.Join(root, "external-helper")
	external.Clients[configuration.ClientCodex] = binding
	if needed, err := syncer.CredentialEntrypointPlan(external, configuration.ClientCodex); err != nil || needed {
		t.Fatalf("explicit credential command plan = %v, %v; want no AIGW helper", needed, err)
	}
	invalid := cfg.Clone()
	delete(invalid.Routes, "gpt")
	if _, err := syncer.CredentialEntrypointPlan(invalid, configuration.ClientCodex); err == nil {
		t.Fatal("invalid selected Route was accepted for credential entrypoint planning")
	}
}

func TestReconcileClientRejectsMissingCredentialSourceBeforeProjection(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "codex.toml")
	helper := filepath.Join(root, "data", "credential", "aigw")
	original := []byte("model_provider = \"native\"\n")
	if err := os.WriteFile(target, original, 0o600); err != nil {
		t.Fatal(err)
	}
	syncer := Synchronizer{Discovery: targetDiscovery(target), AIGWExecutable: filepath.Join(root, "missing-aigw"), CredentialPath: helper}
	if err := syncer.ReconcileClient(t.Context(), testConfig(target), configuration.ClientCodex); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing source error = %v, want missing executable", err)
	}
	if got, err := os.ReadFile(target); err != nil || !bytes.Equal(got, original) {
		t.Fatalf("missing source changed client projection: %q, %v", got, err)
	}
	if _, err := os.Lstat(helper); !os.IsNotExist(err) {
		t.Fatalf("missing source created credential entrypoint: %v", err)
	}
}

func TestReconcileClientPreflightRejectsInvalidTargetBeforeEntrypointCreation(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "aigw")
	target := filepath.Join(root, "codex.toml")
	helper := filepath.Join(root, "data", "credential", "aigw")
	if err := os.WriteFile(source, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	syncer := Synchronizer{Discovery: targetDiscovery(target), AIGWExecutable: source, CredentialPath: helper}
	if err := syncer.ReconcileClient(t.Context(), testConfig(target), configuration.ClientCodex); err == nil {
		t.Fatal("invalid client target passed projection preflight")
	}
	if _, err := os.Lstat(helper); !os.IsNotExist(err) {
		t.Fatalf("preflight failure created credential entrypoint: %v", err)
	}
}
