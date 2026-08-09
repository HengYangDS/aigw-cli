package cli_test

import (
	configuration "aigw-cli/internal/configuration"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRouteResetValidationLoadAndSuccess(t *testing.T) {
	t.Run("invalid client", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		if err := execute(t, app, "route", "reset", "other"); err == nil {
			t.Fatal("expected client validation error")
		}
	})

	t.Run("load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Config = configuration.NewStore(t.TempDir())
		if err := execute(t, app, "route", "reset", "codex"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("success", func(t *testing.T) {
		app, out, _, _ := testApp(t, "")
		cfg := configuration.NewConfig()
		addAccountProfile(&cfg, "one", "one", "One", configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, configuration.Models{configuration.ClientCodex: "gpt"})
		cfg.Routes.Default = "one"
		cfg.Routes.Overrides[configuration.ClientCodex] = "one"
		if err := app.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		if err := execute(t, app, "route", "reset", "codex"); err != nil {
			t.Fatal(err)
		}
		got, _ := app.Config.Load()
		if _, ok := got.Routes.Overrides[configuration.ClientCodex]; ok || !strings.Contains(out.String(), "inherits") {
			t.Fatalf("routes=%#v output=%q", got.Routes, out.String())
		}
	})
}

func TestRouteListIsNarrowHumanRouteView(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "gpt", "team", "GPT", configuration.Endpoints{OpenAIResponses: "https://team.test/v1", Anthropic: "https://team.test"}, configuration.ClientCodex, configuration.Models{configuration.ClientCodex: "gpt-test"})
	addAccountProfile(&cfg, "claude", "team", "Claude", configuration.Endpoints{Anthropic: "https://team.test"}, configuration.ClientClaude, configuration.Models{configuration.ClientClaude: "claude-test"})
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Purpose: "Default coding", Account: "team", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-test"}}
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Purpose: "Independent review", Account: "team", Client: configuration.ClientClaude, Models: configuration.Models{configuration.ClientClaude: "claude-test"}}
	cfg.Routes.Default = "gpt"
	cfg.Routes.Overrides[configuration.ClientClaude] = "claude"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app, "route", "list"); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"Current routes", "Default route", "Default profile", "gpt", "Codex", "gpt · Inherits default", "Claude", "claude · Explicit override", "aigw use <profile> --for <claude|codex>"} {
		if !strings.Contains(text, want) {
			t.Fatalf("route list lacks %q:\n%s", want, text)
		}
	}
	for _, unwanted := range []string{"Account diagnostics", "Model profiles", "Client adapters"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("route list should not include operational status section %q:\n%s", unwanted, text)
		}
	}
}

func TestRouteListDoesNotMisstateIncompatibleInheritedRouteAsUsable(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "gpt", "team", "GPT", configuration.Endpoints{OpenAIResponses: "https://team.test/v1", Anthropic: "https://team.test"}, configuration.ClientCodex, configuration.Models{configuration.ClientCodex: "gpt-test"})
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "team", Client: configuration.ClientClaude, Models: configuration.Models{configuration.ClientClaude: "claude-test"}}
	cfg.Routes.Default = "gpt"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app, "route", "list"); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"Claude", "No Claude profile selected", "aigw use claude --for claude", "Next\n  aigw use claude --for claude"} {
		if !strings.Contains(text, want) {
			t.Fatalf("route list lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "Claude            gpt · Inherits default") {
		t.Fatalf("route list misrepresented incompatible inherited route:\n%s", text)
	}
}

func TestCheckProbesTheDefaultRouteInsteadOfAnOverride(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["claude-account"] = configuration.Account{Label: "Claude Gateway", Endpoints: configuration.Endpoints{Anthropic: "https://claude.test"}}
	cfg.Accounts["codex-account"] = configuration.Account{Label: "Codex Gateway", Endpoints: configuration.Endpoints{OpenAIResponses: "https://codex.test/v1"}}
	cfg.Profiles["claude-fable-5"] = configuration.Profile{Label: "Claude Fable 5", Account: "claude-account", Client: configuration.ClientClaude, Models: configuration.Models{configuration.ClientClaude: "claude-fable-5"}}
	cfg.Profiles["gpt-5.6-terra"] = configuration.Profile{Label: "GPT-5.6 Terra", Account: "codex-account", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-5.6-terra"}}
	cfg.Routes.Default = "claude-fable-5"
	cfg.Routes.Overrides[configuration.ClientCodex] = "gpt-5.6-terra"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("claude-account", "claude-token"); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("codex-account", "codex-token"); err != nil {
		t.Fatal(err)
	}
	var gotHost, gotAuthorization string
	app.HTTP.(*fakeHTTP).handler = func(req *http.Request) (*http.Response, error) {
		gotHost = req.URL.Host
		gotAuthorization = req.Header.Get("Authorization")
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":[]}`)), Request: req}, nil
	}
	if err := execute(t, app, "check"); err != nil {
		t.Fatal(err)
	}
	if gotHost != "claude.test" || gotAuthorization != "Bearer claude-token" {
		t.Fatalf("check probe host=%q authorization=%q, want default Claude route", gotHost, gotAuthorization)
	}
}

func TestStatusGuidesClientSpecificRouteInsteadOfBlankRepair(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMXAPI", Endpoints: configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1", Anthropic: "https://dmx.test"}}
	cfg.Profiles["gpt-5.6-sol"] = configuration.Profile{Label: "GPT", Account: "dmx", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-5.6-sol"}}
	cfg.Profiles["claude-fable-5"] = configuration.Profile{Label: "Claude", Account: "dmx", Client: configuration.ClientClaude, Models: configuration.Models{configuration.ClientClaude: "claude-fable-5"}}
	cfg.Routes.Default = "gpt-5.6-sol"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	if err := execute(t, app, "status"); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if strings.Contains(text, "Claude             ·") || strings.Contains(text, "aigw repair") {
		t.Fatalf("status should not show blank Claude route or misleading repair:\n%s", text)
	}
	for _, want := range []string{"Claude", "No Claude profile selected", "aigw use claude-fable-5 --for claude"} {
		if !strings.Contains(text, want) {
			t.Fatalf("status lacks %q:\n%s", want, text)
		}
	}
}

func TestTestCommandExplainsUnconfiguredStateBeforeResolvingRoutes(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	err := execute(t, app, "test", "--for", "claude")
	if err == nil {
		t.Fatal("test command unexpectedly succeeded")
	}
	text := out.String()
	for _, want := range []string{"Not configured", "No service profiles have been created.", "aigw setup"} {
		if !strings.Contains(text, want) {
			t.Fatalf("unconfigured test output lacks %q:\n%s", want, text)
		}
	}
	for _, unwanted := range []string{"Connectivity test", `unknown profile ""`} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("unconfigured test output retained %q:\n%s", unwanted, text)
		}
	}
}

