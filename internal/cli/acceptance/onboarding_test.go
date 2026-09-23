package cli_test

import (
	"aigw-cli/internal/cli"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/secrets"
	surfaceidentity "aigw-cli/internal/surface"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncProjectsExplicitlyEnabledCodexWhenItBecomesAvailable(t *testing.T) {
	app, _, secretStore, runner, _ := testApp(t, "")
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{configuration.ClientCodex: "/usr/local/bin/codex"},
		Surfaces: []discovery.Surface{{
			ID:          string(surfaceidentity.CodexHomeDefault),
			Authority:   string(surfaceidentity.AuthorityAIGW),
			ConfigPath:  target,
			Present:     true,
			AutoManaged: true,
		}},
	}}
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{
		Label:     "DMXAPI",
		Endpoints: configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"},
	}
	cfg.Routes["gpt"] = qualifiedRoute("GPT", "dmx", "gpt-test", configuration.ProtocolOpenAIResponses)
	cfg.SetSelectedRoute(configuration.ClientCodex, "gpt")
	cfg.SetClientActivation(configuration.ClientCodex, true, "", nil)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "test-token"); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatalf("sync after installing Codex: %v", err)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("sync started credential binding plans: %#v", runner.plans)
	}
	after, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	adapter := after.Clients[configuration.ClientCodex]
	if !adapter.Enabled || adapter.Executable != "/usr/local/bin/codex" || len(adapter.Targets) != 1 {
		t.Fatalf("Codex adapter after sync = %#v", adapter)
	}
	assertSameExistingPath(t, adapter.Targets[0], target)
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `model = "gpt-test" # managed by AIGW`) {
		t.Fatalf("sync did not project newly discovered Codex target:\n%s", data)
	}
}

func TestUseCreatesCodexProjectionWhenClientIsInstalledAfterManifestSetup(t *testing.T) {
	app, _, secretStore, runner, _ := testApp(t, "")
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)
	if err := cli.Execute(app, []string{"setup", "--from", manifestPath}); err != nil {
		t.Fatalf("initial manifest setup: %v", err)
	}

	home := t.TempDir()
	target := filepath.Join(home, ".codex", "config.toml")
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{configuration.ClientCodex: "/usr/local/bin/codex"},
		Surfaces: []discovery.Surface{{
			ID:          string(surfaceidentity.CodexHomeDefault),
			Authority:   string(surfaceidentity.AuthorityAIGW),
			ConfigPath:  target,
			Present:     false,
			AutoManaged: true,
		}},
	}}
	if err := secretStore.Set("dmxapi", "test-token"); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"use", "--for", "codex", "dmxapi-gpt"}); err != nil {
		t.Fatalf("select Codex after installation without an existing config file: %v", err)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("sync started credential binding plans: %#v", runner.plans)
	}
	after, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	adapter := after.Clients[configuration.ClientCodex]
	if adapter.Route != "dmxapi-gpt" || !adapter.Enabled || adapter.Executable != "/usr/local/bin/codex" || len(adapter.Targets) != 1 {
		t.Fatalf("Codex adapter after sync = %#v", adapter)
	}
	if adapter.Targets[0] != target {
		t.Fatalf("Codex target = %q, want %q", adapter.Targets[0], target)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read newly created Codex projection: %v", err)
	}
	if !strings.Contains(string(data), `model = "gpt-test" # managed by AIGW`) {
		t.Fatalf("sync did not create the newly discovered Codex target:\n%s", data)
	}
}

func TestSyncActivatesSelectedEnvironmentAccountAfterManifestSetup(t *testing.T) {
	app, out, _, runner, _ := testApp(t, "")
	tokens := map[string]string{}
	app.Secrets = secrets.NewEnvironmentStore(func(key string) string { return tokens[key] })
	app.Discovery = fakeDiscovery{}
	tokens[secrets.EnvironmentKey("aihubmix")] = "test-token"
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)
	if err := cli.Execute(app, []string{"setup", "--from", manifestPath}); err != nil {
		t.Fatalf("initial manifest setup: %v", err)
	}

	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	app.ClaudeSettingsPath = settingsPath
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{configuration.ClientClaude: "/usr/local/bin/claude"},
	}}
	before, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := cli.Execute(app, []string{"sync", "--dry-run", "--json"}); err != nil {
		t.Fatalf("preview sync after setting one environment Token: %v", err)
	}
	var preview struct {
		DryRun     bool              `json:"dry_run"`
		Selections map[string]string `json:"selections"`
		Targets    []struct {
			Client string `json:"client"`
			Target string `json:"target"`
			Action string `json:"action"`
		} `json:"targets"`
	}
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatalf("decode sync preview: %v\n%s", err, out.String())
	}
	if !preview.DryRun || len(preview.Targets) != 1 || preview.Targets[0].Client != configuration.ClientClaude || preview.Targets[0].Target != settingsPath || preview.Targets[0].Action != "project" {
		t.Fatalf("sync preview = %#v", preview)
	}
	wantRoutes := map[string]string{
		configuration.ClientClaude: "aihubmix-claude",
	}
	if !maps.Equal(preview.Selections, wantRoutes) {
		t.Fatalf("sync preview selections = %#v, want %#v", preview.Selections, wantRoutes)
	}
	out.Reset()
	if err := cli.Execute(app, []string{"sync", "--dry-run"}); err != nil {
		t.Fatalf("render sync preview after setting one environment Token: %v", err)
	}
	for client, profile := range wantRoutes {
		if !strings.Contains(out.String(), "Client · "+client) || !strings.Contains(out.String(), profile) {
			t.Fatalf("sync preview omitted %s route %s:\n%s", client, profile, out.String())
		}
	}
	afterPreview, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := afterPreview.SelectedRoute(configuration.ClientClaude); got != before.SelectedRoute(configuration.ClientClaude) {
		t.Fatalf("dry-run changed Claude selection from %q to %q", before.SelectedRoute(configuration.ClientClaude), got)
	}
	if _, err := os.Stat(settingsPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry-run wrote Claude settings: %v", err)
	}

	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatalf("sync after setting one environment Token: %v", err)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("sync started credential binding plans: %#v", runner.plans)
	}
	after, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if after.SelectedRoute(configuration.ClientClaude) != wantRoutes[configuration.ClientClaude] {
		t.Fatalf("bindings = %#v, want selections %#v", after.Clients, wantRoutes)
	}
	adapter := after.Clients[configuration.ClientClaude]
	if !adapter.Enabled || adapter.Executable != "/usr/local/bin/claude" {
		t.Fatalf("Claude adapter after sync = %#v", adapter)
	}
	data := readFile(t, settingsPath)
	if !strings.Contains(string(data), `"ANTHROPIC_BASE_URL": "https://aihubmix.test"`) {
		t.Fatalf("sync did not project the environment-backed Account:\n%s", data)
	}
}

