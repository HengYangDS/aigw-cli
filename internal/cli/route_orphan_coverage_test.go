package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/config"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/discovery"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/recovery"
)

func TestOrphanRouteCommandsRejectOtherSurfaces(t *testing.T) {
	for _, verb := range []string{"recover-orphan", "settle"} {
		t.Run(verb, func(t *testing.T) {
			out := &bytes.Buffer{}
			store := config.NewStore(filepath.Join(t.TempDir(), "config.toml"))
			err := Execute(&App{Config: store, Out: out, Err: out}, []string{"route", verb, "other"})
			if err == nil || !strings.Contains(err.Error(), "only `aigw route") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestAirOrphanRunDependencyErrors(t *testing.T) {
	t.Run("recover surface", func(t *testing.T) {
		if err := runAirRecoverOrphan(&App{}, true, false, "", false, false); err == nil {
			t.Fatal("expected surface discovery error")
		}
	})

	t.Run("recover config", func(t *testing.T) {
		h := newAirOrphanCLIHarness(t)
		h.app.Config = config.NewStore(t.TempDir())
		if err := runAirRecoverOrphan(h.app, true, false, "", false, false); err == nil {
			t.Fatal("expected config load error")
		}
	})

	t.Run("settle surface", func(t *testing.T) {
		if err := runAirSettle(&App{}, true, false, "case"); err == nil {
			t.Fatal("expected surface discovery error")
		}
	})

	t.Run("settle case", func(t *testing.T) {
		h := newAirOrphanCLIHarness(t)
		if err := runAirSettle(h.app, true, false, ""); err == nil || !strings.Contains(err.Error(), "--case-id") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestResolveAirRecoverySurfacesRequiresStandalone(t *testing.T) {
	t.Run("air error", func(t *testing.T) {
		if _, _, err := resolveAirRecoverySurfaces(&App{}); err == nil {
			t.Fatal("expected Air discovery error")
		}
	})

	t.Run("standalone missing", func(t *testing.T) {
		app := &App{Discovery: reconciliationDiscovery{result: discovery.Result{Surfaces: []discovery.Surface{{
			ID: discovery.SurfaceAirCodex, Authority: discovery.AuthorityJetBrainsAI, ConfigPath: "/air", Present: true, ManualFallbackAllowed: true,
		}}}}}
		if _, _, err := resolveAirRecoverySurfaces(app); err == nil || !strings.Contains(err.Error(), "standalone") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestAirRecoveryStoreDirectoryResolution(t *testing.T) {
	t.Run("platform", func(t *testing.T) {
		home := t.TempDir()
		if _, err := airRecoveryStore(&App{GOOS: "linux", Env: []string{"HOME=" + home}}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("config fallback", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.toml")
		if _, err := airRecoveryStore(&App{GOOS: "unsupported", Config: config.NewStore(path)}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("unavailable", func(t *testing.T) {
		if _, err := airRecoveryStore(&App{GOOS: "unsupported"}); err == nil {
			t.Fatal("expected unavailable data directory")
		}
	})
}

func TestAirOrphanHumanRenderers(t *testing.T) {
	tests := []struct {
		name string
		run  func(*App) error
		want string
	}{
		{name: "recovery plan", run: func(app *App) error {
			return renderAirRecoveryPlan(app, false, recovery.AirRecoveryPlan{SurfaceID: discovery.SurfaceAirCodex, State: "orphan", Action: "quarantine", CaseID: "case-1"})
		}, want: "Air orphan recovery preview"},
		{name: "recovery receipt", run: func(app *App) error {
			return renderAirRecoveryReceipt(app, false, recovery.AirRecoveryReceipt{SurfaceID: discovery.SurfaceAirCodex, State: recovery.AirRecoveryStateQuarantined, CaseID: "case-1"})
		}, want: "quarantined and removed"},
		{name: "settlement plan", run: func(app *App) error {
			return renderAirSettlementPlan(app, false, recovery.AirSettlementPlan{SurfaceID: discovery.SurfaceAirCodex, State: recovery.AirRecoveryStateQuarantined, Action: "inspect", CaseID: "case-1"})
		}, want: "settlement preview"},
		{name: "settled receipt", run: func(app *App) error {
			return renderAirSettlementReceipt(app, false, recovery.AirSettlementReceipt{SurfaceID: discovery.SurfaceAirCodex, State: recovery.AirRecoveryStateSettled, CaseID: "case-1"})
		}, want: "host roundtrip was accepted"},
		{name: "quarantined receipt", run: func(app *App) error {
			return renderAirSettlementReceipt(app, false, recovery.AirSettlementReceipt{SurfaceID: discovery.SurfaceAirCodex, State: recovery.AirRecoveryStateQuarantined, CaseID: "case-1"})
		}, want: "remains quarantined"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			if err := test.run(&App{Out: out, Err: out}); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), test.want) || !strings.Contains(out.String(), "case-1") {
				t.Fatalf("output = %q", out.String())
			}
		})
	}
}
