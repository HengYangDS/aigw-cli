package renaming

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/client/claude"
	"aigw-cli/internal/client/codex"
	"aigw-cli/internal/client/hermes"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
	"aigw-cli/internal/synchronization"
)

func TestAccountFinalizationConvergesForwardingBackupBeforeRetirement(t *testing.T) {
	store, _ := renameFinalizationState(t)
	cfg, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SetForwardingEndpoint(codex.ID, "http://127.0.0.1:8792/v1"); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(t.Context(), cfg, clients.Admission().IDs()); err != nil {
		t.Fatal(err)
	}
	state, err := store.CaptureVerifiedBackupState()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ConvergeVerifiedBackup(state.Snapshot); err != nil {
		t.Fatal(err)
	}
	component := strings.TrimSuffix(store.Path(), filepath.Ext(store.Path())) + ".forwarding.toml.bak"
	if err := os.Remove(component); err != nil {
		t.Fatal(err)
	}
	plan, err := planFinalize(Renamer{Config: store, Secrets: secret.NewMemoryStore(), Accounts: &faultProbeStore{}}, "old", "new", FinalizeOptions{})
	if err != nil || plan.Status == StatusAlreadyFinalized || plan.Actions.Backup != "converge-to-verified-current" {
		t.Fatalf("finalization ignored missing forwarding recovery state: %#v, %v", plan, err)
	}
}

func migrationConfig() domain.Config {
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["old"] = domain.Account{Label: "Old", Endpoints: domain.Endpoints{OpenAIResponses: "https://old.test/v1", Anthropic: "https://old.test"}}
	cfg.Routes["codex"] = domain.Route{Label: "Codex", Account: "old", Model: "gpt", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	cfg.Routes["claude"] = domain.Route{Label: "Claude", Account: "old", Model: "claude", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.SetSelectedRoute(codex.ID, "codex", "")
	cfg.SetSelectedRoute(claude.ID, "claude", "")
	return cfg
}

func renamedConfig() domain.Config {
	cfg := migrationConfig()
	providerAccount := cfg.Accounts["old"]
	delete(cfg.Accounts, "old")
	cfg.Accounts["new"] = providerAccount
	for id, route := range cfg.Routes {
		route.Account = "new"
		cfg.Routes[id] = route
	}
	return cfg
}

func renameFinalizationState(t *testing.T) (store.Store, store.Snapshot) {
	t.Helper()
	configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	if err := configStore.Save(migrationConfig()); err != nil {
		t.Fatal(err)
	}
	current := renamedConfig()
	if err := configStore.Save(current); err != nil {
		t.Fatal(err)
	}
	if err := configStore.SaveVerifiedCheckpoint(t.Context(), current, clients.Admission().IDs()); err != nil {
		t.Fatal(err)
	}
	state, err := configStore.CaptureVerifiedBackupState()
	if err != nil {
		t.Fatal(err)
	}
	return configStore, state.Snapshot
}

func TestCanceledAccountOperationPreservesOwnedState(t *testing.T) {
	for _, operation := range []string{"rename", "finalize"} {
		t.Run(operation, func(t *testing.T) {
			configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
			if operation == "finalize" {
				configStore, _ = renameFinalizationState(t)
			} else if err := configStore.Save(migrationConfig()); err != nil {
				t.Fatal(err)
			}
			before, err := configStore.CaptureSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			tokens := secret.NewMemoryStore()
			for _, id := range []string{"old", "new"} {
				if id == "new" && operation == "rename" {
					continue
				}
				if err := tokens.Set(id, "source-token"); err != nil {
					t.Fatal(err)
				}
			}
			deps := Renamer{Config: configStore, Secrets: tokens, Accounts: &faultProbeStore{}, Synchronizer: synchronization.Synchronizer{Registry: clients.Registry(), Config: configStore}}
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			if operation == "rename" {
				_, err = deps.RenameAccount(ctx, "old", "new", false)
			} else {
				_, err = deps.FinalizeAccount(ctx, "old", "new", false, FinalizeOptions{})
			}
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled %s error = %v", operation, err)
			}
			if value, err := tokens.Get("old"); err != nil || value != "source-token" {
				t.Fatalf("source token = %q, error = %v", value, err)
			}
			target, targetErr := tokens.Get("new")
			if operation == "rename" && !errors.Is(targetErr, secret.ErrNotFound) ||
				operation == "finalize" && (targetErr != nil || target != "source-token") {
				t.Fatalf("canceled %s changed target credential: %q, %v", operation, target, targetErr)
			}
			after, err := configStore.CaptureSnapshot()
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("canceled %s changed configuration: %v", operation, err)
			}
		})
	}
}

