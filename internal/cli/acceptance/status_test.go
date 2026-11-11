package cli_test

import (
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/store"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestStatusSuggestsProviderSpecificDiagnostics(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["dmx"] = domain.Provider{Label: "DMXAPI", Endpoints: domain.Endpoints{Responses: "https://dmx.test/v1"}, BalanceProbe: &domain.BalanceProbe{Kind: "dmxapi", BaseURL: "https://www.dmxapi.cn"}}
	cfg.Models[domain.NewModelRef("dmx", "gpt-5.6-sol")] = qualifiedModel("GPT", "dmx", "gpt-5.6-sol", domain.ProtocolResponses)
	cfg.SetSelectedModel(codex.ID, domain.NewModelRef("dmx", "gpt-5.6-sol"), "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	if err := cli.Execute(app, []string{"status"}); err != nil {
		t.Fatal(err)
	}
	text := strings.Join(strings.Fields(out.String()), " ")
	if !strings.Contains(text, "aigw provider edit dmx --balance-token-stdin --balance-user-id <user-id>") {
		t.Fatalf("status should name the provider's balance credentials:\n%s", out)
	}
}

func TestStatusWarnsWhenEnabledClaudeAdapterExecutableIsUnavailable(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["dmx"] = domain.Provider{Label: "DMX", Endpoints: domain.Endpoints{Anthropic: "https://example.test"}}
	cfg.Models[domain.NewModelRef("dmx", "claude-fable-5")] = qualifiedModel("Claude Fable", "dmx", "claude-fable-5", domain.ProtocolAnthropic)
	cfg.SetSelectedModel(claude.ID, domain.NewModelRef("dmx", "claude-fable-5"), "")
	cfg.SetClientActivation(claude.ID, true, "/opt/claude-real", nil)
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
	if !strings.Contains(text, "Claude executable is unavailable") || !strings.Contains(text, "aigw sync") {
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
	for _, want := range []string{
		"unsupported configuration version: found 0, expected 8",
		"AIGW does not reinterpret configuration schemas",
		"\nFix\n",
		"aigw config path",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("localized configuration error lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "aigw check --depth local") {
		t.Fatalf("configuration error names a command that loads the configuration:\n%s", text)
	}
	if strings.Contains(text, "unsupported config version") {
		t.Fatalf("terminal leaked raw configuration error:\n%s", text)
	}
}

func TestStatusDirectsAPredecessorSchemaToTheMatchingRelease(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	if err := os.WriteFile(app.Config.Path(), []byte("version = 5\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"status"}); err == nil {
		t.Fatal("legacy configuration unexpectedly entered normal runtime")
	}
	if text := out.String(); strings.Contains(text, "aigw config migrate") || !strings.Contains(text, "unsupported configuration version: found 5") {
		t.Fatalf("legacy configuration guidance = %s", text)
	}
}

func TestStatusPrioritizesSelectedActivationWhileRetainingModelGuidance(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["dmx"] = domain.Provider{Label: "DMXAPI", Endpoints: domain.Endpoints{Responses: "https://dmx.test/v1", Anthropic: "https://dmx.test"}}
	cfg.Models[domain.NewModelRef("dmx", "gpt-5.6-sol")] = qualifiedModel("GPT", "dmx", "gpt-5.6-sol", domain.ProtocolResponses)
	cfg.Models[domain.NewModelRef("dmx", "claude-fable-5")] = qualifiedModel("Claude", "dmx", "claude-fable-5", domain.ProtocolAnthropic)
	cfg.SetSelectedModel(codex.ID, domain.NewModelRef("dmx", "gpt-5.6-sol"), "")
	cfg.SetRecommendedModel(claude.ID, domain.NewModelRef("dmx", "claude-fable-5"))
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	if err := cli.Execute(app, []string{"status"}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if strings.Contains(text, "Claude             ·") || strings.Contains(text, "aigw sync") {
		t.Fatalf("status should not show blank Claude route or misleading repair:\n%s", text)
	}
	for _, want := range []string{"Claude", "No model selected", "aigw use codex dmx/gpt-5.6-sol"} {
		if !strings.Contains(text, want) {
			t.Fatalf("status lacks %q:\n%s", want, text)
		}
	}
	out.Reset()
	if err := cli.Execute(app, []string{"status", "--json"}); err != nil {
		t.Fatal(err)
	}
	var result struct {
		NextAction string `json:"next_action"`
		Clients    map[string]struct {
			NextAction string `json:"next_action"`
		} `json:"clients"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.NextAction != "aigw use codex dmx/gpt-5.6-sol" || result.Clients[claude.ID].NextAction != "aigw use claude dmx/claude-fable-5" {
		t.Fatalf("JSON status action = %+v, error = %v", result, err)
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
	cfg := domain.NewConfig(clients.Admission())
	addProviderModel(&cfg, "claude", "Claude", domain.Endpoints{Anthropic: "https://example.test"}, claude.ID, "claude-test")
	cfg.SetSelectedModel(claude.ID, domain.NewModelRef("claude", "claude-test"), "")
	cfg.SetClientActivation(claude.ID, true, "/opt/claude-real", nil)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("claude", "token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"status"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Claude executable is unavailable") || !strings.Contains(out.String(), "aigw sync") {
		t.Fatalf("status did not surface the unavailable Claude executable:\n%s", out.String())
	}
}

func TestStatusShowsIndependentModelsAndJSONNeverContainsToken(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	addProviderModel(&cfg, "dmx", "Codex", domain.Endpoints{Anthropic: "https://example.test", Responses: "https://example.test/v1"}, codex.ID, "gpt-test")
	addProviderModel(&cfg, "dmx", "Claude", domain.Endpoints{Anthropic: "https://example.test", Responses: "https://example.test/v1"}, claude.ID, "claude-test")
	cfg.SetSelectedModel(codex.ID, domain.NewModelRef("dmx", "gpt-test"), "")
	cfg.SetSelectedModel(claude.ID, domain.NewModelRef("dmx", "claude-test"), "")
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
	if strings.Contains(out.String(), "never-print-this-secret") || !strings.Contains(out.String(), `"authentication": "provider-token"`) || strings.Contains(out.String(), `"profiles"`) {
		t.Fatalf("unsafe JSON status = %s", out.String())
	}
}

func TestStatusReportsModelTransport(t *testing.T) {
	for _, tc := range []struct {
		name      string
		endpoint  string
		transport string
	}{
		{"http_loopback", "http://localhost:4567/v1", `"external_loopback"`},
		{"https_loopback", "https://[::1]:4567/v1", `"external_loopback"`},
		{"ipv4_loopback_range", "http://127.1.2.3:4567/v1", `"external_loopback"`},
		{"mapped_ipv4_loopback", "http://[::ffff:127.0.0.2]:4567/v1", `"external_loopback"`},
		{"private_network", "https://192.168.1.1:4567/v1", ""},
		{"remote", "https://gateway.test/v1", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, out, secretStore, _, _ := testApp(t, "")
			app.Config = store.NewStore(filepath.Join(t.TempDir(), "localhost-4567-configuration.toml"), clients.Admission())
			cfg := domain.NewConfig(clients.Admission())
			addProviderModel(&cfg, "team", "Team", domain.Endpoints{Responses: tc.endpoint}, codex.ID, "model-test")
			cfg.SetSelectedModel(codex.ID, domain.NewModelRef("team", "model-test"), "")
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
				Clients map[string]map[string]json.RawMessage `json:"clients"`
			}
			if err := json.Unmarshal(out.Bytes(), &document); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(out.String(), `"config_path"`) || strings.Contains(out.String(), app.Config.Path()) {
				t.Fatalf("status exposed a private configuration path: %s", out.String())
			}
			want := map[string]json.RawMessage{
				"state":               json.RawMessage(`"deferred"`),
				"model":               json.RawMessage(`"team/model-test"`),
				"provider":            json.RawMessage(`"team"`),
				"detail":              json.RawMessage(`"The client is not installed or enabled"`),
				"authentication":      json.RawMessage(`"provider-token"`),
				"endpoint_configured": json.RawMessage(`true`),
				"client_files_ready":  json.RawMessage(`false`),
			}
			if tc.transport != "" {
				want["transport"] = json.RawMessage(tc.transport)
			}
			if got := document.Clients["codex"]; !reflect.DeepEqual(got, want) {
				t.Fatalf("Codex client = %s, want %s", got, want)
			}
		})
	}
}

func TestStatusLabelsModelCountAsModelConfigurations(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["team"] = domain.Provider{Label: "Team", Endpoints: domain.Endpoints{Responses: "https://team.test/v1", Anthropic: "https://team.test"}}
	cfg.Models[domain.NewModelRef("team", "gpt-test")] = qualifiedModel("GPT", "team", "gpt-test", domain.ProtocolResponses)
	cfg.Models[domain.NewModelRef("team", "claude-test")] = qualifiedModel("Claude", "team", "claude-test", domain.ProtocolAnthropic)
	cfg.SetSelectedModel(codex.ID, domain.NewModelRef("team", "gpt-test"), "")
	cfg.SetSelectedModel(claude.ID, domain.NewModelRef("team", "claude-test"), "")
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
