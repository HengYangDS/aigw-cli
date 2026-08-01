package cli_test

import (
	configuration "aigw-cli/internal/configuration"
	"os"
	"path/filepath"
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

func TestStatusWarnsWhenClaudePathActivationIsMissing(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "claude", "claude", "Claude", configuration.Endpoints{Anthropic: "https://example.test"}, configuration.ClientClaude, configuration.Models{configuration.ClientClaude: "claude-test"})
	cfg.Routes.Default = "claude"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/claude-real"}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("claude", "token"); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	shimDir := filepath.Join(home, "Library", "Application Support", "aigw", "bin")
	app.ClaudeLauncher.BinDir = shimDir
	app.ClaudeLauncher.Home = home
	app.ClaudeLauncher.Shell = "/bin/zsh"
	app.ClaudeLauncher.AIGWExecutable = filepath.Join(shimDir, "aigw")
	if err := os.MkdirAll(shimDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shimDir, "claude"), []byte("#!/bin/sh\n# AIGW managed Claude launcher\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, app, "status"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Claude PATH activation is missing") || !strings.Contains(out.String(), "aigw repair") {
		t.Fatalf("status did not surface missing Claude PATH activation:\n%s", out.String())
	}
}

func TestStatusShowsInheritanceAndJSONNeverContainsToken(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "dmx", "dmx", "DMX", configuration.Endpoints{Anthropic: "https://example.test", OpenAIResponses: "https://example.test/v1"}, "", configuration.Models{})
	cfg.Routes.Default = "dmx"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "never-print-this-secret")
	if err := execute(t, app); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Claude") || !strings.Contains(out.String(), "Inherits default") {
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
	addAccountProfile(&cfg, "local", "local", "Local Compatibility Layer", configuration.Endpoints{OpenAIResponses: "http://localhost:4567/v1"}, configuration.ClientCodex, configuration.Models{configuration.ClientCodex: "model-test"})
	cfg.Routes.Default = "local"
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
	addAccountProfile(&cfg, "local", "local", "Local Compatibility Layer", configuration.Endpoints{OpenAIResponses: "https://[::1]:4567/v1"}, configuration.ClientCodex, configuration.Models{configuration.ClientCodex: "model-test"})
	cfg.Routes.Default = "local"
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
	addAccountProfile(&cfg, "remote", "remote", "Remote Development Gateway", configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1"}, configuration.ClientCodex, configuration.Models{configuration.ClientCodex: "model-test"})
	cfg.Routes.Default = "remote"
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
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Account: "team", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-test"}}
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "team", Client: configuration.ClientClaude, Models: configuration.Models{configuration.ClientClaude: "claude-test"}}
	cfg.Routes.Default = "gpt"
	cfg.Routes.Overrides[configuration.ClientClaude] = "claude"
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
	if !strings.Contains(text, "Model profiles       2") {
		t.Fatalf("status did not identify configuration count:\n%s", text)
	}
	if strings.Contains(text, "configured service") {
		t.Fatalf("status mislabels configuration count as services:\n%s", text)
	}
}
