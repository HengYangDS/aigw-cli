package cli_test

import (
	"aigw-cli/internal/cli"
	"aigw-cli/internal/configuration"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestStatusSuggestsAccountSpecificDiagnostics(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMXAPI", Endpoints: configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"}, AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://www.dmxapi.cn"}}
	cfg.Profiles["gpt-5.6-sol"] = configuration.Profile{Label: "GPT", Account: "dmx", Client: configuration.ClientCodex, Model: "gpt-5.6-sol"}
	cfg.Routes[configuration.ClientCodex] = "gpt-5.6-sol"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	if err := cli.Execute(app, []string{"status"}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "aigw account connect dmx") {
		t.Fatalf("status should suggest account-specific diagnostics:\n%s", text)
	}
}

func TestStatusWarnsWhenEnabledClaudeAdapterExecutableIsUnavailable(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMX", Endpoints: configuration.Endpoints{Anthropic: "https://example.test"}}
	cfg.Profiles["claude-fable-5"] = configuration.Profile{Label: "Claude Fable", Account: "dmx", Client: configuration.ClientClaude, Model: "claude-fable-5"}
	cfg.Routes[configuration.ClientClaude] = "claude-fable-5"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/claude-real"}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "token"); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"status"}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "Claude executable is unavailable") || !strings.Contains(text, "aigw repair") {
		t.Fatalf("status did not surface the unavailable Claude executable:\n%s", text)
	}
}

func TestTerminalErrorLocalizesUnsupportedConfigVersion(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	if err := os.WriteFile(app.Config.Path(), []byte("version = 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := cli.Execute(app, []string{"status"})
	if err == nil {
		t.Fatal("status unexpectedly succeeded")
	}
	text := out.String()
	for _, want := range []string{"unsupported configuration version: found 0, expected 3", "Recommended action", "aigw check"} {
		if !strings.Contains(text, want) {
			t.Fatalf("localized configuration error lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "unsupported config version") {
		t.Fatalf("terminal leaked raw configuration error:\n%s", text)
	}
}

func TestStatusGuidesClientSpecificRouteInsteadOfBlankRepair(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMXAPI", Endpoints: configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1", Anthropic: "https://dmx.test"}}
	cfg.Profiles["gpt-5.6-sol"] = configuration.Profile{Label: "GPT", Account: "dmx", Client: configuration.ClientCodex, Model: "gpt-5.6-sol"}
	cfg.Profiles["claude-fable-5"] = configuration.Profile{Label: "Claude", Account: "dmx", Client: configuration.ClientClaude, Model: "claude-fable-5"}
	cfg.Routes[configuration.ClientCodex] = "gpt-5.6-sol"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	if err := cli.Execute(app, []string{"status"}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if strings.Contains(text, "Claude             ·") || strings.Contains(text, "aigw repair") {
		t.Fatalf("status should not show blank Claude route or misleading repair:\n%s", text)
	}
	for _, want := range []string{"Claude", "No Claude profile selected", "aigw use claude-fable-5"} {
		if !strings.Contains(text, want) {
			t.Fatalf("status lacks %q:\n%s", want, text)
		}
	}
}

func TestStatusKeepsTheFirstRunNextActionSimple(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	if err := cli.Execute(app, []string{"status"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Not configured", "Get started", "aigw setup"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("status lacks %q:\n%s", want, out.String())
		}
	}
}

func TestStatusWarnsWhenClaudeExecutableIsUnavailable(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "claude", "claude", "Claude", configuration.Endpoints{Anthropic: "https://example.test"}, configuration.ClientClaude, "claude-test")
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/claude-real"}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("claude", "token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"status"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Claude executable is unavailable") || !strings.Contains(out.String(), "aigw repair") {
		t.Fatalf("status did not surface the unavailable Claude executable:\n%s", out.String())
	}
}

func TestStatusShowsIndependentRoutesAndJSONNeverContainsToken(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "codex", "dmx", "Codex", configuration.Endpoints{Anthropic: "https://example.test", OpenAIResponses: "https://example.test/v1"}, configuration.ClientCodex, "gpt-test")
	addAccountProfile(&cfg, "claude", "dmx", "Claude", configuration.Endpoints{Anthropic: "https://example.test", OpenAIResponses: "https://example.test/v1"}, configuration.ClientClaude, "claude-test")
	cfg.Routes[configuration.ClientCodex] = "codex"
	cfg.Routes[configuration.ClientClaude] = "claude"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "never-print-this-secret")
	if err := cli.Execute(app, []string{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Claude") || !strings.Contains(out.String(), "Codex") || strings.Contains(out.String(), "Inherits default") {
		t.Fatalf("human status = %s", out.String())
	}
	out.Reset()
	if err := cli.Execute(app, []string{"status", "--json"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "never-print-this-secret") || !strings.Contains(out.String(), `"authentication": "account-token"`) {
		t.Fatalf("unsafe JSON status = %s", out.String())
	}
}

func TestStatusReportsRouteTransport(t *testing.T) {
	for _, tc := range []struct {
		name      string
		endpoint  string
		transport string
	}{
		{"http_loopback", "http://localhost:4567/v1", `"external_loopback"`},
		{"https_loopback", "https://[::1]:4567/v1", `"external_loopback"`},
		{"remote", "https://gateway.test/v1", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, out, secretStore, _, _ := testApp(t, "")
			app.Config = configuration.NewStore(filepath.Join(t.TempDir(), "localhost-4567-configuration.toml"))
			cfg := configuration.NewConfig()
			addAccountProfile(&cfg, "team", "team", "Team", configuration.Endpoints{OpenAIResponses: tc.endpoint}, configuration.ClientCodex, "model-test")
			cfg.Routes[configuration.ClientCodex] = "team"
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := secretStore.Set("team", "token"); err != nil {
				t.Fatal(err)
			}
			if err := cli.Execute(app, []string{"status", "--json"}); err != nil {
				t.Fatal(err)
			}
			var document struct {
				ConfigPath string                                `json:"config_path"`
				Routes     map[string]map[string]json.RawMessage `json:"routes"`
			}
			if err := json.Unmarshal(out.Bytes(), &document); err != nil {
				t.Fatal(err)
			}
			if document.ConfigPath != app.Config.Path() {
				t.Fatalf("config path = %q, want %q", document.ConfigPath, app.Config.Path())
			}
			want := map[string]json.RawMessage{
				"state":               json.RawMessage(`"deferred"`),
				"profile":             json.RawMessage(`"team"`),
				"account":             json.RawMessage(`"team"`),
				"detail":              json.RawMessage(`"The client is not installed or enabled"`),
				"next_action":         json.RawMessage(`"aigw sync"`),
				"authentication":      json.RawMessage(`"account-token"`),
				"endpoint_configured": json.RawMessage(`true`),
				"adapter_ready":       json.RawMessage(`false`),
			}
			if tc.transport != "" {
				want["transport"] = json.RawMessage(tc.transport)
			}
			if got := document.Routes["codex"]; !reflect.DeepEqual(got, want) {
				t.Fatalf("Codex route = %s, want %s", got, want)
			}
		})
	}
}

