package cli_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	surfaceidentity "aigw-cli/internal/surface"
)

func TestRepairPreservesConfiguredClaudeExecutable(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	claudeExecutable := executableFixture(t, "claude")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "claude", "claude", "Claude", configuration.Endpoints{Anthropic: "https://example.test"}, configuration.ClientClaude, "claude-model")
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: claudeExecutable}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("claude", "token"); err != nil {
		t.Fatal(err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{configuration.ClientClaude: "/different/claude"}}}

	if err := cli.Execute(app, []string{"repair"}); err != nil {
		t.Fatal(err)
	}
	restored, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := restored.Adapters[configuration.ClientClaude].Executable; got != claudeExecutable {
		t.Fatalf("repair replaced configured Claude executable: %q", got)
	}
	if !strings.Contains(out.String(), "Configuration") || !strings.Contains(out.String(), "Synchronized") {
		t.Fatalf("repair did not report configuration reconciliation:\n%s", out.String())
	}
}

func TestRepairCanRestoreClaudeWithoutAnyCodexProfile(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	claudeExecutable := executableFixture(t, "claude")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "claude", "claude", "Claude", configuration.Endpoints{Anthropic: "https://example.test"}, configuration.ClientClaude, "claude-test")
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: claudeExecutable}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("claude", "token"); err != nil {
		t.Fatal(err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{}}

	if err := cli.Execute(app, []string{"repair"}); err != nil {
		t.Fatal(err)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if adapter := got.Adapters[configuration.ClientClaude]; !adapter.Enabled || adapter.Executable != claudeExecutable {
		t.Fatalf("Claude adapter changed during repair: %#v", adapter)
	}
}

func TestRepairHumanPreviewAndDependencyFailures(t *testing.T) {
	t.Run("load", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		app.Config = configuration.NewStore(t.TempDir())
		if err := cli.Execute(app, []string{"repair", "--dry-run"}); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("discovery", func(t *testing.T) {
		app, out, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt")
		app.Discovery = nil
		if err := cli.Execute(app, []string{"repair", "--dry-run"}); err == nil || err.Error() != "Repair prerequisites are unavailable" {
			t.Fatalf("error = %v", err)
		}
		for _, want := range []string{
			"Repair prerequisites are unavailable",
			"AIGW could not inspect the current clients and configuration needed to plan a repair.",
			"Configuration and client projections remain unchanged.",
			"aigw doctor",
		} {
			if !strings.Contains(out.String(), want) {
				t.Fatalf("output missing %q:\n%s", want, out.String())
			}
		}
		if strings.Contains(out.String(), "discovery is unavailable") {
			t.Fatalf("output exposes implementation error:\n%s", out.String())
		}
	})

	t.Run("human preview", func(t *testing.T) {
		app, out, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt")
		if err := cli.Execute(app, []string{"repair", "--dry-run"}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "Repair preview") || !strings.Contains(out.String(), "Preview did not write") {
			t.Fatalf("output = %q", out.String())
		}
	})
}

func TestRepairDiscoversAndEnablesInstalledClients(t *testing.T) {
	app, out, secretStore, runner, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "dmx-claude", "dmx", "DMXAPI", configuration.Endpoints{Anthropic: "https://dmx.test", OpenAIResponses: "https://dmx.test/v1"}, configuration.ClientClaude, "claude-model")
	addAccountProfile(&cfg, "dmx-codex", "dmx", "DMXAPI", configuration.Endpoints{Anthropic: "https://dmx.test", OpenAIResponses: "https://dmx.test/v1"}, configuration.ClientCodex, "gpt-model")
	cfg.Routes[configuration.ClientClaude] = "dmx-claude"
	cfg.Routes[configuration.ClientCodex] = "dmx-codex"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	claudeExecutable := executableFixture(t, "claude")
	target := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{configuration.ClientClaude: claudeExecutable, configuration.ClientCodex: "/opt/codex"}, Surfaces: []discovery.Surface{{
		ID:          string(surfaceidentity.CodexHomeDefault),
		Authority:   string(surfaceidentity.AuthorityAIGW),
		ConfigPath:  target,
		Present:     true,
		AutoManaged: true,
	}}}}
	if err := cli.Execute(app, []string{"repair"}); err != nil {
		t.Fatal(err)
	}
	got, _ := app.Config.Load()
	if !got.Adapters["claude"].Enabled || !got.Adapters["codex"].Enabled || got.Adapters["codex"].Executable != "/opt/codex" || len(runner.plans) != 0 {
		t.Fatalf("repair config=%#v plans=%#v", got, runner.plans)
	}
	if !strings.Contains(out.String(), "Repair completed") || !strings.Contains(out.String(), "Configuration") || !strings.Contains(out.String(), "Synchronized") {
		t.Fatalf("output = %s", out.String())
	}
}

