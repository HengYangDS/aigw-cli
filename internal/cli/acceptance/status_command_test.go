package cli_test

import (
	configuration "aigw-cli/internal/configuration"
	"strings"
	"testing"
)

func TestStatusKeepsTheFirstRunNextActionSimple(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	if err := execute(t, app, "status"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Not configured", "Get started", "aigw setup"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("status lacks %q:\n%s", want, out.String())
		}
	}
}

func TestStatusWarnsWhenClaudeExecutableIsUnavailable(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
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
	if err := execute(t, app, "status"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Claude executable is unavailable") || !strings.Contains(out.String(), "aigw repair") {
		t.Fatalf("status did not surface the unavailable Claude executable:\n%s", out.String())
	}
}

func TestStatusShowsIndependentRoutesAndJSONNeverContainsToken(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "codex", "dmx", "Codex", configuration.Endpoints{Anthropic: "https://example.test", OpenAIResponses: "https://example.test/v1"}, configuration.ClientCodex, "gpt-test")
	addAccountProfile(&cfg, "claude", "dmx", "Claude", configuration.Endpoints{Anthropic: "https://example.test", OpenAIResponses: "https://example.test/v1"}, configuration.ClientClaude, "claude-test")
	cfg.Routes[configuration.ClientCodex] = "codex"
	cfg.Routes[configuration.ClientClaude] = "claude"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "never-print-this-secret")
	if err := execute(t, app); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Claude") || !strings.Contains(out.String(), "Codex") || strings.Contains(out.String(), "Inherits default") {
		t.Fatalf("human status = %s", out.String())
	}
	out.Reset()
	if err := execute(t, app, "status", "--json"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "never-print-this-secret") || !strings.Contains(out.String(), `"secret_available": true`) {
		t.Fatalf("unsafe JSON status = %s", out.String())
	}
}

func TestStatusMarksExternalLoopbackTransportWithoutExposingEndpoint(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "local", "local", "Local Compatibility Layer", configuration.Endpoints{OpenAIResponses: "http://localhost:4567/v1"}, configuration.ClientCodex, "model-test")
	cfg.Routes[configuration.ClientCodex] = "local"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("local", "token"); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, app, "status", "--json"); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{`"transport": "external_loopback"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("status JSON lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "requests use the external listener") {
		t.Fatalf("machine-readable status leaked human transport prose:\n%s", text)
	}
	if strings.Contains(text, "4567") || strings.Contains(text, "localhost") {
		t.Fatalf("status JSON exposed loopback endpoint:\n%s", text)
	}
}

func TestStatusMarksHTTPSLoopbackTransportWithoutExposingEndpoint(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "local", "local", "Local Compatibility Layer", configuration.Endpoints{OpenAIResponses: "https://[::1]:4567/v1"}, configuration.ClientCodex, "model-test")
	cfg.Routes[configuration.ClientCodex] = "local"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("local", "token"); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, app, "status", "--json"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"transport": "external_loopback"`) {
		t.Fatalf("status did not classify HTTPS loopback:\n%s", out.String())
	}
}

func TestStatusDoesNotClassifyRemoteHTTPAsExternalLoopbackTransport(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "remote", "remote", "Remote Development Gateway", configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1"}, configuration.ClientCodex, "model-test")
	cfg.Routes[configuration.ClientCodex] = "remote"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("remote", "token"); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, app, "status", "--json"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "external_loopback") {
		t.Fatalf("status misclassified remote endpoint:\n%s", out.String())
	}
}

func TestStatusLabelsProfileCountAsModelConfigurations(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
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

	if err := execute(t, app, "status"); err != nil {
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
