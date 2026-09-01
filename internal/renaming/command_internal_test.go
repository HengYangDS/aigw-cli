package renaming

import (
	"aigw-cli/internal/prompt"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/account"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
	"aigw-cli/internal/synchronization"
)

type failingRenameConfigStore struct{ err error }

func (store failingRenameConfigStore) CaptureSnapshot() (configuration.Snapshot, error) {
	return configuration.Snapshot{}, store.err
}
func (failingRenameConfigStore) Save(configuration.Config) error { return nil }
func (failingRenameConfigStore) RestoreSnapshot(configuration.Snapshot, configuration.Snapshot) error {
	return nil
}

type renameCoveragePrompt struct {
	selected  string
	selectErr error
	text      string
	textErr   error
}

func (prompt renameCoveragePrompt) Secret(string) (string, error) { return "", nil }
func (prompt renameCoveragePrompt) Select(string, []prompt.Choice) (string, error) {
	return prompt.selected, prompt.selectErr
}
func (prompt renameCoveragePrompt) Text(string) (string, error) { return prompt.text, prompt.textErr }

type renameCoverageSecrets struct {
	values         map[string]string
	getErrors      map[string]error
	setErr         error
	deleteErr      error
	setReplacement string
	retainDelete   bool
	afterDeleteErr error
	deleted        bool
}

