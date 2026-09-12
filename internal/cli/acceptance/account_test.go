package cli_test

import (
	"errors"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

func TestAccountEditValidation(t *testing.T) {
	t.Run("account edit requires change", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		if err := cli.Execute(app, []string{"account", "edit", "one"}); err == nil {
			t.Fatal("expected nothing-to-update error")
		}
	})

	t.Run("account edit load", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		app.Config = configuration.NewStore(t.TempDir())
		if err := cli.Execute(app, []string{"account", "edit", "one", "--label", "New"}); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("account edit unknown", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "m")
		err := cli.Execute(app, []string{"account", "edit", "missing", "--label", "New"})
		if err == nil || !strings.Contains(err.Error(), "Unknown account") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("account edit label and anthropic", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "m")
		if err := cli.Execute(app, []string{"account", "edit", "one", "--label", "Renamed", "--anthropic-url", "https://new.test/"}); err != nil {
			t.Fatal(err)
		}
		cfg, _ := app.Config.Load()
		if cfg.Accounts["one"].Label != "Renamed" || cfg.Accounts["one"].Endpoints.Anthropic != "https://new.test" {
			t.Fatalf("account = %#v", cfg.Accounts["one"])
		}
	})
}

func TestAccountEditUpdatesSharedEndpointWithoutProfileDuplication(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "gpt", "dmx", "DMXAPI", configuration.Endpoints{OpenAIResponses: "https://old.test/v1", Anthropic: "https://old.test"}, configuration.ClientCodex, "gpt-test")
	addAccountProfile(&cfg, "claude", "dmx", "DMXAPI", configuration.Endpoints{}, configuration.ClientClaude, "claude-test")
	cfg.Routes[configuration.ClientCodex] = "gpt"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"account", "edit", "dmx", "--openai-url", "https://new.test/v1"}); err != nil {
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

func TestAccountConnectValidationAndDependencyFailures(t *testing.T) {
	t.Run("non-interactive", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		if err := cli.Execute(app, []string{"account", "connect"}); err == nil || !strings.Contains(err.Error(), "interactive") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("config load", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		app.Interactive = true
		app.Config = configuration.NewStore(t.TempDir())
		if err := cli.Execute(app, []string{"account", "connect"}); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("unknown explicit account", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		app.Interactive = true
		saveProbeProfile(t, app.Config)
		if err := cli.Execute(app, []string{"account", "connect", "missing"}); err == nil || !strings.Contains(strings.ToLower(err.Error()), "unknown") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("no probe", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		app.Interactive = true
		saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt")
		if err := cli.Execute(app, []string{"account", "connect"}); err == nil || !strings.Contains(err.Error(), "does not support") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("unsupported probe", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		app.Interactive = true
		saveProbeProfile(t, app.Config)
		cfg, _ := app.Config.Load()
		providerAccount := cfg.Accounts["dmx"]
		providerAccount.AccountProbe.Kind = "future"
		cfg.Accounts["dmx"] = providerAccount
		if err := app.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		if err := cli.Execute(app, []string{"account", "connect"}); err == nil || !strings.Contains(err.Error(), "does not include") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("secret prompt", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		app.Interactive = true
		saveProbeProfile(t, app.Config)
		want := errors.New("cancelled")
		app.Prompt = &scriptedPrompt{secretErr: want}
		if err := cli.Execute(app, []string{"account", "connect"}); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("text prompt", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		app.Interactive = true
		saveProbeProfile(t, app.Config)
		app.Prompt = &scriptedPrompt{secrets: []string{"system-token"}}
		if err := cli.Execute(app, []string{"account", "connect"}); err == nil || !strings.Contains(err.Error(), "no text") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("credential write", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		app.Interactive = true
		saveProbeProfile(t, app.Config)
		want := errors.New("credential write failed")
		app.Prompt = &scriptedPrompt{secrets: []string{"system-token"}, texts: []string{"user"}}
		app.Accounts = &recordingCredentialStore[secrets.DiagnosticCredential]{backend: app.Accounts, setErr: want}
		if err := cli.Execute(app, []string{"account", "connect"}); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})
}

func TestAccountDisconnectBranches(t *testing.T) {
	t.Run("load", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		app.Config = configuration.NewStore(t.TempDir())
		if err := cli.Execute(app, []string{"account", "disconnect"}); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("unknown", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveProbeProfile(t, app.Config)
		if err := cli.Execute(app, []string{"account", "disconnect", "missing"}); err == nil || !strings.Contains(strings.ToLower(err.Error()), "unknown") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("delete failure", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveProbeProfile(t, app.Config)
		want := errors.New("delete failed")
		app.Accounts = &recordingCredentialStore[secrets.DiagnosticCredential]{backend: app.Accounts, deleteErr: want}
		if err := cli.Execute(app, []string{"account", "disconnect", "dmx"}); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("success", func(t *testing.T) {
		app, out, _, _, _ := testApp(t, "")
		saveProbeProfile(t, app.Config)
		store := app.Accounts
		_ = store.Set("dmx", secrets.DiagnosticCredential{SystemToken: "system", UserID: "user"})
		if err := cli.Execute(app, []string{"account", "disconnect", "dmx"}); err != nil {
			t.Fatal(err)
		}
		if accountCredentialExists(t, store, "dmx") || !strings.Contains(out.String(), "credentials were removed") {
			t.Fatalf("output=%q", out.String())
		}
	})
}
