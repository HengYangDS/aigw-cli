package cli_test

import (
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestRouteAddValidation(t *testing.T) {
	t.Run("route add invalid id", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		err := cli.Execute(app, []string{"route", "add", "bad id", "--account", "one", "--model", "m"})
		if err == nil || !strings.Contains(err.Error(), "Invalid route ID") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("route add load", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		app.Config = store.NewStore(t.TempDir(), clients.Admission())
		if err := cli.Execute(app, []string{"route", "add", "two", "--account", "one", "--model", "m"}); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("route add duplicate", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveCommandRoute(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "m")
		err := cli.Execute(app, []string{"route", "add", "one", "--account", "one", "--model", "m", "--protocol", "anthropic"})
		if err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("route add unknown account", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveCommandRoute(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "m")
		err := cli.Execute(app, []string{"route", "add", "two", "--account", "missing", "--model", "m", "--protocol", "anthropic"})
		if err == nil || !strings.Contains(err.Error(), "Unknown account") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("route add default label", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveCommandRoute(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "m")
		if err := cli.Execute(app, []string{"route", "add", "two", "--account", "one", "--model", "m2", "--protocol", "anthropic"}); err != nil {
			t.Fatal(err)
		}
		cfg, _ := app.Config.Load()
		if cfg.Routes["two"].Label != "two" {
			t.Fatalf("route = %#v", cfg.Routes["two"])
		}
	})
}

func TestRouteRemoveLeavesAccountAndTokenIntact(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["dmx"] = domain.Account{Label: "DMXAPI", Endpoints: domain.Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Routes["gpt-default"] = qualifiedRoute("GPT Default", "dmx", "gpt-default", domain.ProtocolOpenAIResponses)
	cfg.Routes["gpt-unused"] = qualifiedRoute("GPT Unused", "dmx", "gpt-unused", domain.ProtocolOpenAIResponses)
	cfg.SetSelectedRoute(codex.ID, "gpt-default", "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "account-token")
	if err := cli.Execute(app, []string{"route", "remove", "gpt-unused"}); err != nil {
		t.Fatal(err)
	}
	if got, err := secretStore.Get("dmx"); err != nil || got != "account-token" {
		t.Fatalf("account token changed: %q %v", got, err)
	}
	got, _ := app.Config.Load()
	if _, ok := got.Routes["gpt-unused"]; ok || got.Accounts["dmx"].Label != "DMXAPI" {
		t.Fatalf("remove config = %#v", got)
	}
}

func TestRouteAddReusesAccountTokenAndLeavesRouteUntouched(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	addAccountRoute(&cfg, "gpt", "dmx", "GPT", domain.Endpoints{OpenAIResponses: "https://dmx.test/v1", Anthropic: "https://dmx.test"}, codex.ID, "gpt-test")
	cfg.SetSelectedRoute(codex.ID, "gpt", "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "existing-account-token"); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"route", "add", "claude", "--account", "dmx", "--model", "claude-test", "--protocol", "anthropic", "--label", "Claude Test"}); err != nil {
		t.Fatal(err)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	route := got.Routes["claude"]
	if route.Account != "dmx" || route.Model != "claude-test" {
		t.Fatalf("added route = %#v", route)
	}
	if got.SelectedRoute(codex.ID) != "gpt" || !secretExists(t, secretStore, "dmx") || secretExists(t, secretStore, "claude") {
		t.Fatalf("selection or token slots changed: clients=%#v dmx=%v claude=%v", got.Clients, secretExists(t, secretStore, "dmx"), secretExists(t, secretStore, "claude"))
	}
}

func TestRouteAddDoesNotConflateRouteCreationWithClientCompatibility(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	addAccountRoute(&cfg, "gpt", "openai-only", "OpenAI Only", domain.Endpoints{OpenAIResponses: "https://openai.test/v1"}, codex.ID, "gpt-test")
	cfg.SetSelectedRoute(codex.ID, "gpt", "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"route", "add", "claude", "--account", "openai-only", "--model", "claude-test", "--protocol", "openai_responses"}); err != nil {
		t.Fatalf("route creation should remain client-independent: %v", err)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	route, exists := got.Routes["claude"]
	if !exists || route.Account != "openai-only" || route.Model != "claude-test" {
		t.Fatalf("created Route = %#v, exists=%v", route, exists)
	}
	if got.SelectedRoute(claude.ID) != "" {
		t.Fatalf("route creation selected a Claude binding: %#v", got.Clients)
	}

	err = cli.Execute(app, []string{"use", "--for", "claude", "claude"})
	if err == nil || !strings.Contains(err.Error(), `does not admit endpoint protocol "anthropic"`) {
		t.Fatalf("client selection error = %v", err)
	}
}

func TestRouteReadsDoNotInspectCredentials(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	saveCommandRoute(t, app, domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, codex.ID, "gpt")
	observed := &recordingCredentialStore[string]{backend: secret.NewMemoryStore(), existsErr: errors.New("credential observation must not run")}
	app.Secrets = observed

	for _, args := range [][]string{{"route", "list"}, {"route", "show", "one"}} {
		if err := cli.Execute(app, args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	if len(observed.existsCalls)+len(observed.getCalls) != 0 {
		t.Fatalf("Route reads inspected credentials: exists=%q get=%q", observed.existsCalls, observed.getCalls)
	}
}

func TestRouteReadsKeepClientAuthenticationInTheBinding(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["native"] = domain.Account{
		Label: "Native",
		Endpoints: domain.Endpoints{
			OpenAIResponses: "https://native.test/v1",
		},
	}
	cfg.Routes["native"] = domain.Route{
		Label:   "Native",
		Account: "native",
		Model:   "native-model",
		Interfaces: map[domain.EndpointProtocol][]domain.Capability{
			domain.ProtocolOpenAIResponses: {},
		},
	}
	cfg.Clients[codex.ID] = domain.ClientBinding{
		Route: "native", ModelProvider: "amazon-bedrock",
		Authentication: domain.AuthenticationClientNative,
	}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	observed := &recordingCredentialStore[string]{backend: secretStore}
	app.Secrets = observed

	if err := cli.Execute(app, []string{"route", "list"}); err != nil {
		t.Fatal(err)
	}
	if len(observed.existsCalls) != 0 || len(observed.getCalls) != 0 {
		t.Fatalf("route list accessed client-native credentials: exists=%q get=%q", observed.existsCalls, observed.getCalls)
	}
	if text := out.String(); strings.Contains(text, "Authentication") || strings.Contains(text, "Token") {
		t.Fatalf("route list = %q", text)
	}

	out.Reset()
	if err := cli.Execute(app, []string{"route", "show", "native", "--json"}); err != nil {
		t.Fatal(err)
	}
	if len(observed.existsCalls) != 0 || len(observed.getCalls) != 0 {
		t.Fatalf("route show accessed client-native credentials: exists=%q get=%q", observed.existsCalls, observed.getCalls)
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	for _, clientField := range []string{"authentication", "model_provider", "secret_available", "credential_ownership"} {
		if _, exists := result[clientField]; exists {
			t.Fatalf("Route JSON contains client-binding field %q: %s", clientField, out.String())
		}
	}

	out.Reset()
	if err := cli.Execute(app, []string{"route", "show", "native"}); err != nil {
		t.Fatal(err)
	}
	if text := out.String(); strings.Contains(text, "Authentication") || strings.Contains(text, "Token") {
		t.Fatalf("route show = %q", text)
	}
}

func TestRouteListJSONIsStableAndClientIndependent(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["shared"] = domain.Account{Label: "Shared", Endpoints: domain.Endpoints{OpenAIResponses: "https://shared.test/v1", Anthropic: "https://shared.test"}}
	cfg.Routes["zeta"] = qualifiedRoute("Zeta", "shared", "claude-test", domain.ProtocolAnthropic)
	cfg.Routes["alpha"] = qualifiedRoute("Alpha", "shared", "gpt-test", domain.ProtocolOpenAIResponses)
	cfg.Clients[codex.ID] = domain.ClientBinding{
		Route: "alpha", ModelProvider: "amazon-bedrock",
		Authentication: domain.AuthenticationClientNative,
	}
	cfg.SetSelectedRoute(claude.ID, "zeta", "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("shared", "never-print-this-token"); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"route", "list", "--json"}); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Routes []struct {
			ID              string   `json:"id"`
			SelectedClients []string `json:"selected_clients"`
		} `json:"routes"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Routes) != 2 || result.Routes[0].ID != "zeta" || result.Routes[1].ID != "alpha" {
		t.Fatalf("route order = %#v", result.Routes)
	}
	if !slices.Equal(result.Routes[0].SelectedClients, []string{claude.ID}) {
		t.Fatalf("Claude-selected Route = %#v", result.Routes[0])
	}
	if !slices.Equal(result.Routes[1].SelectedClients, []string{codex.ID}) {
		t.Fatalf("Codex-selected Route = %#v", result.Routes[1])
	}
	for _, forbidden := range []string{"never-print-this-token", "authentication", "credential_ownership", "secret_available"} {
		if strings.Contains(out.String(), forbidden) {
			t.Fatalf("route list exposed client or credential field %q: %s", forbidden, out.String())
		}
	}
}

func TestRouteShowRendersEverySecretFreeField(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["shared"] = domain.Account{Label: "Shared", Endpoints: domain.Endpoints{OpenAIResponses: "https://shared.test/v1", Anthropic: "https://shared.test"}}
	cfg.Routes["codex"] = qualifiedRoute("Codex Model", "shared", "gpt-test", domain.ProtocolOpenAIResponses)
	route := cfg.Routes["codex"]
	route.Purpose = "Daily work"
	cfg.Routes["codex"] = route
	cfg.SetSelectedRoute(codex.ID, "codex", "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("shared", "never-render-this"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"route", "show", "codex"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Codex Model", "Daily work", "shared", "codex", "gpt-test", "https://shared.test/v1", "https://shared.test"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("human output lacks %q:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "never-render-this") {
		t.Fatalf("human output leaked token: %s", out.String())
	}
	out.Reset()
	if err := cli.Execute(app, []string{"route", "show", "codex", "--json"}); err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if _, exists := result["secret_available"]; exists || strings.Contains(out.String(), "never-render-this") {
		t.Fatalf("JSON output = %s", out.String())
	}
}

func TestAdvancedRouteReadEditAndRemoveErrors(t *testing.T) {
	t.Run("list load", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		app.Config = store.NewStore(t.TempDir(), clients.Admission())
		if err := cli.Execute(app, []string{"route", "list"}); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("show load", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		app.Config = store.NewStore(t.TempDir(), clients.Admission())
		if err := cli.Execute(app, []string{"route", "show", "one"}); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("show unknown", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		err := cli.Execute(app, []string{"route", "show", "missing"})
		if err == nil || !strings.Contains(err.Error(), "Unknown route") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("edit requires change", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		if err := cli.Execute(app, []string{"route", "edit", "one"}); err == nil {
			t.Fatal("expected nothing-to-update error")
		}
	})

	t.Run("edit load", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		app.Config = store.NewStore(t.TempDir(), clients.Admission())
		if err := cli.Execute(app, []string{"route", "edit", "one", "--label", "New"}); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("edit unknown", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveCommandRoute(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "m")
		err := cli.Execute(app, []string{"route", "edit", "missing", "--label", "New"})
		if err == nil || !strings.Contains(err.Error(), "Unknown route") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("remove load", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		app.Config = store.NewStore(t.TempDir(), clients.Admission())
		if err := cli.Execute(app, []string{"route", "remove", "one"}); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("remove unknown", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveCommandRoute(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "m")
		err := cli.Execute(app, []string{"route", "remove", "missing"})
		if err == nil || !strings.Contains(err.Error(), "Unknown route") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("remove override", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		cfg := domain.NewConfig(clients.Admission())
		addAccountRoute(&cfg, "one", "one", "One", domain.Endpoints{OpenAIResponses: "https://one.test/v1", Anthropic: "https://one.test"}, claude.ID, "m1")
		addAccountRoute(&cfg, "two", "one", "Two", domain.Endpoints{}, codex.ID, "m2")
		cfg.SetSelectedRoute(claude.ID, "one", "")
		cfg.SetSelectedRoute(codex.ID, "two", "")
		if err := app.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		err := cli.Execute(app, []string{"route", "remove", "two"})
		if err == nil || !strings.Contains(err.Error(), "selected for codex") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestRouteEditSynchronizesActiveCodexProjection(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	target := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig(clients.Admission())
	addAccountRoute(&cfg, "one", "one", "One", domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, codex.ID, "gpt")
	cfg.SetSelectedRoute(codex.ID, "one", "")
	cfg.SetClientActivation(codex.ID, true, "/opt/codex", []string{target})
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "token")
	if err := cli.Execute(app, []string{"route", "edit", "one", "--label", "New Label"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Route metadata saved") {
		t.Fatalf("output = %q", out.String())
	}
	projected, err := os.ReadFile(target)
	if err != nil || !strings.Contains(string(projected), "AIGW: New Label") {
		t.Fatalf("route label was not projected: %s, %v", projected, err)
	}
}

func TestRouteRemoveRefusesActiveRoute(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	addAccountRoute(&cfg, "team", "team", "Team", domain.Endpoints{Anthropic: "https://team.test"}, claude.ID, "claude-test")
	cfg.SetSelectedRoute(claude.ID, "team", "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	err := cli.Execute(app, []string{"route", "remove", "team"})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "selected for claude") {
		t.Fatalf("error = %v", err)
	}
}
