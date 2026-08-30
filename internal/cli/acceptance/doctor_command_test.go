package cli_test

import (
	configuration "aigw-cli/internal/configuration"
	"strings"
	"testing"
)

func TestDoctorChecksAccountTokenOnceForSharedProfiles(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMXAPI", Endpoints: configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1", Anthropic: "https://dmx.test"}}
	cfg.Profiles["alpha-model"] = configuration.Profile{Label: "GPT", Account: "dmx", Client: configuration.ClientCodex, Model: "alpha-model"}
	cfg.Profiles["beta-model"] = configuration.Profile{Label: "Claude", Account: "dmx", Client: configuration.ClientClaude, Model: "beta-model"}
	cfg.Routes[configuration.ClientCodex] = "alpha-model"
	cfg.Routes[configuration.ClientClaude] = "beta-model"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	if err := execute(t, app, "doctor"); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if strings.Contains(text, "secret:alpha-model") || strings.Contains(text, "secret:beta-model") {
		t.Fatalf("doctor checked profile secrets instead of account secret:\n%s", text)
	}
	if !strings.Contains(text, "System secret") || !strings.Contains(text, "dmx · available") || !strings.Contains(text, "No problems found") {
		t.Fatalf("doctor did not report account secret cleanly:\n%s", text)
	}
}

func TestDoctorAcceptsConfiguredClaudeExecutableWithoutDiscovery(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "dmx", "dmx", "DMXAPI", configuration.Endpoints{Anthropic: "https://dmx.test"}, configuration.ClientClaude, "claude-model")
	cfg.Routes[configuration.ClientClaude] = "dmx"
	cfg.Adapters["claude"] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "claude")}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	err := execute(t, app, "doctor")
	if err != nil || !strings.Contains(out.String(), "Claude adapter") || !strings.Contains(out.String(), "Enabled") {
		t.Fatalf("doctor did not accept the configured executable; err=%v output=%s", err, out.String())
	}
}
