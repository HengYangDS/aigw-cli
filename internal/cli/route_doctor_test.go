package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/config"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/discovery"
)

type rejectingDoctorRunner struct{ calls int }

func (r *rejectingDoctorRunner) Run(_ context.Context, _ adapters.ProcessPlan) error {
	r.calls++
	return errors.New("route doctor must not execute a client")
}

func TestRouteDoctorClassifiesSurfacesWithoutExecutionOrLeaks(t *testing.T) {
	h := newAirRouteHarness(t)
	if err := Execute(h.app, []string{"route", "fallback", "air", "--confirm-host-idle"}); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	pycharm := filepath.Join(root, "pycharm", "config.toml")
	junie := filepath.Join(root, "bin", "junie")
	sentinel := filepath.Join(root, "junie-executed")
	if err := os.MkdirAll(filepath.Dir(pycharm), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pycharm, []byte("model_provider = \"jetbrains\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(junie), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(junie, []byte("#!/bin/sh\ntouch "+sentinel+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	discovered, err := discoveredResult(h.app)
	if err != nil {
		t.Fatal(err)
	}
	discovered.Surfaces = append(discovered.Surfaces,
		discovery.Surface{ID: discovery.SurfacePyCharmCodex, Product: "PyCharm", Authority: discovery.AuthorityJetBrainsAI, ConfigPath: pycharm, Present: true},
		discovery.Surface{ID: discovery.SurfaceJunieCLI, Product: "Junie CLI", Authority: discovery.AuthorityJetBrainsAI, Executable: junie, Present: true},
	)
	h.app.Discovery = reconciliationDiscovery{result: discovered}
	out := new(bytes.Buffer)
	h.app.Out, h.app.Err = out, out
	runner := &rejectingDoctorRunner{}
	h.app.Runner = runner
	configBefore, _ := os.ReadFile(h.app.Config.Path())
	airBefore, _ := os.ReadFile(h.air)
	standaloneBefore, _ := os.ReadFile(h.standalone)

	if err := Execute(h.app, []string{"route", "doctor", "--json"}); err != nil {
		t.Fatal(err)
	}
	if runner.calls != 0 {
		t.Fatalf("runner calls = %d", runner.calls)
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatalf("Junie executed: %v", err)
	}
	configAfter, _ := os.ReadFile(h.app.Config.Path())
	airAfter, _ := os.ReadFile(h.air)
	standaloneAfter, _ := os.ReadFile(h.standalone)
	if !bytes.Equal(configBefore, configAfter) || !bytes.Equal(airBefore, airAfter) || !bytes.Equal(standaloneBefore, standaloneAfter) {
		t.Fatal("route doctor changed persistent files")
	}
	output := out.String()
	for _, forbidden := range []string{h.air, h.standalone, pycharm, junie, "secret", "gateway.test", "model_provider"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("doctor leaked %q: %s", forbidden, output)
		}
	}
	for _, required := range []string{
		discovery.SurfaceCodexCLIStandalone,
		discovery.SurfaceAirCodex,
		discovery.SurfacePyCharmCodex,
		discovery.SurfaceJunieCLI,
		`"fallback": "staged"`,
		`"host_authentication": "not-probed"`,
		`"billing_evidence": "unknown"`,
		`"ok": true`,
	} {
		if !strings.Contains(output, required) {
			t.Fatalf("doctor output missing %q: %s", required, output)
		}
	}
}

func TestRouteDoctorDoesNotTreatExpectedExternalSurfacesAsFailure(t *testing.T) {
	app := &App{
		Config: config.NewStore(filepath.Join(t.TempDir(), "aigw.toml")),
		Discovery: reconciliationDiscovery{result: discovery.Result{Surfaces: []discovery.Surface{
			{ID: discovery.SurfacePyCharmCodex, Product: "PyCharm", Authority: discovery.AuthorityJetBrainsAI, Present: false},
			{ID: discovery.SurfaceJunieCLI, Product: "Junie CLI", Authority: discovery.AuthorityJetBrainsAI, Present: false},
		}}},
	}
	report, err := buildRouteDoctorReport(app)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("report = %#v", report)
	}
}

