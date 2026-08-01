package cli_test

import (
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorChecksAccountTokenOnceForSharedProfiles(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMXAPI", Endpoints: configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1", Anthropic: "https://dmx.test"}}
	cfg.Profiles["alpha-model"] = configuration.Profile{Label: "GPT", Account: "dmx", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "alpha-model"}}
	cfg.Profiles["beta-model"] = configuration.Profile{Label: "Claude", Account: "dmx", Client: configuration.ClientClaude, Models: configuration.Models{configuration.ClientClaude: "beta-model"}}
	cfg.Routes.Default = "alpha-model"
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

func TestDoctorAcceptsOwnedClaudeLauncherWithoutPathDiscovery(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "dmx", "dmx", "DMXAPI", configuration.Endpoints{Anthropic: "https://dmx.test"}, "", configuration.Models{})
	cfg.Routes.Default = "dmx"
	cfg.Adapters["claude"] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/claude-real"}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	shimDir := t.TempDir()
	app.ClaudeLauncher.BinDir = shimDir
	app.ClaudeLauncher.AIGWExecutable = filepath.Join(shimDir, "aigw")
	if _, err := app.ClaudeLauncher.EnableClaude(); err != nil {
		t.Fatal(err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{}}
	err := execute(t, app, "doctor")
	if err != nil || !strings.Contains(out.String(), "Claude launcher") || !strings.Contains(out.String(), "AIGW-managed Claude launcher is ready") {
		t.Fatalf("doctor did not accept the owned launcher; err=%v output=%s", err, out.String())
	}
}
