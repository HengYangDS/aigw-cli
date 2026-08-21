package cli_test

import (
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	surfaceidentity "aigw-cli/internal/surface"
	"encoding/json"
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
	addAccountProfile(&cfg, "dmx", "dmx", "DMXAPI", configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"}, configuration.ClientCodex, configuration.Models{configuration.ClientCodex: "gpt-5.6-terra"})
	cfg.Routes.Default = "dmx"
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
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Account: "dmx", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-test"}}
	cfg.Routes.Default = "gpt"
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
		Models:  configuration.Models{configuration.ClientCodex: "gpt-test"},
	}
	cfg.Routes.Default = "gpt"
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
		Models:  configuration.Models{configuration.ClientCodex: "gpt-test"},
	}
	cfg.Routes.Default = "gpt"
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

func TestVerifyAllRequiresSynchronizedClientAdapters(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMX", Endpoints: configuration.Endpoints{OpenAIResponses: "https://example.test/v1", Anthropic: "https://example.test"}}
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Account: "dmx", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-test"}}
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "dmx", Client: configuration.ClientClaude, Models: configuration.Models{configuration.ClientClaude: "claude-test"}}
	cfg.Routes.Default = "gpt"
	cfg.Routes.Overrides[configuration.ClientClaude] = "claude"
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
	for _, target := range []string{first, second} {
		if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{OpenAIResponses: "http://127.0.0.1:8791/v1"}}
	cfg.Profiles["terra"] = configuration.Profile{Label: "GPT-5.6 Terra", Account: "gateway", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-5.6-terra"}}
	cfg.Routes.Default = "terra"
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
	var preview struct {
		DryRun  bool `json:"dry_run"`
		Targets []struct {
			Target string `json:"target"`
			Action string `json:"action"`
		} `json:"targets"`
	}
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatalf("decode sync dry-run JSON: %v\n%s", err, out.String())
	}
	wantTargets := []string{first, second}
	if !preview.DryRun || len(preview.Targets) != len(wantTargets) {
		t.Fatalf("sync dry-run preview = %#v", preview)
	}
	for index, want := range wantTargets {
		if preview.Targets[index].Action != "initial-project" {
			t.Fatalf("target %d action = %q, want initial-project", index, preview.Targets[index].Action)
		}
		assertSameExistingPath(t, preview.Targets[index].Target, want)
	}
}