func TestUseSetsDefaultOrClientOverride(t *testing.T) {
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
	if err := execute(t, app, "use", "two", "--for", "claude"); err != nil {
		t.Fatal(err)
	}
	got, _ := app.Config.Load()
	if got.Routes.Default != "one" || got.Routes.Overrides["claude"] != "two" {
		t.Fatalf("routes = %#v", got.Routes)
	}
	if err := execute(t, app, "use", "two", "--all"); err != nil {
		t.Fatal(err)
	}
	got, _ = app.Config.Load()
	if got.Routes.Default != "two" || len(got.Routes.Overrides) != 0 {
		t.Fatalf("all routes = %#v", got.Routes)
	}
}

func TestUseForClaudeDoesNotRequireOrRewriteCodexTargets(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1", Anthropic: "https://gateway.test"}}
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Account: "gateway", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-test"}}
	cfg.Profiles["claude-fable"] = configuration.Profile{Label: "Claude Fable", Account: "gateway", Client: configuration.ClientClaude, Models: configuration.Models{configuration.ClientClaude: "claude-fable"}}
	cfg.Profiles["claude-sonnet"] = configuration.Profile{Label: "Claude Sonnet", Account: "gateway", Client: configuration.ClientClaude, Models: configuration.Models{configuration.ClientClaude: "claude-sonnet"}}
	cfg.Routes.Default = "gpt"
	cfg.Routes.Overrides[configuration.ClientClaude] = "claude-fable"
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Targets: []string{filepath.Join(t.TempDir(), "unavailable-codex-configuration.toml")}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("gateway", "test-token"); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app, "use", "claude-sonnet", "--for", "claude"); err != nil {
		t.Fatalf("Claude-only route change touched Codex target: %v", err)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Routes.Default != "gpt" || got.Routes.Overrides[configuration.ClientClaude] != "claude-sonnet" {
		t.Fatalf("routes = %#v", got.Routes)
	}
}

