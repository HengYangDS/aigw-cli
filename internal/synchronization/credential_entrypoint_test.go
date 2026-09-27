package synchronization

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
)

func TestCancelledProjectionAfterVersionedReaderPreparationRestoresOwnedState(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "aigw")
	target := filepath.Join(root, "codex.toml")
	original := []byte("model_provider = \"native\"\n")
	if err := os.WriteFile(source, []byte("AIGW executable fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, original, 0o600); err != nil {
		t.Fatal(err)
	}
	reader, err := credential.VersionedEntrypointPath(filepath.Join(root, "data"), source, "aigw")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	store := &configStoreStub{onCommit: func() {
		if _, err := os.Lstat(reader); err != nil {
			t.Fatalf("reader was not prepared before configuration commit: %v", err)
		}
		cancel()
	}}
	syncer := Synchronizer{Config: store, Discovery: targetDiscovery(target), AIGWExecutable: source, CredentialPath: reader}
	if err := syncer.CommitProjection(ctx, configuration.NewConfig(), testConfig(target), "test"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled projection error = %v, want context cancellation", err)
	}
	if store.commits != 1 || store.restores != 1 {
		t.Fatalf("cancelled projection commits/restores = %d/%d, want 1/1", store.commits, store.restores)
	}
	for _, path := range []string{reader, reader + ".sha256"} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("cancelled projection retained new reader %s: %v", path, err)
		}
	}
	if got, err := os.ReadFile(target); err != nil || !bytes.Equal(got, original) {
		t.Fatalf("cancelled projection changed client file: %q, %v", got, err)
	}
}

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
	if action, err := syncer.CredentialEntrypointPlan(cfg); err != nil || action != CredentialEntrypointInstall {
		t.Fatalf("default Token client plan = %q, %v; want helper creation", action, err)
	}
	disabled := cfg.Clone()
	disabled.SetClientActivation(configuration.ClientCodex, false, "", nil)
	if action, err := syncer.CredentialEntrypointPlan(disabled); err != nil || action != CredentialEntrypointUnchanged {
		t.Fatalf("disabled client plan = %q, %v; want no helper", action, err)
	}
	external := cfg.Clone()
	binding := external.Clients[configuration.ClientCodex]
	binding.CredentialCommand = filepath.Join(root, "external-helper")
	external.Clients[configuration.ClientCodex] = binding
	if action, err := syncer.CredentialEntrypointPlan(external); err != nil || action != CredentialEntrypointUnchanged {
		t.Fatalf("explicit credential command plan = %q, %v; want no AIGW helper", action, err)
	}
	source := filepath.Join(root, "aigw")
	if err := os.WriteFile(source, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := credential.EnsureEntrypoint(source, helper); err != nil {
		t.Fatal(err)
	}
	if action, err := syncer.CredentialEntrypointPlan(disabled); err != nil || action != CredentialEntrypointUnchanged {
		t.Fatalf("last client disabled plan = %q, %v; cached caller may remain", action, err)
	}
	if action, err := syncer.CredentialEntrypointPlan(external); err != nil || action != CredentialEntrypointUnchanged {
		t.Fatalf("external credential plan = %q, %v; cached caller may remain", action, err)
	}
	otherActive := disabled.Clone()
	account := otherActive.Accounts["gateway"]
	account.Endpoints.Anthropic = "https://gateway.test"
	otherActive.Accounts["gateway"] = account
	otherActive.Routes["claude"] = configuration.Route{Account: "gateway", Model: "claude-test", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}}}
	otherActive.SetSelectedRoute(configuration.ClientClaude, "claude")
	otherActive.SetClientActivation(configuration.ClientClaude, true, "/opt/claude", nil)
	if action, err := syncer.CredentialEntrypointPlan(otherActive); err != nil || action != CredentialEntrypointUnchanged {
		t.Fatalf("another default Token client plan = %q, %v; want retained helper", action, err)
	}
	if err := os.WriteFile(helper+".sha256", []byte("changed receipt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := syncer.CredentialEntrypointPlan(cfg); err == nil {
		t.Fatal("active default Token consumer accepted a changed entrypoint")
	}
	if action, err := syncer.CredentialEntrypointPlan(disabled); err != nil || action != CredentialEntrypointUnchanged {
		t.Fatalf("unused changed entrypoint blocked withdrawal: %q, %v", action, err)
	}
	explicitConsumer := otherActive.Clone()
	binding = explicitConsumer.Clients[configuration.ClientClaude]
	binding.CredentialCommand = helper
	explicitConsumer.Clients[configuration.ClientClaude] = binding
	if action, err := syncer.CredentialEntrypointPlan(explicitConsumer); err != nil || action != CredentialEntrypointUnchanged {
		t.Fatalf("explicit client using the owned path plan = %q, %v; want retained helper", action, err)
	}
	invalid := cfg.Clone()
	delete(invalid.Routes, "gpt")
	if _, err := syncer.CredentialEntrypointPlan(invalid); err == nil {
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

func TestDisablingLastTokenClientRetainsUnprovenCachedEntrypoint(t *testing.T) {
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
	store := configuration.NewStore(filepath.Join(root, "config.toml"))
	before := testConfig(target)
	if err := store.Save(before); err != nil {
		t.Fatal(err)
	}
	syncer := Synchronizer{Config: store, Discovery: targetDiscovery(target), AIGWExecutable: source, CredentialPath: helper}
	if err := syncer.CommitProjection(t.Context(), before, before, "setup"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(helper); err != nil {
		t.Fatalf("setup did not create credential entrypoint: %v", err)
	}
	after := before.Clone()
	after.SetClientActivation(configuration.ClientCodex, false, "", nil)
	if err := syncer.CommitProjection(t.Context(), before, after, "disable", configuration.ClientCodex); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{helper, helper + ".sha256"} {
		if _, err := os.Lstat(path); err != nil {
			t.Fatalf("last configured consumer disabled, cached entrypoint lost at %s: %v", path, err)
		}
	}
	stored, err := store.Load()
	if err != nil || stored.Clients[configuration.ClientCodex].Enabled {
		t.Fatalf("Client Binding was not disabled: %#v, %v", stored.Clients, err)
	}
}

func TestDisablingOneTokenClientRetainsEntrypointForAnother(t *testing.T) {
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
	before := testConfig(target)
	account := before.Accounts["gateway"]
	account.Endpoints.Anthropic = "https://gateway.test"
	before.Accounts["gateway"] = account
	before.Routes["claude"] = configuration.Route{Account: "gateway", Model: "claude-test", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}}}
	before.SetSelectedRoute(configuration.ClientClaude, "claude")
	before.SetClientActivation(configuration.ClientClaude, true, "/opt/claude", nil)
	store := configuration.NewStore(filepath.Join(root, "config.toml"))
	if err := store.Save(before); err != nil {
		t.Fatal(err)
	}
	syncer := Synchronizer{Config: store, Discovery: targetDiscovery(target), AIGWExecutable: source, CredentialPath: helper}
	if err := syncer.CommitProjection(t.Context(), before, before, "setup", configuration.ClientCodex); err != nil {
		t.Fatal(err)
	}
	after := before.Clone()
	after.SetClientActivation(configuration.ClientCodex, false, "", nil)
	if err := syncer.CommitProjection(t.Context(), before, after, "disable", configuration.ClientCodex); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(helper); err != nil {
		t.Fatalf("remaining Claude consumer lost credential entrypoint: %v", err)
	}
}

func TestDisablingDefaultClientRetainsEntrypointForExplicitSelfReference(t *testing.T) {
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
	before := testConfig(target)
	account := before.Accounts["gateway"]
	account.Endpoints.Anthropic = "https://gateway.test"
	before.Accounts["gateway"] = account
	before.Routes["claude"] = configuration.Route{Account: "gateway", Model: "claude-test", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}}}
	before.SetSelectedRoute(configuration.ClientClaude, "claude")
	before.SetClientActivation(configuration.ClientClaude, true, "/opt/claude", nil)
	binding := before.Clients[configuration.ClientClaude]
	binding.CredentialCommand = helper
	before.Clients[configuration.ClientClaude] = binding
	store := configuration.NewStore(filepath.Join(root, "config.toml"))
	if err := store.Save(before); err != nil {
		t.Fatal(err)
	}
	syncer := Synchronizer{Config: store, Discovery: targetDiscovery(target), AIGWExecutable: source, CredentialPath: helper}
	if err := syncer.CommitProjection(t.Context(), before, before, "setup", configuration.ClientCodex); err != nil {
		t.Fatal(err)
	}
	after := before.Clone()
	after.SetClientActivation(configuration.ClientCodex, false, "", nil)
	if err := syncer.CommitProjection(t.Context(), before, after, "disable", configuration.ClientCodex); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(helper); err != nil {
		t.Fatalf("explicit active caller lost the owned entrypoint: %v", err)
	}
}