func TestRouteDoctorRecommendsRepairPreviewForLegacyJetBrainsProjection(t *testing.T) {
	h := newAirRouteHarness(t)
	cfg, err := h.app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	adapter := cfg.Adapters["codex"]
	adapter.Targets = append(adapter.Targets, h.air)
	cfg.Adapters["codex"] = adapter
	if err := h.app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	runtime, _, err := cfg.ResolveRuntime("codex", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := adapters.SyncCodexConfig(h.air, runtime); err != nil {
		t.Fatal(err)
	}
	out := new(bytes.Buffer)
	h.app.Out, h.app.Err = out, out
	if err := Execute(h.app, []string{"route", "doctor"}); err == nil {
		t.Fatal("legacy JetBrains projection unexpectedly passed route doctor")
	}
	text := out.String()
	if !strings.Contains(text, "aigw repair --dry-run") || strings.Contains(text, "aigw route restore air --dry-run") {
		t.Fatalf("route doctor did not recommend the safe legacy repair preview:\n%s", text)
	}
}

func TestRouteDoctorConflictJSONRemainsMachineReadable(t *testing.T) {
	h := newAirRouteHarness(t)
	cfg, err := h.app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	adapter := cfg.Adapters["codex"]
	adapter.Targets = append(adapter.Targets, h.air)
	cfg.Adapters["codex"] = adapter
	if err := h.app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	if err := Execute(h.app, []string{"route", "doctor", "--json"}); err == nil {
		t.Fatal("conflicting Air projection unexpectedly passed route doctor")
	}
	var report routeDoctorReport
	if err := json.Unmarshal(h.app.Out.(*bytes.Buffer).Bytes(), &report); err != nil {
		t.Fatalf("route doctor conflict JSON is not parseable: %v\n%s", err, h.app.Out.(*bytes.Buffer).String())
	}
	if report.OK {
		t.Fatalf("report = %#v, want conflict", report)
	}
}

func TestRouteDoctorRecommendsAirRecoveryForStaleFullSelection(t *testing.T) {
	h := newAirRouteHarness(t)
	stageStaleAirFullSelection(t, h)
	report, err := buildRouteDoctorReport(h.app)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || len(report.Surfaces) == 0 {
		t.Fatalf("report = %#v", report)
	}
	air := routeSurfaceStatus{}
	for _, surface := range report.Surfaces {
		if surface.SurfaceID == discovery.SurfaceAirCodex {
			air = surface
			break
		}
	}
	if air.State != "recoverable-stale-full-selection" {
		t.Fatalf("Air status = %#v", air)
	}
	out := new(bytes.Buffer)
	h.app.Out, h.app.Err = out, out
	if err := Execute(h.app, []string{"route", "doctor"}); err == nil {
		t.Fatal("stale Air full selection unexpectedly passed route doctor")
	}
	text := out.String()
	if !strings.Contains(text, "aigw route recover air --dry-run") {
		t.Fatalf("route doctor output omitted recovery command:\n%s", text)
	}
	if strings.Contains(text, "aigw repair --dry-run") || strings.Contains(text, h.air) {
		t.Fatalf("route doctor gave unsafe recovery guidance:\n%s", text)
	}
}

func TestRouteDoctorAcceptsAirHostMirrorRegardlessOfSurfaceOrderWithoutLeaks(t *testing.T) {
	h := newAirRouteHarness(t)
	standalone, err := os.ReadFile(h.standalone)
	if err != nil {
		t.Fatal(err)
	}
	airMirror := append([]byte(nil), standalone...)
	airMirror = append(airMirror, []byte("\n[jetbrains]\nhost_only = true\n")...)
	if err := os.WriteFile(h.air, airMirror, 0o600); err != nil {
		t.Fatal(err)
	}
	discovered, err := discoveredResult(h.app)
	if err != nil {
		t.Fatal(err)
	}
	if len(discovered.Surfaces) != 2 {
		t.Fatalf("surfaces = %#v", discovered.Surfaces)
	}
	discovered.Surfaces[0], discovered.Surfaces[1] = discovered.Surfaces[1], discovered.Surfaces[0]
	h.app.Discovery = reconciliationDiscovery{result: discovered}

	report, err := buildRouteDoctorReport(h.app)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("report = %#v", report)
	}
	air := routeDoctorSurface(report, discovery.SurfaceAirCodex)
	if air.State != "external-host-mirror" || air.Management != "external-jetbrains" {
		t.Fatalf("Air status = %#v", air)
	}

	out := new(bytes.Buffer)
	h.app.Out, h.app.Err = out, out
	if err := Execute(h.app, []string{"route", "doctor", "--json"}); err != nil {
		t.Fatal(err)
	}
	output := out.String()
	for _, forbidden := range []string{h.air, h.standalone, "gateway.test", "gpt-test", "model_provider", "host_only = true"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("doctor leaked %q: %s", forbidden, output)
		}
	}
	for _, want := range []string{`"state": "external-host-mirror"`, `"management": "external-jetbrains"`, `"ok": true`} {
		if !strings.Contains(output, want) {
			t.Fatalf("doctor output missing %q: %s", want, output)
		}
	}
}