func TestRenamerRetainsRetryStateWhenConfigurationCommitFails(t *testing.T) {
	for _, resource := range []string{"account", "route"} {
		t.Run(resource, func(t *testing.T) {
			configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
			if err := configStore.Save(migrationConfig()); err != nil {
				t.Fatal(err)
			}
			before, err := configStore.CaptureSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			tokens := secret.NewMemoryStore()
			if err := tokens.Set("old", "source-token"); err != nil {
				t.Fatal(err)
			}
			failure := errors.New("configuration commit failed")
			renamer := Renamer{Config: configStore, Secrets: tokens, Accounts: &faultProbeStore{}, Synchronizer: synchronization.Synchronizer{Registry: clients.Registry(), Config: failingRenameConfigStore{err: failure}}}
			if resource == "account" {
				_, err = renamer.RenameAccount(t.Context(), "old", "new", false)
				if value, getErr := tokens.Get("new"); getErr != nil || value != "source-token" {
					t.Fatalf("prepared retry credential = %q, error = %v", value, getErr)
				}
			} else {
				_, err = renamer.RenameRoute(t.Context(), "codex", "new", false)
			}
			if !errors.Is(err, failure) {
				t.Fatalf("rename error = %v, want commit failure", err)
			}
			if value, getErr := tokens.Get("old"); getErr != nil || value != "source-token" {
				t.Fatalf("rollback credential = %q, error = %v", value, getErr)
			}
			after, err := configStore.CaptureSnapshot()
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("failed commit changed configuration: %v", err)
			}
		})
	}
}

func TestRenamePlanningValidationAndReferenceBranches(t *testing.T) {
	cfg := migrationConfig()
	if _, err := planAccount(cfg, "missing", "new"); err == nil {
		t.Fatal("expected unknown account")
	}
	if _, err := planRoute(cfg, "missing", "new"); err == nil {
		t.Fatal("expected unknown route")
	}

	invalid := cfg.Clone()
	invalid.SetSelectedRoute(codex.ID, "missing", "")
	if _, err := planAccount(invalid, "old", "new"); err == nil || !strings.Contains(err.Error(), "Validate") {
		t.Fatalf("account error = %v", err)
	}
	if _, err := planRoute(invalid, "codex", "new-codex"); err == nil || !strings.Contains(err.Error(), "Validate") {
		t.Fatalf("route error = %v", err)
	}

	cfg.SetRecommendedRoute(claude.ID, "claude")
	cfg.SetRecommendedRoute(codex.ID, "codex")
	cfg.Routes["backup"] = cfg.Routes["codex"]
	cfg.Recommendations[hermes.ID] = domain.ClientRecommendation{
		Primary: domain.ClientSelection{Route: "claude", Protocol: domain.ProtocolAnthropic},
		Alternatives: []domain.ClientSelection{
			{Route: "backup", Protocol: domain.ProtocolOpenAIResponses},
			{Route: "codex", Protocol: domain.ProtocolOpenAIResponses},
		},
	}
	plan, err := planRoute(cfg, "codex", "new-codex")
	if err != nil || plan.Config.SelectedRoute(claude.ID) != "claude" ||
		plan.Config.RecommendedRoute(codex.ID) != "new-codex" ||
		plan.Config.RecommendedRoute(claude.ID) != "claude" {
		t.Fatalf("plan=%#v error=%v", plan, err)
	}
	want := cfg.Clone().Recommendations[hermes.ID]
	want.Alternatives[1].Route = "new-codex"
	if !reflect.DeepEqual(plan.Config.Recommendations[hermes.ID], want) || cfg.Recommendations[hermes.ID].Alternatives[1].Route != "codex" || !slices.Contains(plan.AffectedReferences, "recommendations.hermes.alternatives.1.route") {
		t.Fatalf("rename changed alternative order, protocol or input configuration: %#v", plan)
	}
}

