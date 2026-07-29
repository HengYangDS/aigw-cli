package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/config"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/discovery"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

func TestRouteDoctorCommandCoverageErrorsAndSuccess(t *testing.T) {
	t.Run("config read", func(t *testing.T) {
		app := &App{Config: config.NewStore(t.TempDir()), Discovery: reconciliationDiscovery{}, Out: &bytes.Buffer{}}
		cmd := newRouteDoctorCommand(app)
		cmd.SetArgs([]string{"--json"})
		if err := cmd.Execute(); err == nil {
			t.Fatal("expected config read error")
		}
	})

	t.Run("discovery", func(t *testing.T) {
		app := &App{Config: config.NewStore(filepath.Join(t.TempDir(), "config.toml"))}
		if _, err := buildRouteDoctorReport(app); err == nil {
			t.Fatal("expected discovery error")
		}
	})

	t.Run("JSON output", func(t *testing.T) {
		want := errors.New("output failed")
		app := &App{
			Config:    config.NewStore(filepath.Join(t.TempDir(), "config.toml")),
			Discovery: reconciliationDiscovery{},
			Out:       attestFailWriter{err: want},
		}
		cmd := newRouteDoctorCommand(app)
		cmd.SetArgs([]string{"--json"})
		if err := cmd.Execute(); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("healthy human report", func(t *testing.T) {
		out := &bytes.Buffer{}
		renderRouteDoctorReport(&App{Out: out}, routeDoctorReport{OK: true})
		if !strings.Contains(out.String(), "No route ownership conflict") {
			t.Fatalf("output = %q", out.String())
		}
	})
}

func TestRouteDoctorConfiguredSurfaceClassifications(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "missing", "config.toml")
	standalone := filepath.Join(root, "standalone", "config.toml")
	pycharm := filepath.Join(root, "pycharm", "config.toml")
	for _, path := range []string{standalone, pycharm} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("model_provider = \"external\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cfg := dailyCoverageConfig()
	cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Targets: []string{missing, standalone, pycharm}}
	app := dailyCoverageApp(t, cfg)
	app.Discovery = reconciliationDiscovery{result: discovery.Result{Surfaces: []discovery.Surface{
		{ID: discovery.SurfaceAirCodex, Authority: discovery.AuthorityJetBrainsAI, ConfigPath: missing, Present: false},
		{ID: discovery.SurfaceCodexCLIStandalone, Authority: discovery.AuthorityAIGW, ConfigPath: standalone, Present: true},
		{ID: discovery.SurfacePyCharmCodex, Authority: discovery.AuthorityJetBrainsAI, ConfigPath: pycharm, Present: true},
	}}}
	app.DataDir = t.TempDir()
	report, err := buildRouteDoctorReport(app)
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]string{}
	for _, surface := range report.Surfaces {
		states[surface.SurfaceID] = surface.State
	}
	if states[discovery.SurfaceAirCodex] != "configured-surface-missing" || states[discovery.SurfaceCodexCLIStandalone] != "external" || states[discovery.SurfacePyCharmCodex] != "forbidden-aigw-target-membership" {
		t.Fatalf("states = %#v", states)
	}
}

func TestRouteDoctorUnlistedManagedClassifications(t *testing.T) {
	cfg := dailyCoverageConfig()
	runtime, _, err := cfg.ResolveRuntime(domain.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	standalone := filepath.Join(root, "standalone", "config.toml")
	pycharm := filepath.Join(root, "pycharm", "config.toml")
	for _, path := range []string{standalone, pycharm} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("model_provider = \"native\"\nmodel = \"gpt-original\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := adapters.SyncCodexConfig(path, runtime); err != nil {
			t.Fatal(err)
		}
	}
	app := dailyCoverageApp(t, cfg)
	app.Discovery = reconciliationDiscovery{result: discovery.Result{Surfaces: []discovery.Surface{
		{ID: discovery.SurfaceCodexCLIStandalone, Authority: discovery.AuthorityAIGW, ConfigPath: standalone, Present: true},
		{ID: discovery.SurfacePyCharmCodex, Authority: discovery.AuthorityJetBrainsAI, ConfigPath: pycharm, Present: true},
	}}}
	report, err := buildRouteDoctorReport(app)
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]string{}
	for _, surface := range report.Surfaces {
		states[surface.SurfaceID] = surface.State
	}
	if states[discovery.SurfaceCodexCLIStandalone] != "unlisted-aigw-projection" || states[discovery.SurfacePyCharmCodex] != "aigw-managed" {
		t.Fatalf("states = %#v", states)
	}
}

func TestRouteDoctorAirMembershipClassifications(t *testing.T) {
	t.Run("listed without fallback", func(t *testing.T) {
		h := newAirRouteHarness(t)
		cfg, err := h.app.Config.Load()
		if err != nil {
			t.Fatal(err)
		}
		adapter := cfg.Adapters[domain.ClientCodex]
		adapter.Targets = addSortedTarget(adapter.Targets, h.air)
		cfg.Adapters[domain.ClientCodex] = adapter
		if err := h.app.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		report, err := buildRouteDoctorReport(h.app)
		if err != nil {
			t.Fatal(err)
		}
		if got := routeDoctorSurface(report, discovery.SurfaceAirCodex).State; got != "listed-without-valid-fallback" {
			t.Fatalf("Air state = %q", got)
		}
	})

	t.Run("stale unlisted fallback", func(t *testing.T) {
		h := newAirRouteHarness(t)
		if err := Execute(h.app, []string{"route", "fallback", "air", "--confirm-host-idle", "--json"}); err != nil {
			t.Fatal(err)
		}
		cfg, err := h.app.Config.Load()
		if err != nil {
			t.Fatal(err)
		}
		adapter := cfg.Adapters[domain.ClientCodex]
		adapter.Targets = removeTarget(adapter.Targets, h.air)
		cfg.Adapters[domain.ClientCodex] = adapter
		if err := h.app.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		report, err := buildRouteDoctorReport(h.app)
		if err != nil {
			t.Fatal(err)
		}
		if got := routeDoctorSurface(report, discovery.SurfaceAirCodex).State; got != "stale-unlisted-fallback" {
			t.Fatalf("Air state = %q", got)
		}
	})
}