func (store *renameCoverageSecrets) Get(id string) (string, error) {
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

func (store *renameCoverageSecrets) Set(id, value string) error {
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

func (store *renameCoverageSecrets) Delete(id string) error {
	if store.deleteErr != nil {
		return store.deleteErr
	}
	store.deleted = true
	if !store.retainDelete {
		delete(store.values, id)
	}
	return nil
}

func (store *renameCoverageSecrets) Exists(id string) (bool, error) {
	_, err := store.Get(id)
	if errors.Is(err, secrets.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

type renameCoverageAccounts struct {
	values         map[string]account.Credential
	getErrors      map[string]error
	setErr         error
	deleteErr      error
	setReplacement account.Credential
	retainDelete   bool
	afterDeleteErr error
	deleted        bool
}

func (store *renameCoverageAccounts) Get(id string) (account.Credential, error) {
	if store.deleted && store.afterDeleteErr != nil {
		return account.Credential{}, store.afterDeleteErr
	}
	if err := store.getErrors[id]; err != nil {
		return account.Credential{}, err
	}
	if value, ok := store.values[id]; ok {
		return value, nil
	}
	return account.Credential{}, account.ErrNotFound
}

func (store *renameCoverageAccounts) Set(id string, value account.Credential) error {
	if store.setErr != nil {
		return store.setErr
	}
	if store.setReplacement != (account.Credential{}) {
		value = store.setReplacement
	}
	if store.values == nil {
		store.values = map[string]account.Credential{}
	}
	store.values[id] = value
	return nil
}

func (store *renameCoverageAccounts) Delete(id string) error {
	if store.deleteErr != nil {
		return store.deleteErr
	}
	store.deleted = true
	if !store.retainDelete {
		delete(store.values, id)
	}
	return nil
}

func (store *renameCoverageAccounts) Has(id string) bool { _, err := store.Get(id); return err == nil }

func renameCoverageConfig() configuration.Config {
	cfg := configuration.NewConfig()
	cfg.Accounts["old"] = configuration.Account{Label: "Old", Endpoints: configuration.Endpoints{OpenAIResponses: "https://old.test/v1", Anthropic: "https://old.test"}}
	cfg.Profiles["codex"] = configuration.Profile{Label: "Codex", Account: "old", Client: configuration.ClientCodex, Model: "gpt"}
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "old", Client: configuration.ClientClaude, Model: "claude"}
	cfg.Routes[configuration.ClientCodex] = "codex"
	cfg.Routes[configuration.ClientClaude] = "claude"
	return cfg
}

func renamedCoverageConfig() configuration.Config {
	cfg := renameCoverageConfig()
	providerAccount := cfg.Accounts["old"]
	delete(cfg.Accounts, "old")
	cfg.Accounts["new"] = providerAccount
	for id, profile := range cfg.Profiles {
		profile.Account = "new"
		cfg.Profiles[id] = profile
	}
	return cfg
}

func renameFinalizationState(t *testing.T) (configuration.Store, configuration.VerifiedBackupSnapshot) {
	t.Helper()
	store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
	if err := store.Save(renameCoverageConfig()); err != nil {
		t.Fatal(err)
	}
	current := renamedCoverageConfig()
	if err := store.Save(current); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(current, configuration.AdmittedClientIDs()); err != nil {
		t.Fatal(err)
	}
	state, err := store.CaptureVerifiedBackupState()
	if err != nil {
		t.Fatal(err)
	}
	return store, state.Snapshot
}

func TestResolveRenameIDsErrorBranches(t *testing.T) {
	if _, _, err := resolveIDs(Dependencies{}, "profile", nil, nil); err == nil {
		t.Fatal("expected non-interactive error")
	}
	if _, _, err := resolveIDs(Dependencies{Interactive: true}, "profile", nil, nil); err == nil {
		t.Fatal("expected missing prompt error")
	}
	if _, _, err := resolveIDs(Dependencies{Interactive: true, Prompt: renameCoveragePrompt{}}, "profile", nil, nil); err == nil || !strings.Contains(err.Error(), "No profiles") {
		t.Fatalf("error = %v", err)
	}
	want := errors.New("select failed")
	deps := Dependencies{Interactive: true, Prompt: renameCoveragePrompt{selectErr: want}}
	if _, _, err := resolveIDs(deps, "profile", nil, []prompt.Choice{{Value: "old"}}); !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	want = errors.New("text failed")
	deps.Prompt = renameCoveragePrompt{selected: "old", textErr: want}
	if _, _, err := resolveIDs(deps, "profile", nil, []prompt.Choice{{Value: "old"}}); !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestRenameCommandsRejectInvalidInvocationAndCommitFailure(t *testing.T) {
	validStore := configuration.NewStore(filepath.Join(t.TempDir(), "config.toml"))
	if err := validStore.Save(renameCoverageConfig()); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("injected commit failure")
	base := Dependencies{
		Config:       validStore,
		Secrets:      &renameCoverageSecrets{},
		Accounts:     &renameCoverageAccounts{},
		Out:          &bytes.Buffer{},
		Synchronizer: synchronization.Synchronizer{Config: failingRenameConfigStore{err: failure}},
	}

	for _, test := range []struct {
		name string
		new  func(Dependencies) interface {
			SetArgs([]string)
			Execute() error
		}
		deps Dependencies
		args []string
		want string
	}{
		{name: "profile needs explicit IDs", new: func(deps Dependencies) interface {
			SetArgs([]string)
			Execute() error
		} {
			return NewProfileCommand(deps)
		}, deps: base, want: "requires <old> <new>"},
		{name: "profile interactive selection fails", new: func(deps Dependencies) interface {
			SetArgs([]string)
			Execute() error
		} {
			return NewProfileCommand(deps)
		}, deps: func() Dependencies {
			value := base
			value.Interactive = true
			value.Prompt = renameCoveragePrompt{selectErr: failure}
			return value
		}(), want: "Select profile"},
		{name: "profile commit fails", new: func(deps Dependencies) interface {
			SetArgs([]string)
			Execute() error
		} {
			return NewProfileCommand(deps)
		}, deps: base, args: []string{"codex", "renamed"}, want: failure.Error()},
		{name: "account finalize needs IDs", new: func(deps Dependencies) interface {
			SetArgs([]string)
			Execute() error
		} {
			return NewAccountCommand(deps)
		}, deps: base, args: []string{"--finalize", "old"}, want: "requires explicit"},
		{name: "account confirmation needs finalize", new: func(deps Dependencies) interface {
			SetArgs([]string)
			Execute() error
		} {
			return NewAccountCommand(deps)
		}, deps: base, args: []string{"old", "new", "--confirm-api-token-rotation"}, want: "require --finalize"},
		{name: "account interactive selection fails", new: func(deps Dependencies) interface {
			SetArgs([]string)
			Execute() error
		} {
			return NewAccountCommand(deps)
		}, deps: func() Dependencies {
			value := base
			value.Interactive = true
			value.Prompt = renameCoveragePrompt{selectErr: failure}
			return value
		}(), want: "Select account"},
		{name: "account commit fails", new: func(deps Dependencies) interface {
			SetArgs([]string)
			Execute() error
		} {
			return NewAccountCommand(deps)
		}, deps: base, args: []string{"old", "new"}, want: failure.Error()},
	} {
		t.Run(test.name, func(t *testing.T) {
			command := test.new(test.deps)
			command.SetArgs(test.args)
			if err := command.Execute(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Execute() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestRenameCommandsRejectUnreadableConfigurationAndInvalidPlans(t *testing.T) {
	for _, constructor := range []struct {
		name string
		new  func(Dependencies) interface {
			SetArgs([]string)
			Execute() error
		}
	}{
		{name: "profile", new: func(deps Dependencies) interface {
			SetArgs([]string)
			Execute() error
		} {
			return NewProfileCommand(deps)
		}},
		{name: "account", new: func(deps Dependencies) interface {
			SetArgs([]string)
			Execute() error
		} {
			return NewAccountCommand(deps)
		}},
	} {
		t.Run(constructor.name+" unreadable configuration", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "configuration.toml")
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
			command := constructor.new(Dependencies{Config: configuration.NewStore(path), Out: &bytes.Buffer{}})
			command.SetArgs([]string{"old", "new"})
			if err := command.Execute(); err == nil {
				t.Fatal("unreadable configuration was accepted")
			}
		})

		t.Run(constructor.name+" invalid plan", func(t *testing.T) {
			store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
			if err := store.Save(renameCoverageConfig()); err != nil {
				t.Fatal(err)
			}
			command := constructor.new(Dependencies{Config: store, Secrets: &renameCoverageSecrets{}, Accounts: &renameCoverageAccounts{}, Out: &bytes.Buffer{}})
			command.SetArgs([]string{"missing", "new"})
			if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "Unknown") {
				t.Fatalf("Execute() error = %v", err)
			}
		})
	}
}

func TestRenamePlanningValidationAndReferenceBranches(t *testing.T) {
	cfg := renameCoverageConfig()
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
	base, err := planAccount(renameCoverageConfig(), "old", "new")
	if err != nil {
		t.Fatal(err)
	}
	want := errors.New("read failed")
	tests := []struct {
		name     string
		secrets  *renameCoverageSecrets
		accounts *renameCoverageAccounts
	}{
		{name: "source token", secrets: &renameCoverageSecrets{getErrors: map[string]error{"old": want}}, accounts: &renameCoverageAccounts{}},
		{name: "target token", secrets: &renameCoverageSecrets{values: map[string]string{"old": "token"}, getErrors: map[string]error{"new": want}}, accounts: &renameCoverageAccounts{}},
		{name: "source probe", secrets: &renameCoverageSecrets{}, accounts: &renameCoverageAccounts{getErrors: map[string]error{"old": want}}},
		{name: "target probe", secrets: &renameCoverageSecrets{}, accounts: &renameCoverageAccounts{values: map[string]account.Credential{"old": {SystemToken: "s", UserID: "u"}}, getErrors: map[string]error{"new": want}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deps := Dependencies{Secrets: test.secrets, Accounts: test.accounts}
			if _, err := planCredentialCopies(deps, base); !errors.Is(err, want) {
				t.Fatalf("error = %v, want %v", err, want)
			}
		})
	}
}

func TestApplyAccountCredentialCopiesFailures(t *testing.T) {
	want := errors.New("operation failed")
	probe := account.Credential{SystemToken: "system", UserID: "user"}
	plan := Plan{OldID: "old", NewID: "new", tokenCopy: tokenCopy{value: "token", copy: true}}
	tests := []struct {
		name     string
		secrets  *renameCoverageSecrets
		accounts *renameCoverageAccounts
		plan     Plan
	}{
		{name: "token set", secrets: &renameCoverageSecrets{setErr: want}, accounts: &renameCoverageAccounts{}, plan: plan},
		{name: "token get", secrets: &renameCoverageSecrets{getErrors: map[string]error{"new": want}}, accounts: &renameCoverageAccounts{}, plan: plan},
		{name: "token differs", secrets: &renameCoverageSecrets{setReplacement: "other"}, accounts: &renameCoverageAccounts{}, plan: plan},
		{name: "probe set", secrets: &renameCoverageSecrets{}, accounts: &renameCoverageAccounts{setErr: want}, plan: Plan{NewID: "new", probeCopy: probeCopy{value: probe, copy: true}}},
		{name: "probe get", secrets: &renameCoverageSecrets{}, accounts: &renameCoverageAccounts{getErrors: map[string]error{"new": want}}, plan: Plan{NewID: "new", probeCopy: probeCopy{value: probe, copy: true}}},
		{name: "probe differs", secrets: &renameCoverageSecrets{}, accounts: &renameCoverageAccounts{setReplacement: account.Credential{SystemToken: "other", UserID: "other"}}, plan: Plan{NewID: "new", probeCopy: probeCopy{value: probe, copy: true}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := applyCredentialCopies(Dependencies{Secrets: test.secrets, Accounts: test.accounts}, test.plan)
			if err == nil {
				t.Fatal("expected copy failure")
			}
		})
	}
}

func TestPlanAccountFinalizeCredentialReadErrors(t *testing.T) {
	want := errors.New("read failed")
	probe := account.Credential{SystemToken: "system", UserID: "user"}
	tests := []struct {
		name     string
		secrets  *renameCoverageSecrets
		accounts *renameCoverageAccounts
	}{
		{name: "target token", secrets: &renameCoverageSecrets{getErrors: map[string]error{"new": want}}, accounts: &renameCoverageAccounts{}},
		{name: "source token", secrets: &renameCoverageSecrets{values: map[string]string{"new": "token"}, getErrors: map[string]error{"old": want}}, accounts: &renameCoverageAccounts{}},
		{name: "source probe", secrets: &renameCoverageSecrets{values: map[string]string{"new": "token"}}, accounts: &renameCoverageAccounts{getErrors: map[string]error{"old": want}}},
		{name: "target probe", secrets: &renameCoverageSecrets{values: map[string]string{"new": "token"}}, accounts: &renameCoverageAccounts{values: map[string]account.Credential{"old": probe}, getErrors: map[string]error{"new": want}}},
		{name: "target probe absent", secrets: &renameCoverageSecrets{values: map[string]string{"new": "token"}}, accounts: &renameCoverageAccounts{values: map[string]account.Credential{"old": probe}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store, _ := renameFinalizationState(t)
			deps := Dependencies{Config: store, Secrets: test.secrets, Accounts: test.accounts}
			if _, err := planFinalize(deps, "old", "new", FinalizeOptions{}); err == nil {
				t.Fatal("expected finalization planning failure")
			}
		})
	}
}

func TestPlanAccountFinalizeRejectsIncompleteRenameState(t *testing.T) {
	t.Run("source account remains", func(t *testing.T) {
		store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
		cfg := renameCoverageConfig()
		if err := store.Save(cfg); err != nil {
			t.Fatal(err)
		}
		if err := store.SaveVerifiedCheckpoint(cfg, configuration.AdmittedClientIDs()); err != nil {
			t.Fatal(err)
		}
		deps := Dependencies{Config: store, Secrets: &renameCoverageSecrets{}, Accounts: &renameCoverageAccounts{}}
		if _, err := planFinalize(deps, "old", "new", FinalizeOptions{}); err == nil || !strings.Contains(err.Error(), "still exists") {
			t.Fatalf("planFinalize() error = %v", err)
		}
	})

	t.Run("target account missing", func(t *testing.T) {
		store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
		cfg := renamedCoverageConfig()
		if err := store.Save(cfg); err != nil {
			t.Fatal(err)
		}
		if err := store.SaveVerifiedCheckpoint(cfg, configuration.AdmittedClientIDs()); err != nil {
			t.Fatal(err)
		}
		deps := Dependencies{Config: store, Secrets: &renameCoverageSecrets{}, Accounts: &renameCoverageAccounts{}}
		if _, err := planFinalize(deps, "old", "missing", FinalizeOptions{}); err == nil || !strings.Contains(err.Error(), "does not exist") {
			t.Fatalf("planFinalize() error = %v", err)
		}
	})

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
		secrets  *renameCoverageSecrets
		accounts *renameCoverageAccounts
	}{
		{name: "token delete", plan: Plan{OldID: "old", deleteToken: true}, secrets: &renameCoverageSecrets{deleteErr: want}, accounts: &renameCoverageAccounts{}},
		{name: "token verify read", plan: Plan{OldID: "old", deleteToken: true}, secrets: &renameCoverageSecrets{values: map[string]string{"old": "token"}, afterDeleteErr: want}, accounts: &renameCoverageAccounts{}},
		{name: "token remains", plan: Plan{OldID: "old", deleteToken: true}, secrets: &renameCoverageSecrets{values: map[string]string{"old": "token"}, retainDelete: true}, accounts: &renameCoverageAccounts{}},
		{name: "external read", plan: Plan{OldID: "old", externalTokenCleanup: true}, secrets: &renameCoverageSecrets{getErrors: map[string]error{"old": want}}, accounts: &renameCoverageAccounts{}},
		{name: "external remains", plan: Plan{OldID: "old", externalTokenCleanup: true}, secrets: &renameCoverageSecrets{values: map[string]string{"old": "token"}}, accounts: &renameCoverageAccounts{}},
		{name: "probe delete", plan: Plan{OldID: "old", deleteProbe: true}, secrets: &renameCoverageSecrets{}, accounts: &renameCoverageAccounts{deleteErr: want}},
		{name: "probe verify read", plan: Plan{OldID: "old", deleteProbe: true}, secrets: &renameCoverageSecrets{}, accounts: &renameCoverageAccounts{values: map[string]account.Credential{"old": {SystemToken: "s", UserID: "u"}}, afterDeleteErr: want}},
		{name: "probe remains", plan: Plan{OldID: "old", deleteProbe: true}, secrets: &renameCoverageSecrets{}, accounts: &renameCoverageAccounts{values: map[string]account.Credential{"old": {SystemToken: "s", UserID: "u"}}, retainDelete: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store, snapshot := renameFinalizationState(t)
			test.plan.snapshot = snapshot
			deps := Dependencies{Config: store, Secrets: test.secrets, Accounts: test.accounts}
			if _, err := applyFinalize(context.Background(), deps, test.plan); err == nil {
				t.Fatal("expected cleanup failure")
			}
		})
	}

	t.Run("backup convergence", func(t *testing.T) {
		_, snapshot := renameFinalizationState(t)
		deps := Dependencies{Config: configuration.NewStore(t.TempDir()), Secrets: &renameCoverageSecrets{}, Accounts: &renameCoverageAccounts{}}
		if _, err := applyFinalize(context.Background(), deps, Plan{snapshot: snapshot}); err == nil || !strings.Contains(err.Error(), "Converge") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestApplyFinalizeStopsBeforeCleanupWhenProbeVerificationFails(t *testing.T) {
	store, snapshot := renameFinalizationState(t)
	plan := Plan{NewID: "new", verifyProbe: true, snapshot: snapshot, Account: configuration.Account{Label: "New"}}
	deps := Dependencies{Config: store, Secrets: &renameCoverageSecrets{}, Accounts: &renameCoverageAccounts{}}
	if _, err := applyFinalize(context.Background(), deps, plan); err == nil || !strings.Contains(err.Error(), "does not declare precise diagnostics") {
		t.Fatalf("applyFinalize() error = %v", err)
	}
}

func TestVerifyFinalizedAccountProbeFailures(t *testing.T) {
	base := Plan{NewID: "new", Account: configuration.Account{Label: "New"}}
	deps := Dependencies{Secrets: &renameCoverageSecrets{}, Accounts: &renameCoverageAccounts{}}
	if err := verifyFinalizedAccountProbe(context.Background(), deps, base); err == nil || !strings.Contains(err.Error(), "does not declare") {
		t.Fatalf("error = %v", err)
	}

	base.Account.AccountProbe = &configuration.AccountProbe{Kind: "future", BaseURL: "https://new.test"}
	if err := verifyFinalizedAccountProbe(context.Background(), deps, base); err == nil || !strings.Contains(err.Error(), "not included") {
		t.Fatalf("error = %v", err)
	}

	base.Account.AccountProbe = &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://new.test"}
	want := errors.New("read failed")
	deps.Secrets = &renameCoverageSecrets{getErrors: map[string]error{"new": want}}
	if err := verifyFinalizedAccountProbe(context.Background(), deps, base); !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}

	deps.Secrets = &renameCoverageSecrets{values: map[string]string{"new": "token"}}
	deps.Accounts = &renameCoverageAccounts{getErrors: map[string]error{"new": want}}
	if err := verifyFinalizedAccountProbe(context.Background(), deps, base); !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}

	deps.Accounts = &renameCoverageAccounts{values: map[string]account.Credential{"new": {SystemToken: "s", UserID: "u"}}}
	deps.HTTP = coverageHTTP(func(*http.Request) (*http.Response, error) { return nil, want })
	if err := verifyFinalizedAccountProbe(context.Background(), deps, base); !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
}

func TestWriteRenameResultHumanStatuses(t *testing.T) {
	statuses := []Plan{
		{Resource: "account", OldID: "old", NewID: "new", Status: "blocked", Finalize: true, Account: configuration.Account{Label: "New"}, ExternalTODOs: []string{"external action"}},
		{Resource: "account", OldID: "old", NewID: "new", Status: "planned", Finalize: true, Account: configuration.Account{Label: "New"}},
		{Resource: "account", OldID: "old", NewID: "new", Status: "already-finalized", Finalize: true, Account: configuration.Account{Label: "New"}},
		{Resource: "account", OldID: "old", NewID: "new", Status: "finalized", Finalize: true, Account: configuration.Account{Label: "New"}},
		{Resource: "profile", OldID: "old", NewID: "new", Status: "planned", Profile: configuration.Profile{Account: "account"}, AffectedReferences: []string{"routes.default"}},
		{Resource: "account", OldID: "old", NewID: "new", Status: "applied", Account: configuration.Account{Label: "New"}, AffectedReferences: []string{"profiles.codex.account"}},
		{Resource: "profile", OldID: "old", NewID: "new", Status: "applied", Profile: configuration.Profile{Account: "account"}},
	}
	for index, plan := range statuses {
		t.Run(fmt.Sprintf("%d-%s", index, plan.Status), func(t *testing.T) {
			out := &bytes.Buffer{}
			if err := writeResult(Dependencies{Out: out}, plan, false); err != nil {
				t.Fatal(err)
			}
			if out.Len() == 0 {
				t.Fatal("empty result")
			}
		})
	}
}

type coverageHTTP func(*http.Request) (*http.Response, error)

func (do coverageHTTP) Do(request *http.Request) (*http.Response, error) { return do(request) }