func TestAccountCredentialPlanningReadErrors(t *testing.T) {
	base, err := planAccount(migrationConfig(), "old", "new")
	if err != nil {
		t.Fatal(err)
	}
	want := errors.New("read failed")
	tests := []struct {
		name     string
		secrets  *faultTokenStore
		accounts *faultProbeStore
	}{
		{name: "source token", secrets: &faultTokenStore{getErrors: map[string]error{"old": want}}, accounts: &faultProbeStore{}},
		{name: "target token", secrets: &faultTokenStore{values: map[string]string{"old": "token"}, getErrors: map[string]error{"new": want}}, accounts: &faultProbeStore{}},
		{name: "source probe", secrets: &faultTokenStore{}, accounts: &faultProbeStore{getErrors: map[string]error{"old": want}}},
		{name: "target probe", secrets: &faultTokenStore{}, accounts: &faultProbeStore{values: map[string]secret.DiagnosticCredential{"old": {SystemToken: "s", UserID: "u"}}, getErrors: map[string]error{"new": want}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deps := Renamer{Secrets: test.secrets, Accounts: test.accounts}
			if _, err := planCredentialCopies(deps, base); !errors.Is(err, want) {
				t.Fatalf("error = %v, want %v", err, want)
			}
		})
	}
}

func TestApplyAccountCredentialCopiesFailures(t *testing.T) {
	want := errors.New("operation failed")
	probe := secret.DiagnosticCredential{SystemToken: "system", UserID: "user"}
	plan := Plan{OldID: "old", NewID: "new", tokenCopy: tokenCopy{value: "token", copy: true}}
	tests := []struct {
		name     string
		secrets  *faultTokenStore
		accounts *faultProbeStore
		plan     Plan
	}{
		{name: "token set", secrets: &faultTokenStore{setErr: want}, accounts: &faultProbeStore{}, plan: plan},
		{name: "token get", secrets: &faultTokenStore{getErrors: map[string]error{"new": want}}, accounts: &faultProbeStore{}, plan: plan},
		{name: "token differs", secrets: &faultTokenStore{setReplacement: "other"}, accounts: &faultProbeStore{}, plan: plan},
		{name: "probe set", secrets: &faultTokenStore{}, accounts: &faultProbeStore{setErr: want}, plan: Plan{NewID: "new", probeCopy: probeCopy{value: probe, copy: true}}},
		{name: "probe get", secrets: &faultTokenStore{}, accounts: &faultProbeStore{getErrors: map[string]error{"new": want}}, plan: Plan{NewID: "new", probeCopy: probeCopy{value: probe, copy: true}}},
		{name: "probe differs", secrets: &faultTokenStore{}, accounts: &faultProbeStore{setReplacement: secret.DiagnosticCredential{SystemToken: "other", UserID: "other"}}, plan: Plan{NewID: "new", probeCopy: probeCopy{value: probe, copy: true}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := applyCredentialCopies(Renamer{Secrets: test.secrets, Accounts: test.accounts}, test.plan)
			if err == nil {
				t.Fatal("expected copy failure")
			}
		})
	}
}

