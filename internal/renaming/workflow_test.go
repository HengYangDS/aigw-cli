package renaming

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
	"aigw-cli/internal/synchronization"
)

type failingRenameConfigStore struct{ err error }

func (store failingRenameConfigStore) CaptureSnapshot() (configuration.Snapshot, error) {
	return configuration.Snapshot{}, store.err
}
func (failingRenameConfigStore) Save(configuration.Config) error { return nil }
func (store failingRenameConfigStore) Commit(before configuration.Snapshot, _ configuration.Config) (configuration.Snapshot, error) {
	return before, store.err
}
func (failingRenameConfigStore) RestoreSnapshot(configuration.Snapshot, configuration.Snapshot) error {
	return nil
}

type faultTokenStore struct {
	values         map[string]string
	getErrors      map[string]error
	setErr         error
	deleteErr      error
	setReplacement string
	retainDelete   bool
	afterDeleteErr error
	deleted        bool
}

func (store *faultTokenStore) Get(id string) (string, error) {
	if store.deleted && store.afterDeleteErr != nil {
		return "", store.afterDeleteErr
	}
	if err := store.getErrors[id]; err != nil {
		return "", err
	}
	if value, ok := store.values[id]; ok {
		return value, nil
	}
	return "", secrets.ErrNotFound
}

func (store *faultTokenStore) Set(id, value string) error {
	if store.setErr != nil {
		return store.setErr
	}
	if store.setReplacement != "" {
		value = store.setReplacement
	}
	if store.values == nil {
		store.values = map[string]string{}
	}
	store.values[id] = value
	return nil
}

func (store *faultTokenStore) Delete(id string) error {
	if store.deleteErr != nil {
		return store.deleteErr
	}
	store.deleted = true
	if !store.retainDelete {
		delete(store.values, id)
	}
	return nil
}

func (store *faultTokenStore) Exists(id string) (bool, error) {
	_, err := store.Get(id)
	if errors.Is(err, secrets.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

type faultProbeStore struct {
	values         map[string]secrets.DiagnosticCredential
	getErrors      map[string]error
	setErr         error
	deleteErr      error
	setReplacement secrets.DiagnosticCredential
	retainDelete   bool
	afterDeleteErr error
	deleted        bool
}

func (store *faultProbeStore) Get(id string) (secrets.DiagnosticCredential, error) {
	if store.deleted && store.afterDeleteErr != nil {
		return secrets.DiagnosticCredential{}, store.afterDeleteErr
	}
	if err := store.getErrors[id]; err != nil {
		return secrets.DiagnosticCredential{}, err
	}
	if value, ok := store.values[id]; ok {
		return value, nil
	}
	return secrets.DiagnosticCredential{}, secrets.ErrNotFound
}

func (store *faultProbeStore) Set(id string, value secrets.DiagnosticCredential) error {
	if store.setErr != nil {
		return store.setErr
	}
	if store.setReplacement != (secrets.DiagnosticCredential{}) {
		value = store.setReplacement
	}
	if store.values == nil {
		store.values = map[string]secrets.DiagnosticCredential{}
	}
	store.values[id] = value
	return nil
}

func (store *faultProbeStore) Delete(id string) error {
	if store.deleteErr != nil {
		return store.deleteErr
	}
	store.deleted = true
	if !store.retainDelete {
		delete(store.values, id)
	}
	return nil
}

func (store *faultProbeStore) Exists(id string) (bool, error) {
	_, err := store.Get(id)
	if errors.Is(err, secrets.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

func migrationConfig() configuration.Config {
	cfg := configuration.NewConfig()
	cfg.Accounts["old"] = configuration.Account{Label: "Old", Endpoints: configuration.Endpoints{OpenAIResponses: "https://old.test/v1", Anthropic: "https://old.test"}}
	cfg.Profiles["codex"] = configuration.Profile{Label: "Codex", Account: "old", Client: configuration.ClientCodex, Model: "gpt"}
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "old", Client: configuration.ClientClaude, Model: "claude"}
	cfg.Routes[configuration.ClientCodex] = "codex"
	cfg.Routes[configuration.ClientClaude] = "claude"
	return cfg
}

func renamedConfig() configuration.Config {
	cfg := migrationConfig()
	providerAccount := cfg.Accounts["old"]
	delete(cfg.Accounts, "old")
	cfg.Accounts["new"] = providerAccount
	for id, profile := range cfg.Profiles {
		profile.Account = "new"
		cfg.Profiles[id] = profile
	}
	return cfg
}

func renameFinalizationState(t *testing.T) (configuration.Store, configuration.Snapshot) {
	t.Helper()
	store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
	if err := store.Save(migrationConfig()); err != nil {
		t.Fatal(err)
	}
	current := renamedConfig()
	if err := store.Save(current); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(t.Context(), current, configuration.AdmittedClientIDs()); err != nil {
		t.Fatal(err)
	}
	state, err := store.CaptureVerifiedBackupState()
	if err != nil {
		t.Fatal(err)
	}
	return store, state.Snapshot
}

func TestCanceledAccountRenamePreservesOwnedState(t *testing.T) {
	store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
	if err := store.Save(migrationConfig()); err != nil {
		t.Fatal(err)
	}
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	tokens := secrets.NewMemoryStore()
	if err := tokens.Set("old", "source-token"); err != nil {
		t.Fatal(err)
	}
	deps := Service{Config: store, Secrets: tokens, Accounts: &faultProbeStore{}, Synchronizer: synchronization.Synchronizer{Config: store}}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = deps.RenameAccount(ctx, "old", "new", false)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled rename error = %v", err)
	}
	if value, err := tokens.Get("old"); err != nil || value != "source-token" {
		t.Fatalf("source token = %q, error = %v", value, err)
	}
	if _, err := tokens.Get("new"); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("canceled rename created a target credential: %v", err)
	}
	after, err := store.CaptureSnapshot()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("canceled rename changed configuration: %v", err)
	}
}