func TestStatusLabelsProfileCountAsModelConfigurations(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["team"] = configuration.Account{Label: "Team", Endpoints: configuration.Endpoints{OpenAIResponses: "https://team.test/v1", Anthropic: "https://team.test"}}
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Account: "team", Client: configuration.ClientCodex, Model: "gpt-test"}
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "team", Client: configuration.ClientClaude, Model: "claude-test"}
	cfg.Routes[configuration.ClientCodex] = "gpt"
	cfg.Routes[configuration.ClientClaude] = "claude"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("team", "team-token"); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"status"}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "claude") || !strings.Contains(text, "gpt") {
		t.Fatalf("status did not identify configuration count:\n%s", text)
	}
	if strings.Contains(text, "configured service") {
		t.Fatalf("status mislabels configuration count as services:\n%s", text)
	}
}

func TestRouteListIsNarrowHumanRouteView(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "gpt", "team", "GPT", configuration.Endpoints{OpenAIResponses: "https://team.test/v1", Anthropic: "https://team.test"}, configuration.ClientCodex, "gpt-test")
	addAccountProfile(&cfg, "claude", "team", "Claude", configuration.Endpoints{Anthropic: "https://team.test"}, configuration.ClientClaude, "claude-test")
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Purpose: "Default coding", Account: "team", Client: configuration.ClientCodex, Model: "gpt-test"}
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Purpose: "Independent review", Account: "team", Client: configuration.ClientClaude, Model: "claude-test"}
	cfg.Routes[configuration.ClientCodex] = "gpt"
	cfg.Routes[configuration.ClientClaude] = "claude"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"route", "list"}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"Current routes", "Codex", "gpt", "Default coding", "Claude", "claude", "Independent review", "aigw use <profile>"} {
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

func TestRouteListReportsEachUnselectedClientIndependently(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "gpt", "team", "GPT", configuration.Endpoints{OpenAIResponses: "https://team.test/v1", Anthropic: "https://team.test"}, configuration.ClientCodex, "gpt-test")
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "team", Client: configuration.ClientClaude, Model: "claude-test"}
	cfg.Routes[configuration.ClientCodex] = "gpt"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"route", "list"}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"Claude", "No Claude profile selected", "aigw use claude", "Next\n  aigw use claude"} {
		if !strings.Contains(text, want) {
			t.Fatalf("route list lacks %q:\n%s", want, text)
		}
	}
}
