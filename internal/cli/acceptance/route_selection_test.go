package cli_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
)

func TestUseRejectsCombiningAllAndFor(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	err := execute(t, app, "use", "one", "--all", "--for", "codex")
	if err == nil || !strings.Contains(err.Error(), "--all and --for cannot be used together") {
		t.Fatalf("error = %v", err)
	}
}

func TestUseRejectsUnknownClientFlag(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	err := execute(t, app, "use", "one", "--for", "bogus")
	if err == nil || !strings.Contains(err.Error(), "--for must be claude or codex") {
		t.Fatalf("error = %v", err)
	}
}

func TestUseSurfacesConfigLoadFailure(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	app.Config = configuration.NewStore(t.TempDir())
	err := execute(t, app, "use", "one")
	if err == nil {
		t.Fatal("expected a config load failure")
	}
}

func TestUseWithoutProfileRequiresInteractiveTerminal(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", configuration.Endpoints{Anthropic: "https://one.test"}, "", configuration.Models{})
	cfg.Routes.Default = "one"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "one-secret")
	err := execute(t, app, "use")
	if err == nil || !strings.Contains(err.Error(), "Non-interactive use requires a profile") {
		t.Fatalf("error = %v", err)
	}
}

func TestUseWithoutProfilePromptsInteractively(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", configuration.Endpoints{Anthropic: "https://one.test"}, "", configuration.Models{})
	addAccountProfile(&cfg, "two", "two", "Two", configuration.Endpoints{Anthropic: "https://two.test"}, "", configuration.Models{})
	cfg.Routes.Default = "one"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "one-secret")
	_ = secretStore.Set("two", "two-secret")
	app.Interactive = true
	app.Prompt = &fakePrompt{selected: "two"}
	if err := execute(t, app, "use"); err != nil {
		t.Fatal(err)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Routes.Default != "two" {
		t.Fatalf("routes = %#v, want the interactively chosen profile", got.Routes)
	}
}

func TestUseSurfacesInteractiveSelectionFailure(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", configuration.Endpoints{Anthropic: "https://one.test"}, "", configuration.Models{})
	cfg.Routes.Default = "one"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "one-secret")
	app.Interactive = true
	want := errors.New("selection cancelled")
	app.Prompt = &fakePrompt{selectErr: want}
	err := execute(t, app, "use")
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestUseRejectsUnknownProfile(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", configuration.Endpoints{Anthropic: "https://one.test"}, "", configuration.Models{})
	cfg.Routes.Default = "one"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "one-secret")
	err := execute(t, app, "use", "does-not-exist")
	if err == nil || !strings.Contains(err.Error(), "Unknown profile") {
		t.Fatalf("error = %v", err)
	}
}

func TestUseSurfacesUnknownAccountReference(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["one"] = configuration.Account{Label: "One", Endpoints: configuration.Endpoints{Anthropic: "https://one.test"}}
	cfg.Profiles["one"] = configuration.Profile{Label: "One", Account: "one"}
	cfg.Routes.Default = "one"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "one-secret")
	// Store.Save validates referential integrity, so a dangling account
	// reference can only reach accountForInput through a file edited
	// outside AIGW (e.g. by hand or by another tool) after the fact.
	data, err := os.ReadFile(app.Config.Path())
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte("\n[profiles.broken]\nlabel = \"Broken\"\naccount = \"ghost\"\n")...)
	if err := os.WriteFile(app.Config.Path(), data, 0o600); err != nil {
		t.Fatal(err)
	}
	err = execute(t, app, "use", "broken")
	if err == nil || !strings.Contains(err.Error(), "references unknown account") {
		t.Fatalf("error = %v", err)
	}
}

func TestUseWithMissingTokenRequiresInteractiveTerminal(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", configuration.Endpoints{Anthropic: "https://one.test"}, "", configuration.Models{})
	addAccountProfile(&cfg, "two", "two", "Two", configuration.Endpoints{Anthropic: "https://two.test"}, "", configuration.Models{})
	cfg.Routes.Default = "one"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	err := execute(t, app, "use", "two")
	if err == nil || !strings.Contains(err.Error(), "is missing a token") {
		t.Fatalf("error = %v", err)
	}
}

func TestUseWithMissingTokenSurfacesPromptFailure(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", configuration.Endpoints{Anthropic: "https://one.test"}, "", configuration.Models{})
	addAccountProfile(&cfg, "two", "two", "Two", configuration.Endpoints{Anthropic: "https://two.test"}, "", configuration.Models{})
	cfg.Routes.Default = "one"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	app.Interactive = true
	app.Prompt = &fakePrompt{}
	err := execute(t, app, "use", "two")
	if err == nil || !strings.Contains(err.Error(), "no secret") {
		t.Fatalf("error = %v", err)
	}
}

func TestUseWithMissingTokenRejectsFailedVerification(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", configuration.Endpoints{Anthropic: "https://one.test"}, "", configuration.Models{})
	addAccountProfile(&cfg, "two", "two", "Two", configuration.Endpoints{Anthropic: "https://two.test"}, "", configuration.Models{})
	cfg.Routes.Default = "one"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "one-secret")
	app.Interactive = true
	app.Prompt = &fakePrompt{secret: "new-token"}
	app.HTTP.(*fakeHTTP).status = 401
	err := execute(t, app, "use", "two")
	if err == nil || !strings.Contains(err.Error(), "Token validation failed") {
		t.Fatalf("error = %v", err)
	}
	if secretStore.Has("two") {
		t.Fatal("a failed verification must not persist the newly entered token")
	}
}

func TestUseWithMissingTokenSurfacesSecretStoreFailure(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", configuration.Endpoints{Anthropic: "https://one.test"}, "", configuration.Models{})
	addAccountProfile(&cfg, "two", "two", "Two", configuration.Endpoints{Anthropic: "https://two.test"}, "", configuration.Models{})
	cfg.Routes.Default = "one"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	app.Interactive = true
	app.Prompt = &fakePrompt{secret: "new-token"}
	want := errors.New("keychain locked")
	app.Secrets = &failingSecretsStore{setErr: want}
	err := execute(t, app, "use", "two")
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}
