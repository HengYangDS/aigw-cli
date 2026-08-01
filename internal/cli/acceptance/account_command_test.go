package cli_test

import (
	"aigw-cli/internal/account"
	configuration "aigw-cli/internal/configuration"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdvancedProfileAndAccountValidationBranches(t *testing.T) {
	t.Run("profile add invalid id", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		err := execute(t, app, "profile", "add", "bad id", "--account", "one", "--for", "claude", "--model", "m")
		if err == nil || !strings.Contains(err.Error(), "Invalid profile ID") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("profile add load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Config = configuration.NewStore(t.TempDir())
		if err := execute(t, app, "profile", "add", "two", "--account", "one", "--for", "claude", "--model", "m"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("profile add duplicate", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, configuration.Models{configuration.ClientClaude: "m"})
		err := execute(t, app, "profile", "add", "one", "--account", "one", "--for", "claude", "--model", "m")
		if err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("profile add unknown account", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, configuration.Models{configuration.ClientClaude: "m"})
		err := execute(t, app, "profile", "add", "two", "--account", "missing", "--for", "claude", "--model", "m")
		if err == nil || !strings.Contains(err.Error(), "Unknown account") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("profile add default label", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, configuration.Models{configuration.ClientClaude: "m"})
		if err := execute(t, app, "profile", "add", "two", "--account", "one", "--for", "claude", "--model", "m2"); err != nil {
			t.Fatal(err)
		}
		cfg, _ := app.Config.Load()
		if cfg.Profiles["two"].Label != "two" {
			t.Fatalf("profile = %#v", cfg.Profiles["two"])
		}
	})

	t.Run("account edit requires change", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		if err := execute(t, app, "account", "edit", "one"); err == nil {
			t.Fatal("expected nothing-to-update error")
		}
	})

	t.Run("account edit load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Config = configuration.NewStore(t.TempDir())
		if err := execute(t, app, "account", "edit", "one", "--label", "New"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("account edit unknown", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, configuration.Models{configuration.ClientClaude: "m"})
		err := execute(t, app, "account", "edit", "missing", "--label", "New")
		if err == nil || !strings.Contains(err.Error(), "Unknown account") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("account edit label and anthropic", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, configuration.Models{configuration.ClientClaude: "m"})
		if err := execute(t, app, "account", "edit", "one", "--label", "Renamed", "--anthropic-url", "https://new.test/"); err != nil {
			t.Fatal(err)
		}
		cfg, _ := app.Config.Load()
		if cfg.Accounts["one"].Label != "Renamed" || cfg.Accounts["one"].Endpoints.Anthropic != "https://new.test" {
			t.Fatalf("account = %#v", cfg.Accounts["one"])
		}
	})
}

func TestConfigImportRefusesAccountConflictUntilExplicitReplacementAndPreservesToken(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["team"] = configuration.Account{Label: "Personal Gateway", Endpoints: configuration.Endpoints{Anthropic: "https://personal.example.test"}}
	cfg.Profiles["local"] = configuration.Profile{Label: "Local", Account: "team"}
	cfg.Routes.Default = "local"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("team", "personal-token"); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(t.TempDir(), "team.toml")
	manifest := `version = 3
recommended_default = "team-profile"
[accounts.team]
label = "Team Gateway"
[accounts.team.endpoints]
anthropic = "https://team.example.test"
[profiles.team-profile]
label = "Team Profile"
account = "team"
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	err := execute(t, app, "config", "import", manifestPath)
	if err == nil || !strings.Contains(err.Error(), "--replace-account team") {
		t.Fatalf("default import error = %v", err)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Accounts["team"].Endpoints.Anthropic != "https://personal.example.test" || got.Routes.Default != "local" {
		t.Fatalf("default import mutated local identity: %#v", got)
	}
	if token, err := secretStore.Get("team"); err != nil || token != "personal-token" {
		t.Fatalf("default import altered token: %q, %v", token, err)
	}

	if err := execute(t, app, "config", "import", manifestPath, "--replace-account", "team"); err != nil {
		t.Fatal(err)
	}
	got, err = app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Accounts["team"].Endpoints.Anthropic != "https://team.example.test" || got.Routes.Default != "local" {
		t.Fatalf("explicit replacement result: %#v", got)
	}
	if token, err := secretStore.Get("team"); err != nil || token != "personal-token" {
		t.Fatalf("explicit replacement altered token: %q, %v", token, err)
	}
}

func TestAdapterAuthBindsCurrentCodexAccount(t *testing.T) {
	app, _, secretStore, runner := testApp(t, "")
	target := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMX", Endpoints: configuration.Endpoints{OpenAIResponses: "https://example.test/v1"}}
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Account: "dmx", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-test"}}
	cfg.Routes.Default = "gpt"
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/codex-real", Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "rebind-token"); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app, "adapter", "auth", "codex"); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 1 || runner.plans[0].Stdin != "rebind-token\n" {
		t.Fatalf("auth plans = %#v", runner.plans)
	}
}

func TestConfigImportReportsMissingAccountTokensNotProfileTokens(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	manifestPath := filepath.Join(t.TempDir(), "team.toml")
	manifest := `version = 3
recommended_default = "gpt-long-model"
[accounts.dmx]
label = "DMXAPI"
[accounts.dmx.endpoints]
openai_responses = "https://dmx.test/v1"
[profiles."gpt-long-model"]
label = "GPT Long Model"
account = "dmx"
client = "codex"
[profiles."gpt-long-model".models]
codex = "gpt-long-model"
[profiles."claude-long-model"]
label = "Claude Long Model"
account = "dmx"
client = "claude"
[profiles."claude-long-model".models]
claude = "claude-long-model"
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "existing-token")
	if err := execute(t, app, "config", "import", manifestPath); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if strings.Contains(text, "Token required") || strings.Contains(text, "gpt-long-model  ") || strings.Contains(text, "claude-long-model  ") {
		t.Fatalf("import reported profile-level missing tokens despite account token:\n%s", text)
	}
	for _, want := range []string{"Accounts", "System secret", "dmx", "Token available", "aigw models"} {
		if !strings.Contains(text, want) {
			t.Fatalf("import output lacks %q:\n%s", want, text)
		}
	}
}