func TestFinalizeCredentialEntrypointRejectsMissingActiveHelper(t *testing.T) {
	root := t.TempDir()
	syncer := Synchronizer{CredentialPath: filepath.Join(root, "data", "credential", "aigw")}
	err := syncer.finalizeCredentialEntrypoint(testConfig(filepath.Join(root, "codex.toml")))
	if err == nil || !strings.Contains(err.Error(), "disappeared after client projection") {
		t.Fatalf("missing active helper finalization = %v", err)
	}
}

func TestFinalizeCredentialEntrypointRejectsInvalidBinding(t *testing.T) {
	root := t.TempDir()
	cfg := testConfig(filepath.Join(root, "codex.toml"))
	delete(cfg.Routes, "gpt")
	syncer := Synchronizer{CredentialPath: filepath.Join(root, "data", "credential", "aigw")}
	if err := syncer.finalizeCredentialEntrypoint(cfg); err == nil || !strings.Contains(err.Error(), "inspect credential entrypoint") {
		t.Fatalf("invalid binding finalization = %v", err)
	}
}

func TestWithdrawingLastClientPreservesChangedCachedEntrypoint(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "aigw")
	helper := filepath.Join(root, "data", "credential", "aigw")
	target := filepath.Join(root, "codex.toml")
	original := []byte("model_provider = \"native\"\n")
	if err := os.WriteFile(source, []byte("AIGW executable fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, original, 0o600); err != nil {
		t.Fatal(err)
	}
	store := configuration.NewStore(filepath.Join(root, "config.toml"))
	before := testConfig(target)
	if err := store.Save(before); err != nil {
		t.Fatal(err)
	}
	syncer := Synchronizer{Config: store, Discovery: targetDiscovery(target), AIGWExecutable: source, CredentialPath: helper}
	if err := syncer.CommitProjection(t.Context(), before, before, "setup"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(helper+".sha256", []byte("changed receipt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	after := before.Clone()
	after.SetClientActivation(configuration.ClientCodex, false, "", nil)
	if err := syncer.CommitProjection(t.Context(), before, after, "disable", configuration.ClientCodex); err != nil {
		t.Fatalf("withdrawal should not inspect an unneeded cached entrypoint: %v", err)
	}
	stored, loadErr := store.Load()
	if loadErr != nil || stored.Clients[configuration.ClientCodex].Enabled {
		t.Fatalf("client withdrawal was not committed: %#v, %v", stored.Clients, loadErr)
	}
	if got, readErr := os.ReadFile(target); readErr != nil || !bytes.Equal(got, original) {
		t.Fatalf("client projection was not withdrawn: %q, %v", got, readErr)
	}
	if _, statErr := os.Lstat(helper); statErr != nil {
		t.Fatalf("changed cached entrypoint was removed: %v", statErr)
	}
}
