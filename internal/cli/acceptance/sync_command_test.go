package cli_test

import (
	configuration "aigw-cli/internal/configuration"
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

func TestRepairResyncsAnExistingTruncatedCodexProjection(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	target := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\nmodel = \"gpt-original\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "dmx", "dmx", "DMXAPI", configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"}, configuration.ClientCodex, "gpt-5.6-terra")
	cfg.Routes[configuration.ClientCodex] = "dmx"
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	if err := execute(t, app, "sync"); err != nil {
		t.Fatal(err)
	}
	projected, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(strings.Replace(string(projected), "# <<< AIGW managed provider <<<\n", "", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{configuration.ClientCodex: "/opt/codex"}, Surfaces: []discovery.Surface{{
		ID:          string(surfaceidentity.CodexHomeDefault),
		Authority:   string(surfaceidentity.AuthorityAIGW),
		ConfigPath:  target,
		Present:     true,
		AutoManaged: true,
	}}}}

	if err := execute(t, app, "repair"); err != nil {
		t.Fatalf("repair did not resync the existing Codex projection: %v", err)
	}
	repaired, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(repaired), "# <<< AIGW managed provider <<<\n") {
		t.Fatalf("repair falsely succeeded without restoring the provider terminator:\n%s", repaired)
	}
}

func TestSyncReconcilesCodexConfigWithoutRebindingCredentials(t *testing.T) {
	app, _, secretStore, runner := testApp(t, "")
	target := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMX", Endpoints: configuration.Endpoints{OpenAIResponses: "https://example.test/v1"}}
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Account: "dmx", Client: configuration.ClientCodex, Model: "gpt-test"}
	cfg.Routes[configuration.ClientCodex] = "gpt"
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/usr/local/bin/codex", Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "test-token"); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app, "sync"); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("sync started credential binding plans: %#v", runner.plans)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `model = "gpt-test" # managed by AIGW`) {
		t.Fatalf("sync did not reconcile Codex config:\n%s", data)
	}
}

func TestSyncDiscoversAndProjectsCodexInstalledAfterSetup(t *testing.T) {
	app, _, secretStore, runner := testApp(t, "")
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
	cfg.Profiles["gpt"] = configuration.Profile{
		Label:   "GPT",
		Account: "dmx",
		Client:  configuration.ClientCodex,
		Model:   "gpt-test",
	}
	cfg.Routes[configuration.ClientCodex] = "gpt"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "test-token"); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app, "sync"); err != nil {
		t.Fatalf("sync after installing Codex: %v", err)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("sync started credential binding plans: %#v", runner.plans)
	}
	after, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	adapter := after.Adapters[configuration.ClientCodex]
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

