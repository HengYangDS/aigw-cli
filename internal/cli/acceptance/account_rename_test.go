package cli_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aigw-cli/internal/account"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

func TestAccountRenameInteractiveCopiesCredentialsAndUpdatesEveryProfile(t *testing.T) {
	app, out, secretStore, runner := testApp(t, "")
	cfg := accountRenameConfig()
	beforeRoutes := cfg.Routes
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	const token = "account-rename-source-token"
	probe := account.Credential{SystemToken: "account-rename-system-token", UserID: "account-rename-user"}
	if err := secretStore.Set("zeta-old", token); err != nil {
		t.Fatal(err)
	}
	if err := app.Accounts.Set("zeta-old", probe); err != nil {
		t.Fatal(err)
	}
	prompt := &fakePrompt{selected: "zeta-old", texts: []string{"zeta-new"}}
	app.Interactive = true
	app.Prompt = prompt

	if err := execute(t, app, "account", "rename"); err != nil {
		t.Fatal(err)
	}

	if got, want := choiceValues(prompt.choices), []string{"alpha", "zeta-old"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("account choices = %q, want %q", got, want)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Accounts["zeta-old"]; ok {
		t.Fatal("source account remains in configuration")
	}
	renamed, ok := got.Accounts["zeta-new"]
	if !ok || renamed.Label != "Zeta" || renamed.Endpoints != cfg.Accounts["zeta-old"].Endpoints || !reflect.DeepEqual(renamed.AccountProbe, cfg.Accounts["zeta-old"].AccountProbe) {
		t.Fatalf("renamed account = %#v, present = %v", renamed, ok)
	}
	for profileID, profile := range got.Profiles {
		if profile.Account != "zeta-new" {
			t.Fatalf("profile %q account = %q, want zeta-new", profileID, profile.Account)
		}
		if profile.Model != cfg.Profiles[profileID].Model || profile.Label != cfg.Profiles[profileID].Label {
			t.Fatalf("profile %q changed beyond its account reference: %#v", profileID, profile)
		}
	}
	if !reflect.DeepEqual(got.Routes, beforeRoutes) {
		t.Fatalf("routes changed: got %#v, want %#v", got.Routes, beforeRoutes)
	}
	for _, id := range []string{"zeta-old", "zeta-new"} {
		if gotToken, err := secretStore.Get(id); err != nil || gotToken != token {
			t.Fatalf("token slot %q = %q, %v", id, gotToken, err)
		}
		if gotProbe, err := app.Accounts.Get(id); err != nil || !reflect.DeepEqual(gotProbe, probe) {
			t.Fatalf("probe slot %q = %#v, %v", id, gotProbe, err)
		}
	}
	if len(runner.plans) != 0 {
		t.Fatalf("disabled adapters unexpectedly ran: %#v", runner.plans)
	}
	if strings.Contains(out.String(), token) || strings.Contains(out.String(), probe.SystemToken) || strings.Contains(out.String(), probe.UserID) {
		t.Fatalf("account rename leaked credentials: %s", out.String())
	}
	if !strings.Contains(out.String(), "aigw verify --for all") || !strings.Contains(out.String(), "aigw account rename zeta-old zeta-new --finalize") {
		t.Fatalf("account rename omitted safe next steps: %s", out.String())
	}
}

func TestAccountRenameDryRunJSONIsSecretFreeAndDoesNotWrite(t *testing.T) {
	app, out, secretStore, runner := testApp(t, "")
	if err := app.Config.Save(accountRenameConfig()); err != nil {
		t.Fatal(err)
	}
	const token = "account-rename-dry-run-token"
	probe := account.Credential{SystemToken: "account-rename-dry-run-system", UserID: "account-rename-dry-run-user"}
	if err := secretStore.Set("zeta-old", token); err != nil {
		t.Fatal(err)
	}
	if err := app.Accounts.Set("zeta-old", probe); err != nil {
		t.Fatal(err)
	}
	beforeConfig, err := os.ReadFile(app.Config.Path())
	if err != nil {
		t.Fatal(err)
	}
	beforeFiles := directoryNames(t, filepath.Dir(app.Config.Path()))
	out.Reset()

	if err := execute(t, app, "account", "rename", "zeta-old", "zeta-new", "--dry-run", "--json"); err != nil {
		t.Fatal(err)
	}

	var result struct {
		Resource           string            `json:"resource"`
		OldID              string            `json:"old_id"`
		NewID              string            `json:"new_id"`
		Status             string            `json:"status"`
		AffectedReferences []string          `json:"affected_references"`
		Actions            map[string]string `json:"actions"`
		ExternalTODOs      []string          `json:"external_todos"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode JSON output: %v\n%s", err, out.String())
	}
	if result.Resource != "account" || result.OldID != "zeta-old" || result.NewID != "zeta-new" || result.Status != "planned" {
		t.Fatalf("rename result = %#v", result)
	}
	wantReferences := []string{"profiles.claude-profile.account", "profiles.codex-profile.account"}
	if !reflect.DeepEqual(result.AffectedReferences, wantReferences) {
		t.Fatalf("affected references = %q, want %q", result.AffectedReferences, wantReferences)
	}
	if result.Actions["api_token"] != "copy-and-retain-source" || result.Actions["account_probe"] != "copy-and-retain-source" {
		t.Fatalf("credential actions = %#v", result.Actions)
	}
	if result.Actions["authentication"] != "unchanged" || result.Actions["backup"] != "refresh-on-apply" {
		t.Fatalf("transaction actions = %#v", result.Actions)
	}
	if result.ExternalTODOs == nil || len(result.ExternalTODOs) != 0 {
		t.Fatalf("external todos = %#v, want empty array", result.ExternalTODOs)
	}
	for _, forbidden := range []string{token, probe.SystemToken, probe.UserID, app.Config.Path()} {
		if strings.Contains(out.String(), forbidden) {
			t.Fatalf("dry-run output leaked %q: %s", forbidden, out.String())
		}
	}
	afterConfig, err := os.ReadFile(app.Config.Path())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(afterConfig, beforeConfig) {
		t.Fatal("dry-run changed configuration bytes")
	}
	if got := directoryNames(t, filepath.Dir(app.Config.Path())); !reflect.DeepEqual(got, beforeFiles) {
		t.Fatalf("dry-run changed config directory entries: before %q, after %q", beforeFiles, got)
	}
	if secretExists(t, secretStore, "zeta-new") || accountCredentialExists(t, app.Accounts, "zeta-new") {
		t.Fatal("dry-run created target credential slots")
	}
	if len(runner.plans) != 0 {
		t.Fatalf("dry-run invoked a client: %#v", runner.plans)
	}
}

func TestAccountRenameResumesWithEqualTargetCredentials(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	if err := app.Config.Save(accountRenameConfig()); err != nil {
		t.Fatal(err)
	}
	const token = "account-rename-equal-token"
	probe := account.Credential{SystemToken: "account-rename-equal-system", UserID: "account-rename-equal-user"}
	for _, id := range []string{"zeta-old", "zeta-new"} {
		if err := secretStore.Set(id, token); err != nil {
			t.Fatal(err)
		}
		if err := app.Accounts.Set(id, probe); err != nil {
			t.Fatal(err)
		}
	}

	if err := execute(t, app, "account", "rename", "zeta-old", "zeta-new"); err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{"zeta-old", "zeta-new"} {
		if got, err := secretStore.Get(id); err != nil || got != token {
			t.Fatalf("token slot %q = %q, %v", id, got, err)
		}
		if got, err := app.Accounts.Get(id); err != nil || !reflect.DeepEqual(got, probe) {
			t.Fatalf("probe slot %q = %#v, %v", id, got, err)
		}
	}
}

func TestAccountRenameRefusesInconsistentCredentialSlots(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T, *secrets.MemoryStore, account.Store)
	}{
		{
			name: "different API tokens",
			setup: func(t *testing.T, store *secrets.MemoryStore, _ account.Store) {
				if err := store.Set("zeta-old", "source-token"); err != nil {
					t.Fatal(err)
				}
				if err := store.Set("zeta-new", "different-target-token"); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "target token without source",
			setup: func(t *testing.T, store *secrets.MemoryStore, _ account.Store) {
				if err := store.Set("zeta-new", "target-only-token"); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "different account probe credentials",
			setup: func(t *testing.T, _ *secrets.MemoryStore, store account.Store) {
				if err := store.Set("zeta-old", account.Credential{SystemToken: "source-system", UserID: "same-user"}); err != nil {
					t.Fatal(err)
				}
				if err := store.Set("zeta-new", account.Credential{SystemToken: "different-system", UserID: "same-user"}); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "target account probe without source",
			setup: func(t *testing.T, _ *secrets.MemoryStore, store account.Store) {
				if err := store.Set("zeta-new", account.Credential{SystemToken: "target-system", UserID: "target-user"}); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, out, secretStore, _ := testApp(t, "")
			if err := app.Config.Save(accountRenameConfig()); err != nil {
				t.Fatal(err)
			}
			tt.setup(t, secretStore, app.Accounts)
			before, err := os.ReadFile(app.Config.Path())
			if err != nil {
				t.Fatal(err)
			}

			err = execute(t, app, "account", "rename", "zeta-old", "zeta-new", "--dry-run")
			if err == nil || !strings.Contains(err.Error(), "credential slot") {
				t.Fatalf("error = %v, want credential slot refusal", err)
			}
			after, readErr := os.ReadFile(app.Config.Path())
			if readErr != nil {
				t.Fatal(readErr)
			}
			if !reflect.DeepEqual(after, before) {
				t.Fatal("refused credential state changed configuration")
			}
			for _, forbidden := range []string{"source-token", "different-target-token", "target-only-token", "source-system", "different-system", "same-user", "target-system", "target-user"} {
				if strings.Contains(out.String(), forbidden) || strings.Contains(err.Error(), forbidden) {
					t.Fatalf("credential value %q leaked: output=%s error=%v", forbidden, out.String(), err)
				}
			}
		})
	}
}

func TestAccountRenameSupportsAccountWithoutCredentials(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	if err := app.Config.Save(accountRenameConfig()); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app, "account", "rename", "zeta-old", "zeta-new"); err != nil {
		t.Fatal(err)
	}

	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Accounts["zeta-new"]; !ok {
		t.Fatalf("renamed account missing: %#v", got.Accounts)
	}
	for _, id := range []string{"zeta-old", "zeta-new"} {
		if _, err := secretStore.Get(id); !errors.Is(err, secrets.ErrNotFound) {
			t.Fatalf("token slot %q error = %v, want not found", id, err)
		}
		if _, err := app.Accounts.Get(id); !errors.Is(err, account.ErrNotFound) {
			t.Fatalf("probe slot %q error = %v, want not found", id, err)
		}
	}
}

func TestAccountRenameEnvironmentTokenRequiresEqualTargetVariable(t *testing.T) {
	t.Run("equal target", func(t *testing.T) {
		app, out, _, _ := testApp(t, "")
		if err := app.Config.Save(accountRenameConfig()); err != nil {
			t.Fatal(err)
		}
		const token = "environment-account-rename-token"
		values := map[string]string{
			secrets.EnvironmentKey("zeta-old"): token,
			secrets.EnvironmentKey("zeta-new"): token,
		}
		app.Secrets = secrets.NewEnvironmentStore(func(key string) string { return values[key] })

		if err := execute(t, app, "account", "rename", "zeta-old", "zeta-new"); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), token) {
			t.Fatalf("environment token leaked: %s", out.String())
		}
	})

	t.Run("missing target", func(t *testing.T) {
		app, out, _, _ := testApp(t, "")
		if err := app.Config.Save(accountRenameConfig()); err != nil {
			t.Fatal(err)
		}
		const token = "environment-account-rename-source"
		app.Secrets = secrets.NewEnvironmentStore(func(key string) string {
			if key == secrets.EnvironmentKey("zeta-old") {
				return token
			}
			return ""
		})
		before, err := os.ReadFile(app.Config.Path())
		if err != nil {
			t.Fatal(err)
		}

		if err := execute(t, app, "account", "rename", "zeta-old", "zeta-new", "--dry-run", "--json"); err != nil {
			t.Fatal(err)
		}
		var result struct {
			Status        string            `json:"status"`
			Actions       map[string]string `json:"actions"`
			ExternalTODOs []string          `json:"external_todos"`
		}
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatalf("decode JSON output: %v\n%s", err, out.String())
		}
		if result.Status != "blocked" || result.Actions["api_token"] != "provide-equal-environment-variable" {
			t.Fatalf("blocked rename plan = %#v", result)
		}
		if len(result.ExternalTODOs) != 1 || !strings.Contains(result.ExternalTODOs[0], secrets.EnvironmentKey("zeta-new")) {
			t.Fatalf("external todos = %#v", result.ExternalTODOs)
		}
		if strings.Contains(out.String(), token) {
			t.Fatalf("environment token leaked: %s", out.String())
		}

		out.Reset()
		err = execute(t, app, "account", "rename", "zeta-old", "zeta-new")
		if err == nil || !strings.Contains(err.Error(), secrets.EnvironmentKey("zeta-new")) || strings.Contains(err.Error(), token) {
			t.Fatalf("apply error = %v", err)
		}
		after, readErr := os.ReadFile(app.Config.Path())
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !reflect.DeepEqual(after, before) {
			t.Fatal("blocked environment rename changed configuration")
		}
	})
}

func TestAccountRenameRefusesUnverifiedTokenCopy(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	if err := app.Config.Save(accountRenameConfig()); err != nil {
		t.Fatal(err)
	}
	const token = "account-rename-dropped-token"
	if err := secretStore.Set("zeta-old", token); err != nil {
		t.Fatal(err)
	}
	app.Secrets = droppingSecretStore{Store: secretStore}
	before, err := os.ReadFile(app.Config.Path())
	if err != nil {
		t.Fatal(err)
	}

	err = execute(t, app, "account", "rename", "zeta-old", "zeta-new")
	if err == nil || !strings.Contains(err.Error(), "verify target API token slot") {
		t.Fatalf("error = %v, want target verification failure", err)
	}
	after, readErr := os.ReadFile(app.Config.Path())
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatal("unverified token copy changed configuration")
	}
	if !secretExists(t, secretStore, "zeta-old") || secretExists(t, secretStore, "zeta-new") {
		t.Fatalf("token slots after failed copy: source=%v target=%v", secretExists(t, secretStore, "zeta-old"), secretExists(t, secretStore, "zeta-new"))
	}
	if strings.Contains(out.String(), token) || strings.Contains(err.Error(), token) {
		t.Fatalf("failed copy leaked token: output=%s error=%v", out.String(), err)
	}
}

func TestAccountRenameAuthenticationFailureRollsBackConfigAndRetainsBothCredentialSlots(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := accountRenameConfig()
	target := filepath.Join(t.TempDir(), "codex", "configuration.toml")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(app.Config.Path())
	if err != nil {
		t.Fatal(err)
	}
	const token = "account-rename-auth-token"
	probe := account.Credential{SystemToken: "account-rename-auth-system", UserID: "account-rename-auth-user"}
	if err := secretStore.Set("zeta-old", token); err != nil {
		t.Fatal(err)
	}
	if err := app.Accounts.Set("zeta-old", probe); err != nil {
		t.Fatal(err)
	}
	app.Runner = &failingRunner{err: errors.New("authentication refused"), remaining: 1}

	err = execute(t, app, "account", "rename", "zeta-old", "zeta-new")
	if err == nil || !strings.Contains(err.Error(), "was rolled back") {
		t.Fatalf("error = %v, want rolled-back authentication failure", err)
	}
	after, readErr := os.ReadFile(app.Config.Path())
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatal("authentication failure did not restore exact configuration bytes")
	}
	for _, id := range []string{"zeta-old", "zeta-new"} {
		if got, err := secretStore.Get(id); err != nil || got != token {
			t.Fatalf("token slot %q = %q, %v", id, got, err)
		}
		if got, err := app.Accounts.Get(id); err != nil || !reflect.DeepEqual(got, probe) {
			t.Fatalf("probe slot %q = %#v, %v", id, got, err)
		}
	}
	for _, forbidden := range []string{token, probe.SystemToken, probe.UserID} {
		if strings.Contains(out.String(), forbidden) || strings.Contains(err.Error(), forbidden) {
			t.Fatalf("authentication failure leaked %q: output=%s error=%v", forbidden, out.String(), err)
		}
	}
}

func TestAccountRenameNonCurrentCodexAccountDoesNotReauthenticate(t *testing.T) {
	app, _, secretStore, runner := testApp(t, "")
	cfg := accountRenameConfig()
	cfg.Accounts["active"] = configuration.Account{Label: "Active", Endpoints: configuration.Endpoints{OpenAIResponses: "https://active.test/v1"}}
	cfg.Profiles["active-profile"] = configuration.Profile{Label: "Active", Account: "active", Client: configuration.ClientCodex, Model: "active-model"}
	cfg.Routes[configuration.ClientCodex] = "active-profile"
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{filepath.Join(t.TempDir(), "codex", "configuration.toml")}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("zeta-old", "non-current-token"); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app, "account", "rename", "zeta-old", "zeta-new"); err != nil {
		t.Fatal(err)
	}

	if len(runner.plans) != 0 {
		t.Fatalf("non-current account rename invoked Codex authentication: %#v", runner.plans)
	}
}

func TestAccountRenameRefusesInvalidOrExistingTargetWithoutMutation(t *testing.T) {
	for _, target := range []string{"alpha", "Invalid Target"} {
		t.Run(target, func(t *testing.T) {
			app, _, secretStore, _ := testApp(t, "")
			if err := app.Config.Save(accountRenameConfig()); err != nil {
				t.Fatal(err)
			}
			if err := secretStore.Set("zeta-old", "source-token"); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(app.Config.Path())
			if err != nil {
				t.Fatal(err)
			}

			if err := execute(t, app, "account", "rename", "zeta-old", target, "--dry-run"); err == nil {
				t.Fatalf("rename to %q unexpectedly succeeded", target)
			}
			after, err := os.ReadFile(app.Config.Path())
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(after, before) || !secretExists(t, secretStore, "zeta-old") {
				t.Fatalf("refused target %q changed state", target)
			}
		})
	}
}

func TestAccountRenameNonInteractiveRequiresBothIDs(t *testing.T) {
	for _, args := range [][]string{{"account", "rename"}, {"account", "rename", "zeta-old"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			app, _, _, _ := testApp(t, "")
			if err := app.Config.Save(accountRenameConfig()); err != nil {
				t.Fatal(err)
			}
			err := execute(t, app, args...)
			if err == nil || !strings.Contains(err.Error(), "non-interactive") {
				t.Fatalf("error = %v, want explicit non-interactive guidance", err)
			}
		})
	}
}

type droppingSecretStore struct{ secrets.Store }

func (droppingSecretStore) Set(string, string) error { return nil }

func accountRenameConfig() configuration.Config {
	cfg := configuration.NewConfig()
	cfg.Accounts["alpha"] = configuration.Account{Label: "Alpha", Endpoints: configuration.Endpoints{OpenAIResponses: "https://alpha.test/v1"}}
	cfg.Accounts["zeta-old"] = configuration.Account{
		Label: "Zeta",
		Endpoints: configuration.Endpoints{
			OpenAIResponses: "https://zeta.test/v1",
			Anthropic:       "https://zeta.test/anthropic",
		},
		AccountProbe: &configuration.AccountProbe{Kind: "future-provider", BaseURL: "https://probe.zeta.test"},
	}
	cfg.Profiles["codex-profile"] = configuration.Profile{Label: "Codex", Purpose: "Codex purpose", Account: "zeta-old", Client: configuration.ClientCodex, Model: "codex-model"}
	cfg.Profiles["claude-profile"] = configuration.Profile{Label: "Claude", Purpose: "Claude purpose", Account: "zeta-old", Client: configuration.ClientClaude, Model: "claude-model"}
	cfg.Routes[configuration.ClientCodex] = "codex-profile"
	cfg.Routes[configuration.ClientClaude] = "claude-profile"
	return cfg
}
