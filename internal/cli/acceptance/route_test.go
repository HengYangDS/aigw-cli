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

func TestModelAddValidation(t *testing.T) {
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
		saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "m")
		err := cli.Execute(app, []string{"route", "add", "one", "--account", "one", "--model", "m", "--protocol", "anthropic"})
		if err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("route add unknown account", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "m")
		err := cli.Execute(app, []string{"route", "add", "two", "--account", "missing", "--model", "m", "--protocol", "anthropic"})
		if err == nil || !strings.Contains(err.Error(), "Unknown account") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("route add default label", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "m")
		if err := cli.Execute(app, []string{"route", "add", "two", "--account", "one", "--model", "m2", "--protocol", "anthropic"}); err != nil {
			t.Fatal(err)
		}
		cfg, _ := app.Config.Load()
		if cfg.Models["two"].Label != "two" {
			t.Fatalf("route = %#v", cfg.Models["two"])
		}
	})
}

func TestModelRemoveLeavesAccountAndTokenIntact(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["dmx"] = domain.Provider{Label: "DMXAPI", Endpoints: domain.Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Models["gpt-default"] = qualifiedModel("GPT Default", "dmx", "gpt-default", domain.ProtocolOpenAIResponses)
	cfg.Models["gpt-unused"] = qualifiedModel("GPT Unused", "dmx", "gpt-unused", domain.ProtocolOpenAIResponses)
	cfg.SetSelectedModel(codex.ID, "gpt-default", "")
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
	if _, ok := got.Models["gpt-unused"]; ok || got.Providers["dmx"].Label != "DMXAPI" {
		t.Fatalf("remove config = %#v", got)
	}
}

func TestModelAddReusesAccountTokenAndLeavesModelUntouched(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	addAccountModel(&cfg, "gpt", "dmx", "GPT", domain.Endpoints{OpenAIResponses: "https://dmx.test/v1", Anthropic: "https://dmx.test"}, codex.ID, "gpt-test")
	cfg.SetSelectedModel(codex.ID, "gpt", "")
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
	route := got.Models["claude"]
	if route.Provider != "dmx" || route.Family != "claude-test" {
		t.Fatalf("added route = %#v", route)
	}
	if got.SelectedModel(codex.ID) != "gpt" || !secretExists(t, secretStore, "dmx") || secretExists(t, secretStore, "claude") {
		t.Fatalf("selection or token slots changed: clients=%#v dmx=%v claude=%v", got.Clients, secretExists(t, secretStore, "dmx"), secretExists(t, secretStore, "claude"))
	}
}

func TestModelAddDoesNotConflateModelCreationWithClientCompatibility(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	addAccountModel(&cfg, "gpt", "openai-only", "OpenAI Only", domain.Endpoints{OpenAIResponses: "https://openai.test/v1"}, codex.ID, "gpt-test")
	cfg.SetSelectedModel(codex.ID, "gpt", "")
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
	route, exists := got.Models["claude"]
	if !exists || route.Provider != "openai-only" || route.Family != "claude-test" {
		t.Fatalf("created Route = %#v, exists=%v", route, exists)
	}
	if got.SelectedModel(claude.ID) != "" {
		t.Fatalf("route creation selected a Claude binding: %#v", got.Clients)
	}

	err = cli.Execute(app, []string{"use", "--for", "claude", "claude"})
	if err == nil || !strings.Contains(err.Error(), `does not admit endpoint protocol "anthropic"`) {
		t.Fatalf("client selection error = %v", err)
	}
}

func TestModelReadsDoNotInspectCredentials(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	saveCommandModel(t, app, domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, codex.ID, "gpt")
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

func TestModelReadsKeepClientAuthenticationInTheBinding(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["native"] = domain.Provider{
		Label: "Native",
		Endpoints: domain.Endpoints{
			OpenAIResponses: "https://native.test/v1",
		},
	}
	cfg.Models["native"] = domain.Model{
		Label:    "Native",
		Provider: "native",
		Family:   "native-model",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{
			domain.ProtocolOpenAIResponses: {},
		},
	}
	cfg.Clients[codex.ID] = domain.ClientBinding{
		Model: "native", ModelProvider: "amazon-bedrock",
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

func TestModelListJSONIsStableAndClientIndependent(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["shared"] = domain.Provider{Label: "Shared", Endpoints: domain.Endpoints{OpenAIResponses: "https://shared.test/v1", Anthropic: "https://shared.test"}}
	cfg.Models["zeta"] = qualifiedModel("Zeta", "shared", "claude-test", domain.ProtocolAnthropic)
	cfg.Models["alpha"] = qualifiedModel("Alpha", "shared", "gpt-test", domain.ProtocolOpenAIResponses)
	cfg.Clients[codex.ID] = domain.ClientBinding{
		Model: "alpha", ModelProvider: "amazon-bedrock",
		Authentication: domain.AuthenticationClientNative,
	}
	cfg.SetSelectedModel(claude.ID, "zeta", "")
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

func TestModelShowRendersEverySecretFreeField(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["shared"] = domain.Provider{Label: "Shared", Endpoints: domain.Endpoints{OpenAIResponses: "https://shared.test/v1", Anthropic: "https://shared.test"}}
	cfg.Models["codex"] = qualifiedModel("Codex Model", "shared", "gpt-test", domain.ProtocolOpenAIResponses)
	route := cfg.Models["codex"]
	route.Purpose = "Daily work"
	cfg.Models["codex"] = route
	cfg.SetSelectedModel(codex.ID, "codex", "")
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

func TestAdvancedModelReadEditAndRemoveErrors(t *testing.T) {
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
		saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "m")
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
		saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "m")
		err := cli.Execute(app, []string{"route", "remove", "missing"})
		if err == nil || !strings.Contains(err.Error(), "Unknown route") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("remove override", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		cfg := domain.NewConfig(clients.Admission())
		addAccountModel(&cfg, "one", "one", "One", domain.Endpoints{OpenAIResponses: "https://one.test/v1", Anthropic: "https://one.test"}, claude.ID, "m1")
		addAccountModel(&cfg, "two", "one", "Two", domain.Endpoints{}, codex.ID, "m2")
		cfg.SetSelectedModel(claude.ID, "one", "")
		cfg.SetSelectedModel(codex.ID, "two", "")
		if err := app.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		err := cli.Execute(app, []string{"route", "remove", "two"})
		if err == nil || !strings.Contains(err.Error(), "selected for codex") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestModelEditSynchronizesActiveCodexProjection(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	target := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig(clients.Admission())
	addAccountModel(&cfg, "one", "one", "One", domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, codex.ID, "gpt")
	cfg.SetSelectedModel(codex.ID, "one", "")
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

func TestModelRemoveRefusesActiveModel(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	addAccountModel(&cfg, "team", "team", "Team", domain.Endpoints{Anthropic: "https://team.test"}, claude.ID, "claude-test")
	cfg.SetSelectedModel(claude.ID, "team", "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	err := cli.Execute(app, []string{"route", "remove", "team"})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "selected for claude") {
		t.Fatalf("error = %v", err)
	}
}