func TestRouteDoctorRejectsAirOrphanAndPartialResidueWithoutLeaks(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(string) string
		wantState string
		secret    string
	}{
		{
			name: "exact orphan",
			mutate: func(text string) string {
				return strings.Replace(text, "https://gateway.test/v1", "https://orphan.test/v1", 1)
			},
			wantState: "orphaned-aigw-marker",
			secret:    "orphan.test",
		},
		{
			name: "partial residue",
			mutate: func(string) string {
				return "model_provider = \"aigw\" # managed by AIGW\n# >>> AIGW managed provider >>>\nbase_url = \"https://partial.test/v1\"\n"
			},
			wantState: "partial-or-foreign-residue",
			secret:    "partial.test",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newAirRouteHarness(t)
			standalone, err := os.ReadFile(h.standalone)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(h.air, []byte(tt.mutate(string(standalone))), 0o600); err != nil {
				t.Fatal(err)
			}
			report, err := buildRouteDoctorReport(h.app)
			if err != nil {
				t.Fatal(err)
			}
			if report.OK {
				t.Fatalf("report = %#v, want conflict", report)
			}
			air := routeDoctorSurface(report, discovery.SurfaceAirCodex)
			if air.State != tt.wantState || air.Management != "external-jetbrains" {
				t.Fatalf("Air status = %#v", air)
			}

			out := new(bytes.Buffer)
			h.app.Out, h.app.Err = out, out
			if err := Execute(h.app, []string{"route", "doctor", "--json"}); err == nil {
				t.Fatal("route doctor unexpectedly accepted Air conflict")
			}
			output := out.String()
			for _, forbidden := range []string{h.air, h.standalone, tt.secret, "gpt-test", "model_provider"} {
				if strings.Contains(output, forbidden) {
					t.Fatalf("doctor leaked %q: %s", forbidden, output)
				}
			}
			if !strings.Contains(output, `"state": "`+tt.wantState+`"`) || !strings.Contains(output, `"ok": false`) {
				t.Fatalf("doctor output = %s", output)
			}
		})
	}
}

func routeDoctorSurface(report routeDoctorReport, surfaceID string) routeSurfaceStatus {
	for _, surface := range report.Surfaces {
		if surface.SurfaceID == surfaceID {
			return surface
		}
	}
	return routeSurfaceStatus{}
}

func TestRouteDoctorIsNotAMutationCommand(t *testing.T) {
	if mutationCommand(&App{}, []string{"route", "doctor", "--json"}) {
		t.Fatal("route doctor must not acquire a mutation lock")
	}
}
