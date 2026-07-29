package cli

import (
	"bytes"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/discovery"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

func TestFirstCheckRuntimeFailureAndGenericProfileBranches(t *testing.T) {
	t.Run("missing default", func(t *testing.T) {
		if _, err := firstCheckRuntime(domain.NewConfig()); err == nil {
			t.Fatal("expected missing default route error")
		}
	})

	t.Run("generic without endpoint", func(t *testing.T) {
		cfg := domain.NewConfig()
		cfg.Accounts["one"] = domain.Account{Label: "One"}
		cfg.Profiles["one"] = domain.Profile{Label: "One", Account: "one"}
		cfg.Routes.Default = "one"
		if _, err := firstCheckRuntime(cfg); err == nil {
			t.Fatal("expected no testable endpoint error")
		}
	})

	t.Run("specific resolution failure", func(t *testing.T) {
		cfg := domain.NewConfig()
		cfg.Accounts["one"] = domain.Account{Label: "One"}
		cfg.Profiles["one"] = domain.Profile{Label: "One", Account: "one", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt"}}
		cfg.Routes.Default = "one"
		if _, err := firstCheckRuntime(cfg); err == nil {
			t.Fatal("expected route resolution error")
		}
	})

	t.Run("generic resolves", func(t *testing.T) {
		cfg := domain.NewConfig()
		cfg.Accounts["one"] = domain.Account{Label: "One", Endpoints: domain.Endpoints{Anthropic: "https://one.test"}}
		cfg.Profiles["one"] = domain.Profile{Label: "One", Account: "one", Models: domain.Models{domain.ClientClaude: "claude"}}
		cfg.Routes.Default = "one"
		runtime, err := firstCheckRuntime(cfg)
		if err != nil || runtime.Client != domain.ClientClaude {
			t.Fatalf("runtime=%#v error=%v", runtime, err)
		}
	})
}

func TestRenderRepairPreviewIncludesKnownAndExplicitSurfaces(t *testing.T) {
	out := &bytes.Buffer{}
	app := &App{Out: out, Err: out}
	before := domain.NewConfig()
	after := cloneConfig(before)
	after.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: "/codex", Targets: []string{"/known", "/explicit"}}
	discovered := discovery.Result{Surfaces: []discovery.Surface{{ID: discovery.SurfaceCodexCLIStandalone, ConfigPath: "/known", Present: true}}}
	plans := []adapters.CodexProjectionPlan{{Target: "/known", Action: "update"}, {Target: "/explicit", Action: "create"}}
	if err := renderRepairPreview(app, false, before, after, discovered, plans); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Repair preview", "update", discovery.SurfaceCodexCLIStandalone, "codex-cli-explicit", "create"} {
		if !bytes.Contains(out.Bytes(), []byte(want)) {
			t.Fatalf("output lacks %q: %s", want, out.String())
		}
	}
}

func TestRepairDesiredConfigDropsUnusableCodexAndKeepsExplicitTargets(t *testing.T) {
	before := domain.NewConfig()
	before.Accounts["one"] = domain.Account{Label: "One", Endpoints: domain.Endpoints{OpenAIResponses: "https://one.test/v1"}}
	before.Profiles["one"] = domain.Profile{Label: "One", Account: "one", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt"}}
	before.Routes.Default = "one"
	before.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: "/old"}
	app := &App{Discovery: reconciliationDiscovery{result: discovery.Result{}}}
	after, _, _, _, err := repairDesiredConfig(app, before)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := after.Adapters[domain.ClientCodex]; ok {
		t.Fatalf("unusable adapter remains: %#v", after.Adapters)
	}

	discovered := discovery.Result{Surfaces: []discovery.Surface{
		{ID: discovery.SurfaceCodexCLIStandalone, ConfigPath: "/standalone", Present: true, AutoManaged: true},
		{ID: discovery.SurfaceAirCodex, ConfigPath: "/air", Present: true},
	}}
	targets := repairCodexTargets(discovered, []string{"", "/standalone", "/air", "/explicit", "/explicit"})
	want := []string{"/standalone", "/explicit"}
	if len(targets) != len(want) || targets[0] != want[0] || targets[1] != want[1] {
		t.Fatalf("targets = %#v, want %#v", targets, want)
	}
}
