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

func TestStatusSuggestsAccountSpecificDiagnostics(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["dmx"] = domain.Account{Label: "DMXAPI", Endpoints: domain.Endpoints{OpenAIResponses: "https://dmx.test/v1"}, AccountProbe: &domain.AccountProbe{Kind: "dmxapi", BaseURL: "https://www.dmxapi.cn"}}
	cfg.Routes["gpt-5.6-sol"] = qualifiedRoute("GPT", "dmx", "gpt-5.6-sol", domain.ProtocolOpenAIResponses)
	cfg.SetSelectedRoute(codex.ID, "gpt-5.6-sol", "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	if err := cli.Execute(app, []string{"status"}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "aigw account diagnostics enable dmx") {
		t.Fatalf("status should suggest account-specific diagnostics:\n%s", text)
	}
}

func TestStatusWarnsWhenEnabledClaudeAdapterExecutableIsUnavailable(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["dmx"] = domain.Account{Label: "DMX", Endpoints: domain.Endpoints{Anthropic: "https://example.test"}}
	cfg.Routes["claude-fable-5"] = qualifiedRoute("Claude Fable", "dmx", "claude-fable-5", domain.ProtocolAnthropic)
	cfg.SetSelectedRoute(claude.ID, "claude-fable-5", "")
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
	for _, want := range []string{
		"unsupported configuration version: found 0, expected 6",
		"AIGW does not reinterpret configuration schemas",
		"Recommended action",
		"aigw doctor",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("localized configuration error lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "unsupported config version") {
		t.Fatalf("terminal leaked raw configuration error:\n%s", text)
	}
}

func TestStatusDirectsTheSupportedPredecessorToExplicitMigration(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	if err := os.WriteFile(app.Config.Path(), []byte("version = 5\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"status"}); err == nil {
		t.Fatal("legacy configuration unexpectedly entered normal runtime")
	}
	if text := out.String(); !strings.Contains(text, "aigw config migrate --dry-run") {
		t.Fatalf("legacy configuration guidance = %s", text)
	}
}

func TestStatusPrioritizesSelectedActivationWhileRetainingRouteGuidance(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["dmx"] = domain.Account{Label: "DMXAPI", Endpoints: domain.Endpoints{OpenAIResponses: "https://dmx.test/v1", Anthropic: "https://dmx.test"}}
	cfg.Routes["gpt-5.6-sol"] = qualifiedRoute("GPT", "dmx", "gpt-5.6-sol", domain.ProtocolOpenAIResponses)
	cfg.Routes["claude-fable-5"] = qualifiedRoute("Claude", "dmx", "claude-fable-5", domain.ProtocolAnthropic)
	cfg.SetSelectedRoute(codex.ID, "gpt-5.6-sol", "")
	cfg.SetRecommendedRoute(claude.ID, "claude-fable-5")
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
	for _, want := range []string{"Claude", "No Claude route selected", "aigw use --for codex gpt-5.6-sol"} {
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
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.NextAction != "aigw use --for codex gpt-5.6-sol" || result.Clients[claude.ID].NextAction != "aigw use --for claude claude-fable-5" {
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
	addAccountRoute(&cfg, "claude", "claude", "Claude", domain.Endpoints{Anthropic: "https://example.test"}, claude.ID, "claude-test")
	cfg.SetSelectedRoute(claude.ID, "claude", "")
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
	if !strings.Contains(out.String(), "Claude executable is unavailable") || !strings.Contains(out.String(), "aigw repair") {
		t.Fatalf("status did not surface the unavailable Claude executable:\n%s", out.String())
	}
}

func TestStatusShowsIndependentRoutesAndJSONNeverContainsToken(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	addAccountRoute(&cfg, "codex", "dmx", "Codex", domain.Endpoints{Anthropic: "https://example.test", OpenAIResponses: "https://example.test/v1"}, codex.ID, "gpt-test")
	addAccountRoute(&cfg, "claude", "dmx", "Claude", domain.Endpoints{Anthropic: "https://example.test", OpenAIResponses: "https://example.test/v1"}, claude.ID, "claude-test")
	cfg.SetSelectedRoute(codex.ID, "codex", "")
	cfg.SetSelectedRoute(claude.ID, "claude", "")
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
	if strings.Contains(out.String(), "never-print-this-secret") || !strings.Contains(out.String(), `"authentication": "account-token"`) || strings.Contains(out.String(), `"profiles"`) {
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
		{"ipv4_loopback_range", "http://127.1.2.3:4567/v1", `"external_loopback"`},
		{"mapped_ipv4_loopback", "http://[::ffff:127.0.0.2]:4567/v1", `"external_loopback"`},
		{"private_network", "https://192.168.1.1:4567/v1", ""},
		{"remote", "https://gateway.test/v1", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, out, secretStore, _, _ := testApp(t, "")
			app.Config = store.NewStore(filepath.Join(t.TempDir(), "localhost-4567-configuration.toml"), clients.Admission())
			cfg := domain.NewConfig(clients.Admission())
			addAccountRoute(&cfg, "team", "team", "Team", domain.Endpoints{OpenAIResponses: tc.endpoint}, codex.ID, "model-test")
			cfg.SetSelectedRoute(codex.ID, "team", "")
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
				"route":               json.RawMessage(`"team"`),
				"account":             json.RawMessage(`"team"`),
				"detail":              json.RawMessage(`"The client is not installed or enabled"`),
				"next_action":         json.RawMessage(`"aigw use --for codex team"`),
				"authentication":      json.RawMessage(`"account-token"`),
				"endpoint_configured": json.RawMessage(`true`),
				"projection_ready":    json.RawMessage(`false`),
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

func TestStatusLabelsRouteCountAsModelConfigurations(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["team"] = domain.Account{Label: "Team", Endpoints: domain.Endpoints{OpenAIResponses: "https://team.test/v1", Anthropic: "https://team.test"}}
	cfg.Routes["gpt"] = qualifiedRoute("GPT", "team", "gpt-test", domain.ProtocolOpenAIResponses)
	cfg.Routes["claude"] = qualifiedRoute("Claude", "team", "claude-test", domain.ProtocolAnthropic)
	cfg.SetSelectedRoute(codex.ID, "gpt", "")
	cfg.SetSelectedRoute(claude.ID, "claude", "")
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
