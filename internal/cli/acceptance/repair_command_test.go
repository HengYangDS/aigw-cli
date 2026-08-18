package cli_test

import (
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/process"
	surfaceidentity "aigw-cli/internal/surface"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type repairLoginRunner struct {
	trustedExecutable string
	shadowExecutable  string
	trustedCalls      int
	shadowCalls       int
	stdinBytes        []int
}

func (runner *repairLoginRunner) Run(_ context.Context, plan process.Plan) error {
	switch plan.Executable {
	case runner.trustedExecutable:
		runner.trustedCalls++
	case runner.shadowExecutable:
		runner.shadowCalls++
	}
	runner.stdinBytes = append(runner.stdinBytes, len(plan.Stdin))
	return nil
}

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
	if err := execute(t, app, "repair"); err != nil {
		t.Fatal(err)
	}
	got, _ := app.Config.Load()
	if !got.Adapters["claude"].Enabled || !got.Adapters["codex"].Enabled || got.Adapters["codex"].Executable != "/opt/codex" || len(runner.plans) != 1 {
		t.Fatalf("repair config=%#v plans=%#v", got, runner.plans)
	}
	if !strings.Contains(out.String(), "Repair completed") {
		t.Fatalf("output = %s", out.String())
	}
}

func TestRepairKeepsConfiguredCodexExecutableAcrossTargetChanges(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	trustedExecutable := "/opt/codex-trusted"
	shadowExecutable := "/tmp/shadow/codex"
	runner := &repairLoginRunner{trustedExecutable: trustedExecutable, shadowExecutable: shadowExecutable}
	app.Runner = runner

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
	addAccountProfile(&cfg, "dmx", "dmx", "DMXAPI", configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"}, configuration.ClientCodex, configuration.Models{configuration.ClientCodex: "gpt-test"})
	cfg.Routes.Default = "dmx"
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

	if err := execute(t, app, "repair"); err != nil {
		t.Fatal(err)
	}
	first, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if first.Adapters[configuration.ClientCodex].Executable != trustedExecutable {
		t.Fatalf("repair replaced configured Codex executable: %#v", first.Adapters[configuration.ClientCodex])
	}
	if runner.shadowCalls != 0 || runner.trustedCalls != 2 {
		t.Fatalf("authentication calls: trusted=%d shadow=%d", runner.trustedCalls, runner.shadowCalls)
	}
	for _, stdinBytes := range runner.stdinBytes {
		if stdinBytes == 0 {
			t.Fatal("authentication did not receive synthetic input")
		}
	}

	if err := execute(t, app, "repair"); err != nil {
		t.Fatal(err)
	}
	second, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("repeated repair changed configuration:\nfirst=%#v\nsecond=%#v", first, second)
	}
	if runner.shadowCalls != 0 || runner.trustedCalls != 2 {
		t.Fatalf("repeated repair rebound authentication: trusted=%d shadow=%d", runner.trustedCalls, runner.shadowCalls)
	}
}