func TestUseRollsBackRouteWhenAdapterSyncFails(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	dir := t.TempDir()
	target := filepath.Join(dir, "configuration.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, "", configuration.Models{})
	addAccountProfile(&cfg, "two", "two", "Two", configuration.Endpoints{OpenAIResponses: "https://two.test/v1"}, "", configuration.Models{})
	cfg.Routes.Default = "one"
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/missing/codex", Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "old-secret")
	_ = secretStore.Set("two", "new-secret")
	app.Runner = &failingRunner{err: errors.New("login failed"), remaining: 1}
	err := execute(t, app, "use", "two")
	if err == nil || !strings.Contains(err.Error(), "was rolled back") {
		t.Fatalf("error = %v", err)
	}
	got, _ := app.Config.Load()
	if got.Routes.Default != "one" {
		t.Fatalf("route was not rolled back: %#v", got.Routes)
	}
}

func TestUseCodexProfileOnSameAccountDoesNotRebindCredentials(t *testing.T) {
	app, _, secretStore, runner := testApp(t, "")
	target := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMX", Endpoints: configuration.Endpoints{OpenAIResponses: "https://example.test/v1"}}
	cfg.Profiles["sol"] = configuration.Profile{Label: "Sol", Account: "dmx", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-5.6-sol"}}
	cfg.Profiles["terra"] = configuration.Profile{Label: "Terra", Account: "dmx", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-5.6-terra"}}
	cfg.Routes.Default = "sol"
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/usr/local/bin/codex", Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "test-token"); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app, "use", "terra", "--for", "codex"); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("same-account model switch rebound credentials: %#v", runner.plans)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `model = "gpt-5.6-terra" # managed by AIGW`) {
		t.Fatalf("Codex model was not switched:\n%s", data)
	}
}

func TestTestCommandUsesAnthropicAPIKeyHeader(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "claude", "anthropic", "Anthropic", configuration.Endpoints{Anthropic: "https://example.test"}, "", configuration.Models{})
	cfg.Routes.Default = "claude"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("anthropic", "anthropic-test-secret"); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app, "test", "--for", "claude"); err != nil {
		t.Fatal(err)
	}
	httpClient := app.HTTP.(*fakeHTTP)
	if httpClient.headers.Get("X-Api-Key") != "anthropic-test-secret" {
		t.Fatalf("x-api-key header = %q", httpClient.headers.Get("X-Api-Key"))
	}
	if httpClient.headers.Get("Authorization") != "" {
		t.Fatalf("authorization header = %q", httpClient.headers.Get("Authorization"))
	}
	if strings.Contains(out.String(), "anthropic-test-secret") || strings.Contains(strings.ToLower(out.String()), "x-api-key") {
		t.Fatalf("credential leaked in output: %s", out.String())
	}
}

func TestVerifyClaudeUsesManagedProcessBoundary(t *testing.T) {
	app, out, secretStore, runner := testApp(t, "")
	claudeExecutable := executableFixture(t, "claude")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMX", Endpoints: configuration.Endpoints{Anthropic: "https://example.test"}}
	cfg.Profiles["claude-fable-5"] = configuration.Profile{Label: "Claude Fable", Account: "dmx", Client: configuration.ClientClaude, Models: configuration.Models{configuration.ClientClaude: "claude-fable-5"}}
	cfg.Routes.Default = "claude-fable-5"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: claudeExecutable}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "verify-token"); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app, "verify", "--for", "claude"); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 1 || runner.plans[0].Executable != claudeExecutable || !strings.Contains(strings.Join(runner.plans[0].Args, " "), "AIGW_OK") {
		t.Fatalf("Claude verify plan = %#v", runner.plans)
	}
	if runner.plans[0].Replace {
		t.Fatal("Claude verification must capture a child process instead of replacing AIGW")
	}
	if got := runner.plans[0].Args; len(got) < 7 || got[0] != "--safe-mode" || got[1] != "--disable-slash-commands" || got[2] != "--no-session-persistence" || got[3] != "--print" || got[4] != "--model" || got[5] != "claude-fable-5" {
		t.Fatalf("Claude verification must use an isolated safe-mode invocation with the routed model, got %#v", got)
	}
	if strings.Contains(out.String(), "verify-token") || !strings.Contains(out.String(), "Live protocol verification") {
		t.Fatalf("verify output = %s", out.String())
	}
}

func TestTestCommandUsesAccountTokenForRuntimeProfile(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMXAPI", Endpoints: configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Profiles["gpt-5.6-sol"] = configuration.Profile{Label: "GPT", Account: "dmx", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-5.6-sol"}}
	cfg.Routes.Default = "gpt-5.6-sol"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "account-token")
	if err := execute(t, app, "test", "--for", "codex"); err != nil {
		t.Fatal(err)
	}
	if app.HTTP.(*fakeHTTP).headers.Get("Authorization") != "Bearer account-token" {
		t.Fatalf("authorization header = %q", app.HTTP.(*fakeHTTP).headers.Get("Authorization"))
	}
	if strings.Contains(out.String(), "account-token") || !strings.Contains(out.String(), "gpt-5.6-sol") {
		t.Fatalf("test output = %s", out.String())
	}
}

func TestTestCommandUsesCodexModelsEndpointAndRejectsNotFound(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMXAPI", Endpoints: configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Profiles["gpt-5.6-sol"] = configuration.Profile{Label: "GPT", Account: "dmx", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-5.6-sol"}}
	cfg.Routes.Default = "gpt-5.6-sol"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "account-token")
	var gotPath string
	app.HTTP.(*fakeHTTP).handler = func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":[]}`)), Request: req}, nil
	}
	if err := execute(t, app, "test", "--for", "codex"); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v1/models" {
		t.Fatalf("codex test path = %q", gotPath)
	}

	app.HTTP.(*fakeHTTP).handler = func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader(`{"message":"not found"}`)), Request: req}, nil
	}
	err := execute(t, app, "test", "--for", "codex")
	if err == nil || !strings.Contains(err.Error(), "HTTP 404") {
		t.Fatalf("error = %v", err)
	}
}
