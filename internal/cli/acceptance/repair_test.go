package cli_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/client/claude"
	"aigw-cli/internal/client/codex"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	surfaceidentity "aigw-cli/internal/surface"
)

func TestRepairPreservesConfiguredClaudeExecutable(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	claudeExecutable := executableFixture(t, "claude")
	cfg := configuration.NewConfig(clients.Admission())
	addAccountRoute(&cfg, "claude", "claude", "Claude", configuration.Endpoints{Anthropic: "https://example.test"}, claude.ID, "claude-model")
	cfg.SetSelectedRoute(claude.ID, "claude", "")
	cfg.SetClientActivation(claude.ID, true, claudeExecutable, nil)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("claude", "token"); err != nil {
		t.Fatal(err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{claude.ID: "/different/claude"}}}

	if err := cli.Execute(app, []string{"repair"}); err != nil {
		t.Fatal(err)
	}
	restored, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := restored.Clients[claude.ID].Executable; got != claudeExecutable {
		t.Fatalf("repair replaced configured Claude executable: %q", got)
	}
	if !strings.Contains(out.String(), "Configuration") || !strings.Contains(out.String(), "Synchronized") {
		t.Fatalf("repair did not report configuration reconciliation:\n%s", out.String())
	}
}

func TestRepairCanRestoreClaudeWithoutAnyCodexRoute(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	claudeExecutable := executableFixture(t, "claude")
	cfg := configuration.NewConfig(clients.Admission())
	addAccountRoute(&cfg, "claude", "claude", "Claude", configuration.Endpoints{Anthropic: "https://example.test"}, claude.ID, "claude-test")
	cfg.SetSelectedRoute(claude.ID, "claude", "")
	cfg.SetClientActivation(claude.ID, true, claudeExecutable, nil)
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
	if adapter := got.Clients[claude.ID]; !adapter.Enabled || adapter.Executable != claudeExecutable {
		t.Fatalf("Claude adapter changed during repair: %#v", adapter)
	}
}

func TestRepairHumanPreviewAndDependencyFailures(t *testing.T) {
	t.Run("load", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		app.Config = configuration.NewStore(t.TempDir(), clients.Admission())
		if err := cli.Execute(app, []string{"repair", "--dry-run"}); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("discovery", func(t *testing.T) {
		app, out, _, _, _ := testApp(t, "")
		saveCommandRoute(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, codex.ID, "gpt")
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
		saveCommandRoute(t, app, configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, codex.ID, "gpt")
		if err := cli.Execute(app, []string{"repair", "--dry-run"}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "Repair preview") || !strings.Contains(out.String(), "Preview did not write") {
			t.Fatalf("output = %q", out.String())
		}
	})
}

func TestRepairRefreshesExplicitlyEnabledClients(t *testing.T) {
	app, out, secretStore, runner, _ := testApp(t, "")
	cfg := configuration.NewConfig(clients.Admission())
	addAccountRoute(&cfg, "dmx-claude", "dmx", "DMXAPI", configuration.Endpoints{Anthropic: "https://dmx.test", OpenAIResponses: "https://dmx.test/v1"}, claude.ID, "claude-model")
	addAccountRoute(&cfg, "dmx-codex", "dmx", "DMXAPI", configuration.Endpoints{Anthropic: "https://dmx.test", OpenAIResponses: "https://dmx.test/v1"}, codex.ID, "gpt-model")
	cfg.SetSelectedRoute(claude.ID, "dmx-claude", "")
	cfg.SetSelectedRoute(codex.ID, "dmx-codex", "")
	cfg.SetClientActivation(claude.ID, true, "", nil)
	cfg.SetClientActivation(codex.ID, true, "", nil)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "token")
	claudeExecutable := executableFixture(t, "claude")
	target := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{claude.ID: claudeExecutable, codex.ID: "/opt/codex"}, Surfaces: []discovery.Surface{{
		ID:          string(codex.HomeDefault),
		Authority:   string(surfaceidentity.AuthorityAIGW),
		ConfigPath:  target,
		Present:     true,
		AutoManaged: true,
	}}}}
	if err := cli.Execute(app, []string{"repair"}); err != nil {
		t.Fatal(err)
	}
	got, _ := app.Config.Load()
	if !got.Clients["claude"].Enabled || !got.Clients["codex"].Enabled || got.Clients["codex"].Executable != "/opt/codex" || len(runner.plans) != 0 {
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

	cfg := configuration.NewConfig(clients.Admission())
	addAccountRoute(&cfg, "dmx", "dmx", "DMXAPI", configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"}, codex.ID, "gpt-test")
	cfg.SetSelectedRoute(codex.ID, "dmx", "")
	cfg.SetClientActivation(codex.ID, true, trustedExecutable, []string{existingTarget})
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "synthetic-token"); err != nil {
		t.Fatal(err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{codex.ID: shadowExecutable},
		Surfaces: []discovery.Surface{{
			ID:          string(codex.HomeDefault),
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
	if first.Clients[codex.ID].Executable != trustedExecutable {
		t.Fatalf("repair replaced configured Codex executable: %#v", first.Clients[codex.ID])
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

	cfg := configuration.NewConfig(clients.Admission())
	addAccountRoute(&cfg, "claude", "gateway", "Gateway", configuration.Endpoints{Anthropic: "https://gateway.test", OpenAIResponses: "https://gateway.test/v1"}, claude.ID, "claude-model")
	addAccountRoute(&cfg, "codex", "gateway", "Gateway", configuration.Endpoints{Anthropic: "https://gateway.test", OpenAIResponses: "https://gateway.test/v1"}, codex.ID, "gpt-model")
	cfg.SetSelectedRoute(claude.ID, "claude", "")
	cfg.SetSelectedRoute(codex.ID, "codex", "")
	cfg.SetClientActivation(claude.ID, true, oldClaude, nil)
	cfg.SetClientActivation(codex.ID, true, oldCodex, []string{target})
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	app.Discovery = fakeDiscovery{result: discovery.Result{
		Executables: map[string]string{
			claude.ID: newClaude,
			codex.ID:  newCodex,
		},
		Surfaces: []discovery.Surface{{
			ID:          string(codex.HomeDefault),
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
	if got := after.Clients[claude.ID].Executable; got != newClaude {
		t.Fatalf("Claude executable = %q, want %q", got, newClaude)
	}
	if got := after.Clients[codex.ID].Executable; got != newCodex {
		t.Fatalf("Codex executable = %q, want %q", got, newCodex)
	}
}

func TestRepairResyncsAnExistingTruncatedCodexProjection(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	target := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\nmodel = \"gpt-original\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := configuration.NewConfig(clients.Admission())
	addAccountRoute(&cfg, "dmx", "dmx", "DMXAPI", configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"}, codex.ID, "gpt-5.6-terra")
	cfg.SetSelectedRoute(codex.ID, "dmx", "")
	cfg.SetClientActivation(codex.ID, true, "/opt/codex", []string{target})
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
	app.Discovery = fakeDiscovery{result: discovery.Result{Executables: map[string]string{codex.ID: "/opt/codex"}, Surfaces: []discovery.Surface{{
		ID:          string(codex.HomeDefault),
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