func TestSyncActivatesLateTokenWithoutChangingIndependentRoute(t *testing.T) {
	app, _, secretStore, runner, _ := testApp(t, "")
	app.Discovery = fakeDiscovery{}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)
	if err := cli.Execute(app, []string{"setup", "--from", manifestPath}); err != nil {
		t.Fatalf("initial manifest setup: %v", err)
	}
	before, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	before.SetSelectedRoute(configuration.ClientClaude, "aihubmix-claude")
	before.SetSelectedRoute(configuration.ClientCodex, "dmxapi-gpt")
	before.SetClientActivation(configuration.ClientCodex, true, "", nil)
	if err := app.Config.Save(before); err != nil {
		t.Fatal(err)
	}

	codexTarget := filepath.Join(t.TempDir(), "config.toml")
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{
			configuration.ClientClaude: executableFixture(t, "claude"),
			configuration.ClientCodex:  executableFixture(t, "codex"),
		},
		Surfaces: []discovery.Surface{{
			ID:          string(surfaceidentity.CodexHomeDefault),
			Authority:   string(surfaceidentity.AuthorityAIGW),
			ConfigPath:  codexTarget,
			AutoManaged: true,
		}},
	}}
	if err := secretStore.Set("dmxapi", "team-token"); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatalf("sync after Token became available: %v", err)
	}
	after, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if after.SelectedRoute(configuration.ClientClaude) != before.SelectedRoute(configuration.ClientClaude) ||
		after.SelectedRoute(configuration.ClientCodex) != before.SelectedRoute(configuration.ClientCodex) {
		t.Fatalf("sync changed independent bindings: got %#v, want %#v", after.Clients, before.Clients)
	}
	if after.Clients[configuration.ClientClaude].Enabled {
		t.Fatalf("sync activated Claude through an unselected Account: %#v", after.Clients[configuration.ClientClaude])
	}
	if adapter := after.Clients[configuration.ClientCodex]; !adapter.Enabled || adapter.Executable == "" || len(adapter.Targets) != 1 {
		t.Fatalf("sync did not activate the selected Codex Route: %#v", adapter)
	}
	if data := readFile(t, codexTarget); !strings.Contains(string(data), `model = "gpt-test" # managed by AIGW`) {
		t.Fatalf("sync did not project the selected Codex Route:\n%s", data)
	}
	if _, err := os.Stat(app.ClaudeSettingsPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("sync projected the unready Claude Route: %v", err)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("sync rebound native authentication: %#v", runner.plans)
	}
}

func TestSyncDefersNewlyInstalledClientUntilItsAccountIsConnected(t *testing.T) {
	app, _, _, runner, _ := testApp(t, "")
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{configuration.ClientCodex: "/usr/local/bin/codex"},
		Surfaces: []discovery.Surface{{
			ID:          string(surfaceidentity.CodexHomeDefault),
			Authority:   string(surfaceidentity.AuthorityAIGW),
			ConfigPath:  target,
			Present:     true,
			AutoManaged: true,
		}},
	}}
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{
		Label:     "DMXAPI",
		Endpoints: configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"},
	}
	cfg.Routes["gpt"] = qualifiedRoute("GPT", "dmx", "gpt-test", configuration.ProtocolOpenAIResponses)
	cfg.SetSelectedRoute(configuration.ClientCodex, "gpt")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatalf("sync with an unconnected Account: %v", err)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("sync started credential binding plans: %#v", runner.plans)
	}
	after, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if adapter := after.Clients[configuration.ClientCodex]; adapter.Enabled {
		t.Fatalf("Codex adapter was enabled before Account connection: %#v", adapter)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "model_provider = \"native\"\n" {
		t.Fatalf("sync projected an unconnected Account:\n%s", data)
	}
}
