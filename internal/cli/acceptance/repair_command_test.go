package cli_test

import (
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	surfaceidentity "aigw-cli/internal/surface"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepairHumanPreviewAndDependencyFailures(t *testing.T) {
	t.Run("load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Config = configuration.NewStore(t.TempDir())
		if err := execute(t, app, "repair", "--dry-run"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("discovery", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, configuration.Models{configuration.ClientCodex: "gpt"})
		app.Discovery = nil
		if err := execute(t, app, "repair", "--dry-run"); err == nil || !strings.Contains(err.Error(), "discovery") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("human preview", func(t *testing.T) {
		app, out, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, configuration.Models{configuration.ClientCodex: "gpt"})
		if err := execute(t, app, "repair", "--dry-run"); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "Repair preview") || !strings.Contains(out.String(), "Preview did not write") {
			t.Fatalf("output = %q", out.String())
		}
	})

	t.Run("launcher enable", func(t *testing.T) {
		app, _, secretStore, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, configuration.Models{configuration.ClientClaude: "m"})
		_ = secretStore.Set("one", "token")
		blocker := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		app.ClaudeLauncher.BinDir = filepath.Join(blocker, "bin")
		app.ClaudeLauncher.AIGWExecutable = "/bin/aigw"
		app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{configuration.ClientClaude: "/opt/claude"}}}
		if err := execute(t, app, "repair"); err == nil {
			t.Fatal("expected launcher enable failure")
		}
	})
}

func TestRepairDiscoversAndEnablesInstalledClients(t *testing.T) {
	app, out, secretStore, runner := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "dmx", "dmx", "DMXAPI", configuration.Endpoints{Anthropic: "https://dmx.test", OpenAIResponses: "https://dmx.test/v1"}, "", configuration.Models{})
	cfg.Routes.Default = "dmx"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	shimDir := t.TempDir()
	app.ClaudeLauncher.BinDir = shimDir
	app.ClaudeLauncher.AIGWExecutable = filepath.Join(shimDir, "aigw")
	target := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{configuration.ClientClaude: "/opt/claude", configuration.ClientCodex: "/opt/codex"}, Surfaces: []discovery.Surface{{
		ID:          string(surfaceidentity.CodexHomeDefault),
		Authority:   string(surfaceidentity.AuthorityAIGW),
		ConfigPath:  target,
		Present:     true,
		AutoManaged: true,
	}}}}
	if err := execute(t, app, "repair"); err != nil {
		t.Fatal(err)
	}
	got, _ := app.Config.Load()
	if !got.Adapters["claude"].Enabled || !got.Adapters["codex"].Enabled || len(runner.plans) != 1 {
		t.Fatalf("repair config=%#v plans=%#v", got, runner.plans)
	}
	if !strings.Contains(out.String(), "Repair completed") {
		t.Fatalf("output = %s", out.String())
	}
}