func TestSyncCreatesDefaultCodexProjectionWhenClientIsInstalledAfterManifestSetup(t *testing.T) {
	app, _, secretStore, runner := testApp(t, "")
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)
	if err := execute(t, app, "setup", "--from", manifestPath); err != nil {
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

	if err := execute(t, app, "sync"); err != nil {
		t.Fatalf("sync after installing Codex without an existing config file: %v", err)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("sync started credential binding plans: %#v", runner.plans)
	}
	after, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	adapter := after.Adapters[configuration.ClientCodex]
	if !adapter.Enabled || adapter.Executable != "/usr/local/bin/codex" || len(adapter.Targets) != 1 {
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
	app, out, _, runner := testApp(t, "")
	tokens := map[string]string{}
	app.Secrets = secrets.NewEnvironmentStore(func(key string) string { return tokens[key] })
	app.Discovery = emptyDiscovery{}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)
	if err := execute(t, app, "setup", "--from", manifestPath); err != nil {
		t.Fatalf("initial manifest setup: %v", err)
	}

	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	app.ClaudeSettingsPath = settingsPath
	tokens[secrets.EnvironmentKey("aihubmix")] = "test-token"
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{configuration.ClientClaude: "/usr/local/bin/claude"},
	}}
	before, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := execute(t, app, "sync", "--dry-run", "--json"); err != nil {
		t.Fatalf("preview sync after setting one environment Token: %v", err)
	}
	var preview struct {
		DryRun  bool              `json:"dry_run"`
		Routes  map[string]string `json:"routes"`
		Targets []struct {
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
		configuration.ClientCodex:  "dmxapi-gpt",
	}
	if !maps.Equal(preview.Routes, wantRoutes) {
		t.Fatalf("sync preview routes = %#v, want %#v", preview.Routes, wantRoutes)
	}
	out.Reset()
	if err := execute(t, app, "sync", "--dry-run"); err != nil {
		t.Fatalf("render sync preview after setting one environment Token: %v", err)
	}
	for client, profile := range wantRoutes {
		if !strings.Contains(out.String(), "Route · "+client) || !strings.Contains(out.String(), profile) {
			t.Fatalf("sync preview omitted %s route %s:\n%s", client, profile, out.String())
		}
	}
	afterPreview, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := afterPreview.Routes[configuration.ClientClaude]; got != before.Routes[configuration.ClientClaude] {
		t.Fatalf("dry-run changed Claude route from %q to %q", before.Routes[configuration.ClientClaude], got)
	}
	if _, err := os.Stat(settingsPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry-run wrote Claude settings: %v", err)
	}

	if err := execute(t, app, "sync"); err != nil {
		t.Fatalf("sync after setting one environment Token: %v", err)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("sync started credential binding plans: %#v", runner.plans)
	}
	after, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := after.Routes[configuration.ClientClaude]; got != "aihubmix-claude" {
		t.Fatalf("Claude route = %q, want aihubmix-claude", got)
	}
	if got := after.Routes[configuration.ClientCodex]; got != "dmxapi-gpt" {
		t.Fatalf("Codex route = %q, want dmxapi-gpt", got)
	}
	adapter := after.Adapters[configuration.ClientClaude]
	if !adapter.Enabled || adapter.Executable != "/usr/local/bin/claude" {
		t.Fatalf("Claude adapter after sync = %#v", adapter)
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"ANTHROPIC_BASE_URL": "https://aihubmix.test"`) {
		t.Fatalf("sync did not project the environment-backed Account:\n%s", data)
	}
}

func TestSyncActivatesLateTokenWithoutChangingIndependentRoute(t *testing.T) {
	app, _, secretStore, runner := testApp(t, "")
	app.Discovery = emptyDiscovery{}
	manifestPath := writeConfigurationManifest(t, configurationManifestFixture)
	if err := execute(t, app, "setup", "--from", manifestPath); err != nil {
		t.Fatalf("initial manifest setup: %v", err)
	}
	before, err := app.Config.Load()
	if err != nil {
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

	if err := execute(t, app, "sync"); err != nil {
		t.Fatalf("sync after Token became available: %v", err)
	}
	after, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(after.Routes, before.Routes) {
		t.Fatalf("sync changed independent Routes: got %#v, want %#v", after.Routes, before.Routes)
	}
	if after.Adapters[configuration.ClientClaude].Enabled {
		t.Fatalf("sync activated Claude through an unselected Account: %#v", after.Adapters[configuration.ClientClaude])
	}
	if adapter := after.Adapters[configuration.ClientCodex]; !adapter.Enabled || adapter.Executable == "" || len(adapter.Targets) != 1 {
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
	app, _, _, runner := testApp(t, "")
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
	cfg.Profiles["gpt"] = configuration.Profile{
		Label:   "GPT",
		Account: "dmx",
		Client:  configuration.ClientCodex,
		Model:   "gpt-test",
	}
	cfg.Routes[configuration.ClientCodex] = "gpt"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app, "sync"); err != nil {
		t.Fatalf("sync with an unconnected Account: %v", err)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("sync started credential binding plans: %#v", runner.plans)
	}
	after, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if adapter := after.Adapters[configuration.ClientCodex]; adapter.Enabled {
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

func TestSyncSurfacesCredentialObservationFailure(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt-test")
	want := errors.New("credential observation failed")
	app.Secrets = observationFailureStore{Store: secrets.NewMemoryStore(), err: want}

	if err := execute(t, app, "sync"); err == nil || err.Error() != "Synchronization prerequisites are unavailable" || !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	for _, expected := range []string{
		"Synchronization prerequisites are unavailable",
		"AIGW could not determine which selected Routes can be projected with the currently available clients and credentials.",
		"Configuration and client projections remain unchanged.",
		"aigw doctor",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("output missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), want.Error()) {
		t.Fatalf("output exposes implementation error:\n%s", out.String())
	}
}

func TestVerifyAllRequiresSynchronizedClientAdapters(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMX", Endpoints: configuration.Endpoints{OpenAIResponses: "https://example.test/v1", Anthropic: "https://example.test"}}
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Account: "dmx", Client: configuration.ClientCodex, Model: "gpt-test"}
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "dmx", Client: configuration.ClientClaude, Model: "claude-test"}
	cfg.Routes[configuration.ClientCodex] = "gpt"
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executableFixture(t, "claude")}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "verify-token"); err != nil {
		t.Fatal(err)
	}
	err := execute(t, app, "verify", "--for", "all")
	if err == nil || !strings.Contains(err.Error(), "Full verification requires an enabled Codex adapter with at least one configuration target") {
		t.Fatalf("error = %v", err)
	}
	if _, checkpointErr := app.Config.LoadVerifiedCheckpoint(); checkpointErr == nil {
		t.Fatal("verification checkpoint was written despite failed readiness preflight")
	}
}

func TestSyncDryRunReportsEveryTargetWithoutMutatingProjectionOrCredentials(t *testing.T) {
	app, out, secretStore, runner := testApp(t, "")
	dir := t.TempDir()
	first := filepath.Join(dir, "first.toml")
	second := filepath.Join(dir, "second.toml")
	claudeSettings := filepath.Join(dir, "settings.json")
	app.ClaudeSettingsPath = claudeSettings
	app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{configuration.ClientClaude: "/usr/local/bin/claude"}}}
	for _, target := range []string{first, second} {
		if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{OpenAIResponses: "http://127.0.0.1:8791/v1", Anthropic: "https://gateway.test"}}
	cfg.Profiles["terra"] = configuration.Profile{Label: "GPT-5.6 Terra", Account: "gateway", Client: configuration.ClientCodex, Model: "gpt-5.6-terra"}
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "gateway", Client: configuration.ClientClaude, Model: "claude-test"}
	cfg.Routes[configuration.ClientCodex] = "terra"
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/usr/local/bin/codex", Targets: []string{first, second}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("gateway", "dry-run-token"); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app, "sync", "--dry-run", "--json"); err != nil {
		t.Fatalf("sync --dry-run --json error = %v", err)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("dry-run started credential binding plans: %#v", runner.plans)
	}
	for _, target := range []string{first, second} {
		data, err := os.ReadFile(target)
		if err != nil || string(data) != "model_provider = \"native\"\n" {
			t.Fatalf("dry-run mutated %s: %q, %v", target, data, err)
		}
		if _, err := os.Stat(target + ".aigw-state.json"); !os.IsNotExist(err) {
			t.Fatalf("dry-run wrote sidecar %s: %v", target, err)
		}
	}
	if _, err := os.Stat(claudeSettings); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote Claude settings %s: %v", claudeSettings, err)
	}
	if _, err := os.Stat(claudeSettings + ".aigw-state.json"); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote Claude settings state %s: %v", claudeSettings, err)
	}
	var preview struct {
		DryRun  bool `json:"dry_run"`
		Targets []struct {
			Client string `json:"client"`
			Target string `json:"target"`
			Action string `json:"action"`
		} `json:"targets"`
	}
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatalf("decode sync dry-run JSON: %v\n%s", err, out.String())
	}
	wantTargets := []string{first, second, claudeSettings}
	wantClients := []string{configuration.ClientCodex, configuration.ClientCodex, configuration.ClientClaude}
	wantActions := []string{"initial-project", "initial-project", "project"}
	if !preview.DryRun || len(preview.Targets) != len(wantTargets) {
		t.Fatalf("sync dry-run preview = %#v", preview)
	}
	for index, want := range wantTargets {
		if preview.Targets[index].Client != wantClients[index] {
			t.Fatalf("target %d client = %q, want %q", index, preview.Targets[index].Client, wantClients[index])
		}
		if preview.Targets[index].Action != wantActions[index] {
			t.Fatalf("target %d action = %q, want %q", index, preview.Targets[index].Action, wantActions[index])
		}
		if index < 2 {
			assertSameExistingPath(t, preview.Targets[index].Target, want)
		} else if preview.Targets[index].Target != want {
			t.Fatalf("target %d = %q, want %q", index, preview.Targets[index].Target, want)
		}
	}
}