func TestCanceledAccountFinalizationPreservesOwnedState(t *testing.T) {
	store, _ := renameFinalizationState(t)
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	tokens := secrets.NewMemoryStore()
	for _, id := range []string{"old", "new"} {
		if err := tokens.Set(id, "shared-token"); err != nil {
			t.Fatal(err)
		}
	}
	deps := Service{Config: store, Secrets: tokens, Accounts: &faultProbeStore{}}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = deps.FinalizeAccount(ctx, "old", "new", false, FinalizeOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("canceled finalization error = %v", err)
	}
	for _, id := range []string{"old", "new"} {
		if value, err := tokens.Get(id); err != nil || value != "shared-token" {
			t.Errorf("credential %s = %q, error = %v", id, value, err)
		}
	}
	after, err := store.CaptureSnapshot()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("canceled finalization changed configuration: %v", err)
	}
}

func TestRenameServiceRetainsRetryStateWhenConfigurationCommitFails(t *testing.T) {
	for _, resource := range []string{"account", "profile"} {
		t.Run(resource, func(t *testing.T) {
			store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
			if err := store.Save(migrationConfig()); err != nil {
				t.Fatal(err)
			}
			before, err := store.CaptureSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			tokens := secrets.NewMemoryStore()
			if err := tokens.Set("old", "source-token"); err != nil {
				t.Fatal(err)
			}
			failure := errors.New("configuration commit failed")
			service := Service{Config: store, Secrets: tokens, Accounts: &faultProbeStore{}, Synchronizer: synchronization.Synchronizer{Config: failingRenameConfigStore{err: failure}}}
			if resource == "account" {
				_, err = service.RenameAccount(t.Context(), "old", "new", false)
				if value, getErr := tokens.Get("new"); getErr != nil || value != "source-token" {
					t.Fatalf("prepared retry credential = %q, error = %v", value, getErr)
				}
			} else {
				_, err = service.RenameProfile(t.Context(), "codex", "new", false)
			}
			if !errors.Is(err, failure) {
				t.Fatalf("rename error = %v, want commit failure", err)
			}
			if value, getErr := tokens.Get("old"); getErr != nil || value != "source-token" {
				t.Fatalf("rollback credential = %q, error = %v", value, getErr)
			}
			after, err := store.CaptureSnapshot()
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
	if _, err := planProfile(cfg, "missing", "new"); err == nil {
		t.Fatal("expected unknown profile")
	}

	invalid := cfg.Clone()
	invalid.Routes[configuration.ClientCodex] = "missing"
	if _, err := planAccount(invalid, "old", "new"); err == nil || !strings.Contains(err.Error(), "Validate") {
		t.Fatalf("account error = %v", err)
	}
	if _, err := planProfile(invalid, "codex", "new-codex"); err == nil || !strings.Contains(err.Error(), "Validate") {
		t.Fatalf("profile error = %v", err)
	}

	plan, err := planProfile(cfg, "codex", "new-codex")
	if err != nil || plan.Config.Routes[configuration.ClientClaude] != "claude" {
		t.Fatalf("plan=%#v error=%v", plan, err)
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
		{name: "target probe", secrets: &faultTokenStore{}, accounts: &faultProbeStore{values: map[string]secrets.DiagnosticCredential{"old": {SystemToken: "s", UserID: "u"}}, getErrors: map[string]error{"new": want}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deps := Service{Secrets: test.secrets, Accounts: test.accounts}
			if _, err := planCredentialCopies(deps, base); !errors.Is(err, want) {
				t.Fatalf("error = %v, want %v", err, want)
			}
		})
	}
}

func TestApplyAccountCredentialCopiesFailures(t *testing.T) {
	want := errors.New("operation failed")
	probe := secrets.DiagnosticCredential{SystemToken: "system", UserID: "user"}
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
		{name: "probe differs", secrets: &faultTokenStore{}, accounts: &faultProbeStore{setReplacement: secrets.DiagnosticCredential{SystemToken: "other", UserID: "other"}}, plan: Plan{NewID: "new", probeCopy: probeCopy{value: probe, copy: true}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := applyCredentialCopies(Service{Secrets: test.secrets, Accounts: test.accounts}, test.plan)
			if err == nil {
				t.Fatal("expected copy failure")
			}
		})
	}
}

func TestPlanAccountFinalizeCredentialReadErrors(t *testing.T) {
	want := errors.New("read failed")
	probe := secrets.DiagnosticCredential{SystemToken: "system", UserID: "user"}
	tests := []struct {
		name     string
		secrets  *faultTokenStore
		accounts *faultProbeStore
	}{
		{name: "target token", secrets: &faultTokenStore{getErrors: map[string]error{"new": want}}, accounts: &faultProbeStore{}},
		{name: "source token", secrets: &faultTokenStore{values: map[string]string{"new": "token"}, getErrors: map[string]error{"old": want}}, accounts: &faultProbeStore{}},
		{name: "source probe", secrets: &faultTokenStore{values: map[string]string{"new": "token"}}, accounts: &faultProbeStore{getErrors: map[string]error{"old": want}}},
		{name: "target probe", secrets: &faultTokenStore{values: map[string]string{"new": "token"}}, accounts: &faultProbeStore{values: map[string]secrets.DiagnosticCredential{"old": probe}, getErrors: map[string]error{"new": want}}},
		{name: "target probe absent", secrets: &faultTokenStore{values: map[string]string{"new": "token"}}, accounts: &faultProbeStore{values: map[string]secrets.DiagnosticCredential{"old": probe}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store, _ := renameFinalizationState(t)
			deps := Service{Config: store, Secrets: test.secrets, Accounts: test.accounts}
			if _, err := planFinalize(deps, "old", "new", FinalizeOptions{}); err == nil {
				t.Fatal("expected finalization planning failure")
			}
		})
	}
}

func TestPlanAccountFinalizeRejectsIncompleteRenameState(t *testing.T) {
	for _, test := range []struct {
		name    string
		config  configuration.Config
		target  string
		problem string
	}{
		{"source account remains", migrationConfig(), "new", "still exists"},
		{"target account missing", renamedConfig(), "missing", "does not exist"},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
			if err := store.Save(test.config); err != nil {
				t.Fatal(err)
			}
			if err := store.SaveVerifiedCheckpoint(t.Context(), test.config, configuration.AdmittedClientIDs()); err != nil {
				t.Fatal(err)
			}
			deps := Service{Config: store, Secrets: &faultTokenStore{}, Accounts: &faultProbeStore{}}
			if _, err := planFinalize(deps, "old", test.target, FinalizeOptions{}); err == nil || !strings.Contains(err.Error(), test.problem) {
				t.Fatalf("planFinalize() error = %v", err)
			}
		})
	}
}