func TestConfigImportReportsOnlyMissingAccounts(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	manifestPath := filepath.Join(t.TempDir(), "team.toml")
	manifest := `version = 3
recommended_default = "gpt-long-model"
[accounts.dmx]
label = "DMXAPI"
[accounts.dmx.endpoints]
openai_responses = "https://dmx.test/v1"
[profiles."gpt-long-model"]
label = "GPT Long Model"
account = "dmx"
client = "codex"
[profiles."gpt-long-model".models]
codex = "gpt-long-model"
[profiles."claude-long-model"]
label = "Claude Long Model"
account = "dmx"
client = "claude"
[profiles."claude-long-model".models]
claude = "claude-long-model"
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, app, "config", "import", manifestPath); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "dmx") || !strings.Contains(text, "Token required") || !strings.Contains(text, "aigw rotate dmx") {
		t.Fatalf("import did not point to missing account token:\n%s", text)
	}
	if strings.Contains(text, "gpt-long-model") || strings.Contains(text, "claude-long-model") {
		t.Fatalf("import should not report profile names as missing token slots:\n%s", text)
	}
}

func TestProfileRenameKeepsAccountTokenSlotUnchanged(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMXAPI", Endpoints: configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Profiles["gpt-old"] = configuration.Profile{Label: "GPT Old", Account: "dmx", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-old"}}
	cfg.Routes.Default = "gpt-old"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "account-token")
	if err := execute(t, app, "profile", "rename", "gpt-old", "gpt-new"); err != nil {
		t.Fatal(err)
	}
	if got, err := secretStore.Get("dmx"); err != nil || got != "account-token" {
		t.Fatalf("account token changed: %q %v", got, err)
	}
	if secretStore.Has("gpt-new") || secretStore.Has("gpt-old") {
		t.Fatalf("profile rename created profile-level secret slots")
	}
	got, _ := app.Config.Load()
	if got.Routes.Default != "gpt-new" || got.Profiles["gpt-new"].Account != "dmx" {
		t.Fatalf("rename config = %#v", got)
	}
}

func TestProfileRemoveLeavesAccountAndTokenIntact(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMXAPI", Endpoints: configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Profiles["gpt-default"] = configuration.Profile{Label: "GPT Default", Account: "dmx", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-default"}}
	cfg.Profiles["gpt-unused"] = configuration.Profile{Label: "GPT Unused", Account: "dmx", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-unused"}}
	cfg.Routes.Default = "gpt-default"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "account-token")
	if err := execute(t, app, "profile", "remove", "gpt-unused"); err != nil {
		t.Fatal(err)
	}
	if got, err := secretStore.Get("dmx"); err != nil || got != "account-token" {
		t.Fatalf("account token changed: %q %v", got, err)
	}
	got, _ := app.Config.Load()
	if _, ok := got.Profiles["gpt-unused"]; ok || got.Accounts["dmx"].Label != "DMXAPI" {
		t.Fatalf("remove config = %#v", got)
	}
}

func TestProfileAddReusesAccountTokenAndLeavesRouteUntouched(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "gpt", "dmx", "GPT", configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1", Anthropic: "https://dmx.test"}, configuration.ClientCodex, configuration.Models{configuration.ClientCodex: "gpt-test"})
	cfg.Routes.Default = "gpt"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "existing-account-token"); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app, "profile", "add", "claude", "--account", "dmx", "--for", "claude", "--model", "claude-test", "--label", "Claude Test"); err != nil {
		t.Fatal(err)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	profile := got.Profiles["claude"]
	if profile.Account != "dmx" || profile.Client != configuration.ClientClaude || profile.Models[configuration.ClientClaude] != "claude-test" {
		t.Fatalf("added profile = %#v", profile)
	}
	if got.Routes.Default != "gpt" || !secretStore.Has("dmx") || secretStore.Has("claude") {
		t.Fatalf("route or token slots changed: routes=%#v dmx=%v claude=%v", got.Routes, secretStore.Has("dmx"), secretStore.Has("claude"))
	}
}

func TestAccountEditUpdatesSharedEndpointWithoutProfileDuplication(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "gpt", "dmx", "DMXAPI", configuration.Endpoints{OpenAIResponses: "https://old.test/v1", Anthropic: "https://old.test"}, configuration.ClientCodex, configuration.Models{configuration.ClientCodex: "gpt-test"})
	addAccountProfile(&cfg, "claude", "dmx", "DMXAPI", configuration.Endpoints{}, configuration.ClientClaude, configuration.Models{configuration.ClientClaude: "claude-test"})
	cfg.Routes.Default = "gpt"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app, "account", "edit", "dmx", "--openai-url", "https://new.test/v1"); err != nil {
		t.Fatal(err)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Accounts["dmx"].Endpoints.OpenAIResponses != "https://new.test/v1" {
		t.Fatalf("account endpoint = %#v", got.Accounts["dmx"])
	}
	for _, profile := range got.Profiles {
		if profile.Account != "dmx" {
			t.Fatalf("shared profile lost account reference: %#v", profile)
		}
	}
}

func TestProfileAddRejectsClientWithoutMatchingAccountEndpoint(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "gpt", "openai-only", "OpenAI Only", configuration.Endpoints{OpenAIResponses: "https://openai.test/v1"}, configuration.ClientCodex, configuration.Models{configuration.ClientCodex: "gpt-test"})
	cfg.Routes.Default = "gpt"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	err := execute(t, app, "profile", "add", "claude", "--account", "openai-only", "--for", "claude", "--model", "claude-test")
	if err == nil || !strings.Contains(err.Error(), "no Anthropic endpoint") {
		t.Fatalf("profile add error = %v", err)
	}
	got, loadErr := app.Config.Load()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if _, exists := got.Profiles["claude"]; exists {
		t.Fatalf("unusable Profile was persisted: %#v", got.Profiles["claude"])
	}
}

func TestAccountConnectValidationAndDependencyFailures(t *testing.T) {
	t.Run("non-interactive", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		if err := execute(t, app, "account", "connect"); err == nil || !strings.Contains(err.Error(), "interactive") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("config load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Interactive = true
		app.Config = configuration.NewStore(t.TempDir())
		if err := execute(t, app, "account", "connect"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("unknown explicit account", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Interactive = true
		saveProbeProfile(t, app.Config)
		if err := execute(t, app, "account", "connect", "missing"); err == nil || !strings.Contains(strings.ToLower(err.Error()), "unknown") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("no probe", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Interactive = true
		saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, configuration.Models{configuration.ClientCodex: "gpt"})
		if err := execute(t, app, "account", "connect"); err == nil || !strings.Contains(err.Error(), "does not support") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("unsupported probe", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Interactive = true
		saveProbeProfile(t, app.Config)
		cfg, _ := app.Config.Load()
		providerAccount := cfg.Accounts["dmx"]
		providerAccount.AccountProbe.Kind = "future"
		cfg.Accounts["dmx"] = providerAccount
		if err := app.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		if err := execute(t, app, "account", "connect"); err == nil || !strings.Contains(err.Error(), "does not include") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("secret prompt", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Interactive = true
		saveProbeProfile(t, app.Config)
		want := errors.New("cancelled")
		app.Prompt = &fakePrompt{secretErr: want}
		if err := execute(t, app, "account", "connect"); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("text prompt", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Interactive = true
		saveProbeProfile(t, app.Config)
		app.Prompt = &fakePrompt{secret: "system-token"}
		if err := execute(t, app, "account", "connect"); err == nil || !strings.Contains(err.Error(), "no text") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("credential write", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Interactive = true
		saveProbeProfile(t, app.Config)
		want := errors.New("credential write failed")
		app.Prompt = &fakePrompt{secret: "system-token", text: "user"}
		app.Accounts = failingAccountStore{Store: account.NewMemoryStore(), setErr: want}
		if err := execute(t, app, "account", "connect"); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})
}

func TestAccountDisconnectBranches(t *testing.T) {
	t.Run("load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Config = configuration.NewStore(t.TempDir())
		if err := execute(t, app, "account", "disconnect"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("unknown", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveProbeProfile(t, app.Config)
		if err := execute(t, app, "account", "disconnect", "missing"); err == nil || !strings.Contains(strings.ToLower(err.Error()), "unknown") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("delete failure", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveProbeProfile(t, app.Config)
		want := errors.New("delete failed")
		app.Accounts = failingAccountStore{Store: account.NewMemoryStore(), deleteErr: want}
		if err := execute(t, app, "account", "disconnect", "dmx"); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("success", func(t *testing.T) {
		app, out, _, _ := testApp(t, "")
		saveProbeProfile(t, app.Config)
		store := account.NewMemoryStore()
		_ = store.Set("dmx", account.Credential{SystemToken: "system", UserID: "user"})
		app.Accounts = store
		if err := execute(t, app, "account", "disconnect", "dmx"); err != nil {
			t.Fatal(err)
		}
		if store.Has("dmx") || !strings.Contains(out.String(), "credentials were removed") {
			t.Fatalf("output=%q has=%v", out.String(), store.Has("dmx"))
		}
	})
}

func TestBalanceOperationalAndRenderingBranches(t *testing.T) {
	t.Run("load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Config = configuration.NewStore(t.TempDir())
		if err := execute(t, app, "balance"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("unknown explicit account", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveProbeProfile(t, app.Config)
		if err := execute(t, app, "balance", "missing"); err == nil || !strings.Contains(strings.ToLower(err.Error()), "unknown") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("no probe", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, configuration.Models{configuration.ClientCodex: "gpt"})
		if err := execute(t, app, "balance"); err == nil || !strings.Contains(err.Error(), "does not support") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("missing api token", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveProbeProfile(t, app.Config)
		store := account.NewMemoryStore()
		_ = store.Set("dmx", account.Credential{SystemToken: "system", UserID: "user"})
		app.Accounts = store
		if err := execute(t, app, "balance"); err == nil {
			t.Fatal("expected missing API token")
		}
	})

	t.Run("provider failure", func(t *testing.T) {
		app, _, secretStore, _ := testApp(t, "")
		saveProbeProfile(t, app.Config)
		store := account.NewMemoryStore()
		_ = store.Set("dmx", account.Credential{SystemToken: "system", UserID: "user"})
		app.Accounts = store
		_ = secretStore.Set("dmx", "sk-abcd-middle-wxyz")
		want := errors.New("network failed")
		app.HTTP.(*fakeHTTP).handler = func(*http.Request) (*http.Response, error) { return nil, want }
		if err := execute(t, app, "balance"); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("disabled unlimited token", func(t *testing.T) {
		app, out, secretStore, _ := testApp(t, "")
		saveProbeProfile(t, app.Config)
		store := account.NewMemoryStore()
		_ = store.Set("dmx", account.Credential{SystemToken: "system", UserID: "user"})
		app.Accounts = store
		_ = secretStore.Set("dmx", "sk-abcd-middle-wxyz")
		app.HTTP.(*fakeHTTP).handler = func(request *http.Request) (*http.Response, error) {
			body := `{"success":true,"data":{"quota":6250000}}`
			if strings.Contains(request.URL.Path, "/api/token/search") {
				body = `{"success":true,"data":{"items":[{"name":"Codex","key":"abcd**********wxyz","status":2,"used_quota":1,"remain_quota":0,"unlimited_quota":true,"remain_count":0,"unlimited_count":true,"expired_time":-1}]}}`
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
		}
		if err := execute(t, app, "balance", "dmx"); err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"Disabled", "Unlimited", "Unlimited requests"} {
			if !strings.Contains(out.String(), want) {
				t.Fatalf("output lacks %q: %s", want, out.String())
			}
		}
	})
}

func TestRotateAccountNamePromptsWithAccountLabel(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMXAPI", Endpoints: configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Profiles["gpt-5.6-sol"] = configuration.Profile{Label: "GPT Profile", Account: "dmx", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-5.6-sol"}}
	cfg.Routes.Default = "gpt-5.6-sol"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "old-token")
	prompt := &fakePrompt{secret: "new-token"}
	app.Interactive = true
	app.Prompt = prompt
	if err := execute(t, app, "rotate", "dmx"); err != nil {
		t.Fatal(err)
	}
	if prompt.lastSecretLabel != "Paste DMXAPI token: " {
		t.Fatalf("prompt label = %q", prompt.lastSecretLabel)
	}
}

func TestCheckSuggestsAccountSpecificBalanceCommand(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMXAPI", Endpoints: configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"}, AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://www.dmxapi.cn"}}
	cfg.Profiles["gpt-5.6-sol"] = configuration.Profile{Label: "GPT", Account: "dmx", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-5.6-sol"}}
	cfg.Routes.Default = "gpt-5.6-sol"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	if err := execute(t, app, "check"); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "aigw account connect dmx") || !strings.Contains(text, "aigw balance dmx") {
		t.Fatalf("check should suggest account-specific diagnostics:\n%s", text)
	}
}

func TestStatusSuggestsAccountSpecificDiagnostics(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMXAPI", Endpoints: configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"}, AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://www.dmxapi.cn"}}
	cfg.Profiles["gpt-5.6-sol"] = configuration.Profile{Label: "GPT", Account: "dmx", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-5.6-sol"}}
	cfg.Routes.Default = "gpt-5.6-sol"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	if err := execute(t, app, "status"); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "aigw account connect dmx") {
		t.Fatalf("status should suggest account-specific diagnostics:\n%s", text)
	}
}

func TestBalanceExplainsWhenConfiguredDiagnosticDriverIsNotBundled(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["future"] = configuration.Account{
		Label:        "Future Gateway",
		Endpoints:    configuration.Endpoints{OpenAIResponses: "https://future.test/v1"},
		AccountProbe: &configuration.AccountProbe{Kind: "future-provider", BaseURL: "https://future.test"},
	}
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Account: "future", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-test"}}
	cfg.Routes.Default = "gpt"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("future", "test-token"); err != nil {
		t.Fatal(err)
	}
	err := execute(t, app, "balance")
	if err == nil || !strings.Contains(err.Error(), "is not included in this AIGW version") || !strings.Contains(err.Error(), "aigw check") {
		t.Fatalf("balance error = %v", err)
	}
}
