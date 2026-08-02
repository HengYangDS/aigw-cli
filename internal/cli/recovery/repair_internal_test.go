package recovery

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/claude"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	surfaceidentity "aigw-cli/internal/surface"
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
	plans := []codex.ProjectionPlan{{Target: "/known", Action: "update"}, {Target: "/explicit", Action: "create"}}
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
	plans := []codex.ProjectionPlan{{Target: "/known", Action: "update"}}

	if err := renderRepairPreview(runtime, true, before, after, discovered, plans); err != nil {
		t.Fatal(err)
	}
	var got repairPreview
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.DryRun || got.ConfigurationAction != "update" || len(got.Codex) != 1 || got.Codex[0].SurfaceID != string(surfaceidentity.CodexHomeDefault) {
		t.Fatalf("preview = %#v", got)
	}
}

func TestRunRepairReportsDiscoveryAndLauncherFailures(t *testing.T) {
	store, cfg := configuredRepairStore(t)
	if err := runRepair(context.Background(), invocation.Context{Config: store}, false, false); err == nil || !strings.Contains(err.Error(), "discovery") {
		t.Fatalf("runRepair() error = %v, want discovery failure", err)
	}

	blockedParent := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blockedParent, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime := invocation.Context{
		Config:         store,
		Discovery:      staticDiscovery{result: discovery.Result{ClaudeExecutable: "/portable/claude"}},
		ClaudeLauncher: claude.Launcher{GOOS: "linux", BinDir: filepath.Join(blockedParent, "bin"), AIGWExecutable: "/portable/aigw"},
	}
	if err := runRepair(context.Background(), runtime, false, false); err == nil {
		t.Fatal("runRepair() error = nil, want launcher failure")
	}
	if saved, err := store.Load(); err != nil || saved.Routes.Default != cfg.Routes.Default {
		t.Fatalf("configuration changed after launcher failure: cfg=%#v error=%v", saved, err)
	}
}

func configuredRepairStore(t *testing.T) (configuration.Store, configuration.Config) {
	t.Helper()
	store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
	cfg := configuration.NewConfig()
	cfg.Accounts["one"] = configuration.Account{Label: "One", Endpoints: configuration.Endpoints{Anthropic: "https://one.test"}}
	cfg.Profiles["one"] = configuration.Profile{Label: "One", Account: "one", Client: configuration.ClientClaude, Models: configuration.Models{configuration.ClientClaude: "claude-test"}}
	cfg.Routes.Default = "one"
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	return store, cfg
}

func TestRepairDesiredConfigDropsUnusableCodexAndKeepsExplicitTargets(t *testing.T) {
	before := configuration.NewConfig()
	before.Accounts["one"] = configuration.Account{Label: "One", Endpoints: configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}}
	before.Profiles["one"] = configuration.Profile{Label: "One", Account: "one", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt"}}
	before.Routes.Default = "one"
	before.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/old"}
	runtime := invocation.Context{Discovery: staticDiscovery{result: discovery.Result{}}}
	after, _, _, _, err := repairDesiredConfig(runtime, before)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := after.Adapters[configuration.ClientCodex]; ok {
		t.Fatalf("unusable adapter remains: %#v", after.Adapters)
	}

	discovered := discovery.Result{Surfaces: []discovery.Surface{
		{ID: string(surfaceidentity.CodexHomeDefault), ConfigPath: "/default-home", Present: true, AutoManaged: true},
	}}
	targets := repairCodexTargets(discovered, []string{"", "/default-home", "/explicit", "/explicit"})
	want := []string{"/default-home", "/explicit"}
	if len(targets) != len(want) || targets[0] != want[0] || targets[1] != want[1] {
		t.Fatalf("targets = %#v, want %#v", targets, want)
	}
}
