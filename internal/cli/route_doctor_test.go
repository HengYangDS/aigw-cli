package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/config"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/discovery"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/recovery"
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

func TestRouteDoctorExplainsUnboundAirResidueWithoutSuggestingAnAIGWWrite(t *testing.T) {
	h := newAirRouteHarness(t)
	if err := os.WriteFile(h.air, []byte("model_provider = \"aigw\" # managed by AIGW\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := buildRouteDoctorReport(h.app)
	if err != nil {
		t.Fatal(err)
	}
	air := routeDoctorSurface(report, discovery.SurfaceAirCodex)
	if air.State != "partial-or-foreign-residue" || air.DiskSelection != "aigw-managed" {
		t.Fatalf("Air status = %#v, want unbound managed residue", air)
	}
	out := new(bytes.Buffer)
	h.app.Out, h.app.Err = out, out

	if err := Execute(h.app, []string{"route", "doctor"}); err == nil {
		t.Fatal("unbound Air residue unexpectedly passed route doctor")
	}
	assertUnboundAirBoundaryGuidance(t, out.String(), h.air)
}

func TestRouteDoctorExplainsLegacyOrphanedAirMarkerWithoutSuggestingAnAIGWWrite(t *testing.T) {
	out := new(bytes.Buffer)
	app := &App{Out: out, Err: out}
	renderRouteDoctorReport(app, routeDoctorReport{
		OK: false,
		Surfaces: []routeSurfaceStatus{{
			SurfaceID: discovery.SurfaceAirCodex,
			Product:   "JetBrains Air Codex",
			State:     "orphaned-aigw-marker",
		}},
	})
	assertUnboundAirBoundaryGuidance(t, out.String(), "/private/air/config.toml")
}

func assertUnboundAirBoundaryGuidance(t *testing.T, text string, privatePath string) {
	t.Helper()
	for _, want := range []string{
		"Air has unbound AIGW residue",
		"No AIGW mutation is admitted",
		"aigw route doctor --json",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("route doctor output missing %q:\n%s", want, text)
		}
	}
	for _, forbidden := range []string{
		"aigw repair --dry-run",
		"aigw route recover air --dry-run",
		"aigw route recover-orphan air --dry-run --json",
		"aigw check",
		privatePath,
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("route doctor gave unsafe unbound-residue guidance %q:\n%s", forbidden, text)
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
			wantState: "orphaned-exact-full-selection",
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

func TestRouteDoctorReportsBoundedRecoveryHealthReasonCodesWithoutWrites(t *testing.T) {
	tests := []struct {
		name       string
		prepare    bool
		mutate     func(t *testing.T, ledgerPath, quarantinePath string)
		wantState  string
		wantHealth string
		wantReason string
	}{
		{name: "ledger missing", wantState: "none", wantHealth: "inactive", wantReason: "ledger-missing"},
		{
			name: "ledger invalid", prepare: true,
			mutate: func(t *testing.T, ledgerPath, _ string) {
				if err := os.WriteFile(ledgerPath, []byte("{\"private_url\":\"https://doctor-secret.invalid/v1\",\"private_path\":\"/private/recovery/case\",\"credential\":\"aigw-test-private-doctor-credential\"}\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			wantState: "unknown", wantHealth: "invalid", wantReason: "ledger-invalid",
		},
		{
			name: "ledger permission invalid", prepare: true,
			mutate: func(t *testing.T, ledgerPath, _ string) {
				if runtime.GOOS == "windows" {
					t.Skip("POSIX permission contract")
				}
				if err := os.Chmod(ledgerPath, 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantState: "unknown", wantHealth: "invalid", wantReason: "ledger-permission-invalid",
		},
		{
			name: "quarantine missing", prepare: true,
			mutate: func(t *testing.T, _, quarantinePath string) {
				if err := os.Remove(quarantinePath); err != nil {
					t.Fatal(err)
				}
			},
			wantState: "awaiting-host-roundtrip", wantHealth: "invalid", wantReason: "quarantine-missing",
		},
		{
			name: "quarantine invalid", prepare: true,
			mutate: func(t *testing.T, _, quarantinePath string) {
				if err := os.WriteFile(quarantinePath, []byte("https://doctor-secret.invalid/v1\n/private/recovery/case\naigw-test-private-doctor-credential\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			wantState: "awaiting-host-roundtrip", wantHealth: "invalid", wantReason: "quarantine-invalid",
		},
		{
			name: "quarantine permission invalid", prepare: true,
			mutate: func(t *testing.T, _, quarantinePath string) {
				if runtime.GOOS == "windows" {
					t.Skip("POSIX permission contract")
				}
				if err := os.Chmod(quarantinePath, 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantState: "awaiting-host-roundtrip", wantHealth: "invalid", wantReason: "quarantine-permission-invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newAirOrphanCLIHarness(t)
			caseID := ""
			ledgerPath := filepath.Join(h.dataDir, "recovery", "air", "ledger.json")
			quarantinePath := ""
			private := []string{
				h.air, h.standalone, h.dataDir,
				"https://doctor-secret.invalid/v1", "/private/recovery/case", "aigw-test-private-doctor-credential",
				"orphan.test", "gateway.test", "ledger.json", "config.toml",
			}
			if tt.prepare {
				caseID = previewAirOrphanCase(t, &h).CaseID
				h.app.Out.(*bytes.Buffer).Reset()
				if err := Execute(h.app, []string{
					"route", "recover-orphan", "air", "--case-id", caseID,
					"--confirm-host-idle", "--ack-unset-external-selection", "--json",
				}); err != nil {
					t.Fatal(err)
				}
				quarantinePath = filepath.Join(h.dataDir, "recovery", "air", "quarantine", caseID, "config.toml")
				private = append(private, caseID)
				ledgerData, err := os.ReadFile(ledgerPath)
				if err != nil {
					t.Fatal(err)
				}
				var ledger map[string]any
				if err := json.Unmarshal(ledgerData, &ledger); err != nil {
					t.Fatal(err)
				}
				for key, value := range ledger {
					if strings.HasSuffix(key, "_sha256") {
						if digest, ok := value.(string); ok {
							private = append(private, digest)
							if len(digest) >= 12 {
								private = append(private, digest[:12])
							}
						}
					}
				}
			}
			if tt.mutate != nil {
				tt.mutate(t, ledgerPath, quarantinePath)
			}
			tracked := []string{h.air, h.standalone, h.app.Config.Path(), ledgerPath, quarantinePath}
			before := captureDoctorFiles(t, tracked)
			h.app.Out.(*bytes.Buffer).Reset()
			_ = Execute(h.app, []string{"route", "doctor", "--json"})
			output := h.app.Out.(*bytes.Buffer).Bytes()
			after := captureDoctorFiles(t, tracked)
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("route doctor changed recovery inputs:\nbefore=%#v\nafter=%#v", before, after)
			}
			if !tt.prepare {
				if _, err := os.Stat(filepath.Join(h.dataDir, "recovery")); !os.IsNotExist(err) {
					t.Fatalf("route doctor created recovery storage: %v", err)
				}
			}
			var report struct {
				Surfaces []map[string]any `json:"surfaces"`
				OK       bool             `json:"ok"`
			}
			if err := json.Unmarshal(output, &report); err != nil {
				t.Fatalf("doctor recovery health is not machine-readable JSON: %v\n%s", err, output)
			}
			if report.OK {
				t.Fatalf("unsettled or invalid recovery storage passed doctor: %s", output)
			}
			var air map[string]any
			for _, surface := range report.Surfaces {
				if surface["surface_id"] == discovery.SurfaceAirCodex {
					air = surface
					break
				}
			}
			if air == nil {
				t.Fatalf("Air surface missing: %s", output)
			}
			if air["recovery_state"] != tt.wantState || air["recovery_health"] != tt.wantHealth || air["recovery_reason_code"] != tt.wantReason {
				t.Fatalf("Air recovery health = %#v, want state=%q health=%q reason=%q", air, tt.wantState, tt.wantHealth, tt.wantReason)
			}
			gotKeys := make([]string, 0, len(air))
			for key := range air {
				gotKeys = append(gotKeys, key)
			}
			sort.Strings(gotKeys)
			wantKeys := []string{
				"attribution_state", "baseline_authority", "billing_evidence", "disk_selection", "fallback",
				"host_authentication", "management", "observed_endpoint_hop", "present", "product", "projection_mode",
				"recovery_health", "recovery_reason_code", "recovery_state", "session_metadata", "state", "surface_id", "terminal_outcome",
			}
			if !reflect.DeepEqual(gotKeys, wantKeys) {
				t.Fatalf("Air doctor JSON keys = %v, want %v; body = %s", gotKeys, wantKeys, output)
			}
			for _, secret := range private {
				if secret != "" && bytes.Contains(output, []byte(secret)) {
					t.Fatalf("route doctor leaked %q: %s", secret, output)
				}
			}
		})
	}
}

func TestRouteDoctorReportsRecoveryLifecycleWhenAirConfigIsMissing(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(t *testing.T, ledgerPath string)
		wantState  string
		wantHealth string
		wantReason string
	}{
		{name: "active recovery", wantState: recovery.AirRecoveryStateAwaitingHostRoundtrip, wantHealth: recovery.AirRecoveryHealthHealthy, wantReason: recovery.AirRecoveryReasonOK},
		{
			name: "invalid recovery storage",
			mutate: func(t *testing.T, ledgerPath string) {
				if runtime.GOOS == "windows" {
					t.Skip("POSIX permission contract")
				}
				if err := os.Chmod(ledgerPath, 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantState:  "unknown",
			wantHealth: recovery.AirRecoveryHealthInvalid,
			wantReason: recovery.AirRecoveryReasonLedgerPermission,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newAirOrphanCLIHarness(t)
			caseID := previewAirOrphanCase(t, &h).CaseID
			h.app.Out.(*bytes.Buffer).Reset()
			if err := Execute(h.app, []string{
				"route", "recover-orphan", "air", "--case-id", caseID,
				"--confirm-host-idle", "--ack-unset-external-selection", "--json",
			}); err != nil {
				t.Fatal(err)
			}
			ledgerPath := filepath.Join(h.dataDir, "recovery", "air", "ledger.json")
			quarantinePath := filepath.Join(h.dataDir, "recovery", "air", "quarantine", caseID, "config.toml")
			if tt.mutate != nil {
				tt.mutate(t, ledgerPath)
			}
			if err := os.Remove(h.air); err != nil {
				t.Fatal(err)
			}
			discovered := h.app.Discovery.(reconciliationDiscovery).result
			for index := range discovered.Surfaces {
				if discovered.Surfaces[index].ID == discovery.SurfaceAirCodex {
					discovered.Surfaces[index].Present = false
				}
			}
			h.app.Discovery = reconciliationDiscovery{result: discovered}
			before := captureDoctorFiles(t, []string{ledgerPath, quarantinePath})

			h.app.Out.(*bytes.Buffer).Reset()
			err := Execute(h.app, []string{"route", "doctor", "--json"})
			if err == nil {
				t.Fatal("active or invalid recovery storage passed route doctor when Air config was missing")
			}
			output := h.app.Out.(*bytes.Buffer).Bytes()
			var report routeDoctorReport
			if err := json.Unmarshal(output, &report); err != nil {
				t.Fatalf("route doctor did not emit machine-readable JSON: %v\n%s", err, output)
			}
			air := routeDoctorSurface(report, discovery.SurfaceAirCodex)
			if air.RecoveryState != tt.wantState || air.RecoveryHealth != tt.wantHealth || air.RecoveryReasonCode != tt.wantReason {
				t.Fatalf("Air recovery lifecycle = %#v, want state=%q health=%q reason=%q", air, tt.wantState, tt.wantHealth, tt.wantReason)
			}
			if after := captureDoctorFiles(t, []string{ledgerPath, quarantinePath}); !reflect.DeepEqual(before, after) {
				t.Fatalf("route doctor changed recovery storage:\nbefore=%#v\nafter=%#v", before, after)
			}
			for _, forbidden := range []string{h.air, h.dataDir, caseID, "ledger.json", "config.toml"} {
				if bytes.Contains(output, []byte(forbidden)) || strings.Contains(err.Error(), forbidden) {
					t.Fatalf("route doctor leaked %q: err=%v output=%s", forbidden, err, output)
				}
			}
		})
	}
}

func TestRouteDoctorClassifiesUnreadableCodexConfigsWithoutPathErrors(t *testing.T) {
	tests := []struct {
		name      string
		surfaceID string
		path      func(airRouteHarness) string
	}{
		{name: "standalone Codex", surfaceID: discovery.SurfaceCodexCLIStandalone, path: func(h airRouteHarness) string { return h.standalone }},
		{name: "Air", surfaceID: discovery.SurfaceAirCodex, path: func(h airRouteHarness) string { return h.air }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newAirRouteHarness(t)
			path := tt.path(h)
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
			h.app.Out.(*bytes.Buffer).Reset()

			err := Execute(h.app, []string{"route", "doctor", "--json"})
			if err == nil {
				t.Fatal("unreadable Codex config passed route doctor")
			}
			var pathErr *os.PathError
			if errors.As(err, &pathErr) {
				t.Fatalf("route doctor propagated os.PathError: %v", err)
			}
			output := h.app.Out.(*bytes.Buffer).Bytes()
			var report routeDoctorReport
			if err := json.Unmarshal(output, &report); err != nil {
				t.Fatalf("route doctor did not emit machine-readable JSON: %v\n%s", err, output)
			}
			status := routeDoctorSurface(report, tt.surfaceID)
			if status.State != "inspection-unreadable" || report.OK {
				t.Fatalf("unreadable surface status = %#v, report=%#v", status, report)
			}
			for _, forbidden := range []string{path, "read Codex config", "capture Air config"} {
				if bytes.Contains(output, []byte(forbidden)) || strings.Contains(err.Error(), forbidden) {
					t.Fatalf("route doctor leaked %q: err=%v output=%s", forbidden, err, output)
				}
			}
		})
	}
}

type doctorFileSnapshot struct {
	Exists bool
	Data   string
	Mode   os.FileMode
}

func captureDoctorFiles(t *testing.T, paths []string) map[string]doctorFileSnapshot {
	t.Helper()
	result := make(map[string]doctorFileSnapshot, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			result[path] = doctorFileSnapshot{}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		result[path] = doctorFileSnapshot{Exists: true, Data: string(data), Mode: info.Mode().Perm()}
	}
	return result
}