func TestRepairKeepsConfiguredCodexExecutableAcrossTargetChanges(t *testing.T) {
	app, _, secretStore, runner, _ := testApp(t, "")
	trustedExecutable := executableFixture(t, "codex-trusted")
	shadowExecutable := "/tmp/shadow/codex"

	existingTarget := filepath.Join(t.TempDir(), "existing", "configuration.toml")
	newTarget := filepath.Join(t.TempDir(), "discovered", "configuration.toml")
	for _, target := range []string{existingTarget, newTarget} {
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "dmx", "dmx", "DMXAPI", configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"}, configuration.ClientCodex, "gpt-test")
	cfg.Routes[configuration.ClientCodex] = "dmx"
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: trustedExecutable, Targets: []string{existingTarget}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "synthetic-token"); err != nil {
		t.Fatal(err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{configuration.ClientCodex: shadowExecutable},
		Surfaces: []discovery.Surface{{
			ID:          string(surfaceidentity.CodexHomeDefault),
			Authority:   string(surfaceidentity.AuthorityAIGW),
			ConfigPath:  newTarget,
			Present:     true,
			AutoManaged: true,
		}},
	}}

	if err := cli.Execute(app, []string{"repair"}); err != nil {
		t.Fatal(err)
	}
	first, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if first.Adapters[configuration.ClientCodex].Executable != trustedExecutable {
		t.Fatalf("repair replaced configured Codex executable: %#v", first.Adapters[configuration.ClientCodex])
	}
	if len(runner.plans) != 0 {
		t.Fatalf("repair invoked a client: %#v", runner.plans)
	}

	if err := cli.Execute(app, []string{"repair"}); err != nil {
		t.Fatal(err)
	}
	second, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("repeated repair changed configuration:\nfirst=%#v\nsecond=%#v", first, second)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("repeated repair invoked a client: %#v", runner.plans)
	}
}

func TestRepairMigratesMissingClientExecutables(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	missingRoot := t.TempDir()
	oldClaude := filepath.Join(missingRoot, "old-claude")
	oldCodex := filepath.Join(missingRoot, "old-codex")
	newClaude := executableFixture(t, "claude")
	newCodex := executableFixture(t, "codex")
	target := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "claude", "gateway", "Gateway", configuration.Endpoints{Anthropic: "https://gateway.test", OpenAIResponses: "https://gateway.test/v1"}, configuration.ClientClaude, "claude-model")
	addAccountProfile(&cfg, "codex", "gateway", "Gateway", configuration.Endpoints{Anthropic: "https://gateway.test", OpenAIResponses: "https://gateway.test/v1"}, configuration.ClientCodex, "gpt-model")
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Routes[configuration.ClientCodex] = "codex"
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: oldClaude}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: oldCodex, Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{
			configuration.ClientClaude: newClaude,
			configuration.ClientCodex:  newCodex,
		},
		Surfaces: []discovery.Surface{{
			ID:          string(surfaceidentity.CodexHomeDefault),
			Authority:   string(surfaceidentity.AuthorityAIGW),
			ConfigPath:  target,
			Present:     true,
			AutoManaged: true,
		}},
	}}

	if err := cli.Execute(app, []string{"repair"}); err != nil {
		t.Fatalf("repair after clients moved: %v", err)
	}
	after, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := after.Adapters[configuration.ClientClaude].Executable; got != newClaude {
		t.Fatalf("Claude executable = %q, want %q", got, newClaude)
	}
	if got := after.Adapters[configuration.ClientCodex].Executable; got != newCodex {
		t.Fatalf("Codex executable = %q, want %q", got, newCodex)
	}
}

func TestRepairResyncsAnExistingTruncatedCodexProjection(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
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
	if err := cli.Execute(app, []string{"sync"}); err != nil {
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

	if err := cli.Execute(app, []string{"repair"}); err != nil {
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