func TestFinalizeClientCoverageValidation(t *testing.T) {
	if coversAllAdmittedClients([]string{configuration.ClientCodex, configuration.ClientCodex}) {
		t.Fatal("duplicate clients accepted")
	}
	if coversAllAdmittedClients([]string{configuration.ClientCodex, "future"}) {
		t.Fatal("unknown client accepted")
	}
	if coversAllAdmittedClients([]string{configuration.ClientCodex}) {
		t.Fatal("missing client accepted")
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
		{name: "token verify read", plan: Plan{OldID: "old", deleteToken: true}, secrets: &faultTokenStore{values: map[string]string{"old": "token"}, afterDeleteErr: want}, accounts: &faultProbeStore{}},
		{name: "token remains", plan: Plan{OldID: "old", deleteToken: true}, secrets: &faultTokenStore{values: map[string]string{"old": "token"}, retainDelete: true}, accounts: &faultProbeStore{}},
		{name: "external read", plan: Plan{OldID: "old", externalTokenCleanup: true}, secrets: &faultTokenStore{getErrors: map[string]error{"old": want}}, accounts: &faultProbeStore{}},
		{name: "external remains", plan: Plan{OldID: "old", externalTokenCleanup: true}, secrets: &faultTokenStore{values: map[string]string{"old": "token"}}, accounts: &faultProbeStore{}},
		{name: "probe delete", plan: Plan{OldID: "old", deleteProbe: true}, secrets: &faultTokenStore{}, accounts: &faultProbeStore{deleteErr: want}},
		{name: "probe verify read", plan: Plan{OldID: "old", deleteProbe: true}, secrets: &faultTokenStore{}, accounts: &faultProbeStore{values: map[string]secrets.DiagnosticCredential{"old": {SystemToken: "s", UserID: "u"}}, afterDeleteErr: want}},
		{name: "probe remains", plan: Plan{OldID: "old", deleteProbe: true}, secrets: &faultTokenStore{}, accounts: &faultProbeStore{values: map[string]secrets.DiagnosticCredential{"old": {SystemToken: "s", UserID: "u"}}, retainDelete: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store, snapshot := renameFinalizationState(t)
			test.plan.snapshot = snapshot
			deps := Service{Config: store, Secrets: test.secrets, Accounts: test.accounts}
			if _, err := applyFinalize(context.Background(), deps, test.plan); err == nil {
				t.Fatal("expected cleanup failure")
			}
		})
	}

	t.Run("backup convergence", func(t *testing.T) {
		_, snapshot := renameFinalizationState(t)
		deps := Service{Config: configuration.NewStore(t.TempDir()), Secrets: &faultTokenStore{}, Accounts: &faultProbeStore{}}
		if _, err := applyFinalize(context.Background(), deps, Plan{snapshot: snapshot}); err == nil || !strings.Contains(err.Error(), "Converge") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestApplyFinalizeStopsBeforeCleanupWhenProbeVerificationFails(t *testing.T) {
	store, snapshot := renameFinalizationState(t)
	plan := Plan{NewID: "new", verifyProbe: true, snapshot: snapshot, Account: configuration.Account{Label: "New"}}
	deps := Service{Config: store, Secrets: &faultTokenStore{}, Accounts: &faultProbeStore{}}
	if _, err := applyFinalize(context.Background(), deps, plan); err == nil || !strings.Contains(err.Error(), "does not declare precise diagnostics") {
		t.Fatalf("applyFinalize() error = %v", err)
	}
}

func TestVerifyFinalizedAccountProbeFailures(t *testing.T) {
	base := Plan{NewID: "new", Account: configuration.Account{Label: "New"}}
	deps := Service{Secrets: &faultTokenStore{}, Accounts: &faultProbeStore{}}
	if err := verifyFinalizedAccountProbe(context.Background(), deps, base); err == nil || !strings.Contains(err.Error(), "does not declare") {
		t.Fatalf("error = %v", err)
	}

	base.Account.AccountProbe = &configuration.AccountProbe{Kind: "future", BaseURL: "https://new.test"}
	if err := verifyFinalizedAccountProbe(context.Background(), deps, base); err == nil || !strings.Contains(err.Error(), "not included") {
		t.Fatalf("error = %v", err)
	}

	base.Account.AccountProbe = &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://new.test"}
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

	deps.Accounts = &faultProbeStore{values: map[string]secrets.DiagnosticCredential{"new": {SystemToken: "s", UserID: "u"}}}
	deps.HTTP = probeTransport(func(*http.Request) (*http.Response, error) { return nil, want })
	if err := verifyFinalizedAccountProbe(context.Background(), deps, base); !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
}

type probeTransport func(*http.Request) (*http.Response, error)

func (do probeTransport) Do(request *http.Request) (*http.Response, error) { return do(request) }