func TestPlanAccountFinalizeCredentialReadErrors(t *testing.T) {
	want := errors.New("read failed")
	probe := secret.DiagnosticCredential{SystemToken: "system", UserID: "user"}
	tests := []struct {
		name     string
		secrets  *faultTokenStore
		accounts *faultProbeStore
	}{
		{name: "target token", secrets: &faultTokenStore{getErrors: map[string]error{"new": want}}, accounts: &faultProbeStore{}},
		{name: "source token", secrets: &faultTokenStore{values: map[string]string{"new": "token"}, getErrors: map[string]error{"old": want}}, accounts: &faultProbeStore{}},
		{name: "source probe", secrets: &faultTokenStore{values: map[string]string{"new": "token"}}, accounts: &faultProbeStore{getErrors: map[string]error{"old": want}}},
		{name: "target probe", secrets: &faultTokenStore{values: map[string]string{"new": "token"}}, accounts: &faultProbeStore{values: map[string]secret.DiagnosticCredential{"old": probe}, getErrors: map[string]error{"new": want}}},
		{name: "target probe absent", secrets: &faultTokenStore{values: map[string]string{"new": "token"}}, accounts: &faultProbeStore{values: map[string]secret.DiagnosticCredential{"old": probe}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store, _ := renameFinalizationState(t)
			deps := Renamer{Config: store, Secrets: test.secrets, Accounts: test.accounts}
			if _, err := planFinalize(deps, "old", "new", FinalizeOptions{}); err == nil {
				t.Fatal("expected finalization planning failure")
			}
		})
	}
}

func TestPlanAccountFinalizeRejectsIncompleteRenameState(t *testing.T) {
	for _, test := range []struct {
		name    string
		config  domain.Config
		target  string
		problem string
	}{
		{"source account remains", migrationConfig(), "new", "still exists"},
		{"target account missing", renamedConfig(), "missing", "does not exist"},
	} {
		t.Run(test.name, func(t *testing.T) {
			configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
			if err := configStore.Save(test.config); err != nil {
				t.Fatal(err)
			}
			if err := configStore.SaveVerifiedCheckpoint(t.Context(), test.config, clients.Admission().IDs()); err != nil {
				t.Fatal(err)
			}
			deps := Renamer{Config: configStore, Secrets: &faultTokenStore{}, Accounts: &faultProbeStore{}}
			if _, err := planFinalize(deps, "old", test.target, FinalizeOptions{}); err == nil || !strings.Contains(err.Error(), test.problem) {
				t.Fatalf("planFinalize() error = %v", err)
			}
		})
	}
}

