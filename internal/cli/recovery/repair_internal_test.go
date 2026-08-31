package recovery

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	surfaceidentity "aigw-cli/internal/surface"
	"aigw-cli/internal/synchronization"
)

type staticDiscovery struct{ result discovery.Result }

func (candidate staticDiscovery) Discover() discovery.Result { return candidate.result }

func TestRenderRepairPreviewIncludesKnownAndExplicitSurfaces(t *testing.T) {
	out := &bytes.Buffer{}
	runtime := invocation.Context{Out: out, RenderOut: out, Width: 120}
	before := configuration.NewConfig()
	after := before.Clone()
	after.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/codex", Targets: []string{"/known", "/explicit"}}
	discovered := discovery.Result{Surfaces: []discovery.Surface{{ID: string(surfaceidentity.CodexHomeDefault), ConfigPath: "/known", Present: true}}}
	plans := []synchronization.ProjectionPlan{
		{Client: configuration.ClientCodex, Target: "/known", Action: "update"},
		{Client: configuration.ClientCodex, Target: "/explicit", Action: "create"},
	}
	if err := renderRepairPreview(runtime, false, before, after, discovered, plans); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Repair preview", "update", string(surfaceidentity.CodexHomeDefault), "codex-home-explicit", "create"} {
		if !bytes.Contains(out.Bytes(), []byte(want)) {
			t.Fatalf("output lacks %q: %s", want, out.String())
		}
	}
}

func TestRenderRepairPreviewJSONReportsTheSameSemanticPlan(t *testing.T) {
	out := &bytes.Buffer{}
	runtime := invocation.Context{Out: out}
	before := configuration.NewConfig()
	after := before.Clone()
	after.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true}
	discovered := discovery.Result{Surfaces: []discovery.Surface{{ID: string(surfaceidentity.CodexHomeDefault), ConfigPath: "/known"}}}
	plans := []synchronization.ProjectionPlan{{Client: configuration.ClientCodex, Target: "/known", Action: "update"}}

	if err := renderRepairPreview(runtime, true, before, after, discovered, plans); err != nil {
		t.Fatal(err)
	}
	var got repairPreview
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.DryRun || got.ConfigurationAction != "update" || len(got.Projections) != 1 || got.Projections[0].Client != configuration.ClientCodex || got.Projections[0].SurfaceID != string(surfaceidentity.CodexHomeDefault) {
		t.Fatalf("preview = %#v", got)
	}
}

func TestRunRepairReportsDiscoveryFailureWithoutMutatingConfiguration(t *testing.T) {
	store, cfg := configuredRepairStore(t)
	if err := runRepair(context.Background(), invocation.Context{Config: store}, false, false); err == nil || !strings.Contains(err.Error(), "discovery") {
		t.Fatalf("runRepair() error = %v, want discovery failure", err)
	}
	if saved, err := store.Load(); err != nil || saved.Routes[configuration.ClientCodex] != cfg.Routes[configuration.ClientCodex] {
		t.Fatalf("configuration changed after discovery failure: cfg=%#v error=%v", saved, err)
	}
}

func configuredRepairStore(t *testing.T) (configuration.Store, configuration.Config) {
	t.Helper()
	store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
	cfg := configuration.NewConfig()
	cfg.Accounts["one"] = configuration.Account{Label: "One", Endpoints: configuration.Endpoints{Anthropic: "https://one.test"}}
	cfg.Profiles["one"] = configuration.Profile{Label: "One", Account: "one", Client: configuration.ClientClaude, Model: "claude-test"}
	cfg.Routes[configuration.ClientClaude] = "one"
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	return store, cfg
}

func TestRepairDesiredConfigDropsUnusableCodexAndKeepsExplicitTargets(t *testing.T) {
	before := configuration.NewConfig()
	before.Accounts["one"] = configuration.Account{Label: "One", Endpoints: configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}}
	before.Profiles["one"] = configuration.Profile{Label: "One", Account: "one", Client: configuration.ClientCodex, Model: "gpt"}
	before.Routes[configuration.ClientCodex] = "one"
	before.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/old"}
	runtime := invocation.Context{Discovery: staticDiscovery{result: discovery.Result{}}}
	after, _, err := invocation.Synchronizer(runtime).DesiredClientConfiguration(before)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := after.Adapters[configuration.ClientCodex]; ok {
		t.Fatalf("unusable adapter remains: %#v", after.Adapters)
	}

}

func TestRunRepairReturnsConfigurationCommitFailure(t *testing.T) {
	store, cfg := configuredRepairStore(t)
	backup := store.Path() + ".bak"
	if err := os.Mkdir(backup, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backup, "blocker"), []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime := invocation.Context{
		Config: store,
		Discovery: staticDiscovery{result: discovery.Result{Executables: map[string]string{
			configuration.ClientClaude: "/opt/claude",
		}}},
		ClaudeSettingsPath: filepath.Join(t.TempDir(), ".claude", "settings.json"),
	}
	if err := runRepair(context.Background(), runtime, false, false); err == nil {
		t.Fatal("configuration commit failure was accepted")
	}
	got, err := store.Load()
	if err != nil || got.Routes[configuration.ClientCodex] != cfg.Routes[configuration.ClientCodex] {
		t.Fatalf("configuration changed: %#v, %v", got, err)
	}
}

func TestRunRepairReturnsDryRunPlanAndConvergedProjectionFailures(t *testing.T) {
	tests := []struct {
		name   string
		dryRun bool
	}{
		{name: "dry-run plan", dryRun: true},
		{name: "converged projection"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
			cfg := configuration.NewConfig()
			cfg.Accounts["one"] = configuration.Account{Label: "One", Endpoints: configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}}
			cfg.Profiles["one"] = configuration.Profile{Label: "One", Account: "one", Client: configuration.ClientCodex, Model: "gpt-test"}
			cfg.Routes[configuration.ClientCodex] = "one"
			missingTarget := filepath.Join(t.TempDir(), "missing", "configuration.toml")
			cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{missingTarget}}
			if err := store.Save(cfg); err != nil {
				t.Fatal(err)
			}
			runtime := invocation.Context{
				Config:    store,
				Discovery: staticDiscovery{result: discovery.Result{}},
				Out:       &bytes.Buffer{},
			}
			if err := runRepair(context.Background(), runtime, test.dryRun, false); err == nil || !strings.Contains(err.Error(), "does not exist") {
				t.Fatalf("repair error = %v", err)
			}
		})
	}
}