func TestApplyAccountFinalizeCleanupFailures(t *testing.T) {
	want := errors.New("cleanup failed")
	tests := []struct {
		name     string
		plan     Plan
		secrets  *faultTokenStore
		accounts *faultProbeStore
	}{
		{name: "token delete", plan: Plan{OldID: "old", deleteToken: true}, secrets: &faultTokenStore{deleteErr: want}, accounts: &faultProbeStore{}},
		{name: "token verify metadata", plan: Plan{OldID: "old", deleteToken: true}, secrets: &faultTokenStore{values: map[string]string{"old": "token"}, afterDeleteErr: want}, accounts: &faultProbeStore{}},
		{name: "token remains", plan: Plan{OldID: "old", deleteToken: true}, secrets: &faultTokenStore{values: map[string]string{"old": "token"}, retainDelete: true}, accounts: &faultProbeStore{}},
		{name: "external observation", plan: Plan{OldID: "old", externalTokenCleanup: true}, secrets: &faultTokenStore{getErrors: map[string]error{"old": want}}, accounts: &faultProbeStore{}},
		{name: "external remains", plan: Plan{OldID: "old", externalTokenCleanup: true}, secrets: &faultTokenStore{values: map[string]string{"old": "token"}}, accounts: &faultProbeStore{}},
		{name: "probe delete", plan: Plan{OldID: "old", deleteProbe: true}, secrets: &faultTokenStore{}, accounts: &faultProbeStore{deleteErr: want}},
		{name: "probe verify metadata", plan: Plan{OldID: "old", deleteProbe: true}, secrets: &faultTokenStore{}, accounts: &faultProbeStore{values: map[string]secret.DiagnosticCredential{"old": {SystemToken: "s", UserID: "u"}}, afterDeleteErr: want}},
		{name: "probe remains", plan: Plan{OldID: "old", deleteProbe: true}, secrets: &faultTokenStore{}, accounts: &faultProbeStore{values: map[string]secret.DiagnosticCredential{"old": {SystemToken: "s", UserID: "u"}}, retainDelete: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configStore, snapshot := renameFinalizationState(t)
			test.plan.snapshot = snapshot
			deps := Renamer{Config: configStore, Secrets: test.secrets, Accounts: test.accounts}
			if _, err := applyFinalize(context.Background(), deps, test.plan); err == nil {
				t.Fatal("expected cleanup failure")
			}
		})
	}

	t.Run("backup convergence", func(t *testing.T) {
		_, snapshot := renameFinalizationState(t)
		deps := Renamer{Config: store.NewStore(t.TempDir(), clients.Admission()), Secrets: &faultTokenStore{}, Accounts: &faultProbeStore{}}
		if _, err := applyFinalize(context.Background(), deps, Plan{snapshot: snapshot}); err == nil || !strings.Contains(err.Error(), "Converge") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestFinalizeVerifiesDeletedSlotsWithoutReadingCredentialValues(t *testing.T) {
	store, snapshot := renameFinalizationState(t)
	unavailable := errors.New("credential values are not available for inspection")
	tokens := &faultTokenStore{values: map[string]string{"old": "token"}, getErrors: map[string]error{"old": unavailable}}
	probes := &faultProbeStore{values: map[string]secret.DiagnosticCredential{"old": {SystemToken: "system", UserID: "user"}}, getErrors: map[string]error{"old": unavailable}}
	plan := Plan{OldID: "old", snapshot: snapshot, deleteToken: true, deleteProbe: true}
	result, err := applyFinalize(t.Context(), Renamer{Config: store, Secrets: tokens, Accounts: probes}, plan)
	if err != nil || result.Status != "finalized" {
		t.Fatalf("metadata-only finalization = %#v, %v", result, err)
	}
	if len(tokens.values) != 0 || len(probes.values) != 0 {
		t.Fatal("finalization retained credential slots")
	}
}

func TestApplyFinalizeStopsBeforeCleanupWhenProbeVerificationFails(t *testing.T) {
	store, snapshot := renameFinalizationState(t)
	plan := Plan{NewID: "new", verifyProbe: true, snapshot: snapshot, Account: domain.Account{Label: "New"}}
	deps := Renamer{Config: store, Secrets: &faultTokenStore{}, Accounts: &faultProbeStore{}}
	if _, err := applyFinalize(context.Background(), deps, plan); err == nil || !strings.Contains(err.Error(), "does not declare precise diagnostics") {
		t.Fatalf("applyFinalize() error = %v", err)
	}
}

func TestVerifyFinalizedAccountProbeFailures(t *testing.T) {
	base := Plan{NewID: "new", Account: domain.Account{Label: "New"}}
	deps := Renamer{Secrets: &faultTokenStore{}, Accounts: &faultProbeStore{}}
	if err := verifyFinalizedAccountProbe(context.Background(), deps, base); err == nil || !strings.Contains(err.Error(), "does not declare") {
		t.Fatalf("error = %v", err)
	}

	base.Account.AccountProbe = &domain.AccountProbe{Kind: "future", BaseURL: "https://new.test"}
	if err := verifyFinalizedAccountProbe(context.Background(), deps, base); err == nil || !strings.Contains(err.Error(), "not included") {
		t.Fatalf("error = %v", err)
	}

	base.Account.AccountProbe = &domain.AccountProbe{Kind: "dmxapi", BaseURL: "https://new.test"}
	want := errors.New("read failed")
	deps.Secrets = &faultTokenStore{getErrors: map[string]error{"new": want}}
	if err := verifyFinalizedAccountProbe(context.Background(), deps, base); !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}

	deps.Secrets = &faultTokenStore{values: map[string]string{"new": "token"}}
	deps.Accounts = &faultProbeStore{getErrors: map[string]error{"new": want}}
	if err := verifyFinalizedAccountProbe(context.Background(), deps, base); !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}

	deps.Accounts = &faultProbeStore{values: map[string]secret.DiagnosticCredential{"new": {SystemToken: "s", UserID: "u"}}}
	deps.HTTP = probeTransport(func(*http.Request) (*http.Response, error) { return nil, want })
	if err := verifyFinalizedAccountProbe(context.Background(), deps, base); !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
}

type probeTransport func(*http.Request) (*http.Response, error)

func (do probeTransport) Do(request *http.Request) (*http.Response, error) { return do(request) }
