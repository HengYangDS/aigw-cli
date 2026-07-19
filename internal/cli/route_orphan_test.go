package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/discovery"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

type airOrphanCLIHarness struct {
	airRouteHarness
	dataDir string
	orphan  []byte
}

func newAirOrphanCLIHarness(t *testing.T) airOrphanCLIHarness {
	t.Helper()
	h := newAirRouteHarness(t)
	projected, err := os.ReadFile(h.standalone)
	if err != nil {
		t.Fatal(err)
	}
	orphan := bytes.Replace(projected, []byte("https://gateway.test/v1"), []byte("https://orphan.test/v1"), 1)
	orphan = append(orphan, []byte("\n[jetbrains]\nhost_only = true\n")...)
	if err := os.WriteFile(h.air, orphan, 0o640); err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(filepath.Dir(h.app.Config.Path()), "aigw-data")
	h.app.DataDir = dataDir
	return airOrphanCLIHarness{airRouteHarness: h, dataDir: dataDir, orphan: orphan}
}

type airOrphanPreview struct {
	State  string `json:"state"`
	Action string `json:"action"`
	CaseID string `json:"case_id"`
}

func assertAirRouteJSONKeys(t *testing.T, data []byte) {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, data)
	}
	got := make([]string, 0, len(body))
	for key := range body {
		got = append(got, key)
	}
	sort.Strings(got)
	want := []string{"action", "case_id", "recovery_generation", "state", "surface_id"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("JSON keys = %v, want %v; body = %s", got, want, data)
	}
}

func previewAirOrphanCase(t *testing.T, h *airOrphanCLIHarness) airOrphanPreview {
	t.Helper()
	h.app.Out.(*bytes.Buffer).Reset()
	if err := Execute(h.app, []string{"route", "recover-orphan", "air", "--dry-run", "--json"}); err != nil {
		t.Fatal(err)
	}
	var preview airOrphanPreview
	if err := json.Unmarshal(h.app.Out.(*bytes.Buffer).Bytes(), &preview); err != nil {
		t.Fatalf("preview is not JSON: %v\n%s", err, h.app.Out.(*bytes.Buffer).String())
	}
	return preview
}

func TestAirRecoverOrphanPreviewIsStableReadOnlyAndPathFree(t *testing.T) {
	h := newAirOrphanCLIHarness(t)
	before, _ := os.ReadFile(h.air)
	first := previewAirOrphanCase(t, &h)
	second := previewAirOrphanCase(t, &h)
	assertAirRouteJSONKeys(t, h.app.Out.(*bytes.Buffer).Bytes())
	if first != second || first.CaseID == "" || first.State != "orphaned-exact-full-selection" || first.Action != "quarantine-and-clean" {
		t.Fatalf("previews = %#v %#v", first, second)
	}
	after, _ := os.ReadFile(h.air)
	if !bytes.Equal(before, after) {
		t.Fatal("preview changed Air")
	}
	if _, err := os.Stat(filepath.Join(h.dataDir, "recovery")); !os.IsNotExist(err) {
		t.Fatalf("preview created recovery storage: %v", err)
	}
	if len(h.runner.plans) != 0 {
		t.Fatalf("preview executed a client: %#v", h.runner.plans)
	}
	output := h.app.Out.(*bytes.Buffer).String()
	for _, forbidden := range []string{h.air, h.standalone, h.dataDir, "orphan.test", "gateway.test", "gpt-test", "model_provider"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("preview leaked %q: %s", forbidden, output)
		}
	}
}

func TestAirRecoverOrphanRequiresCaseIdleAndExplicitAcknowledgement(t *testing.T) {
	h := newAirOrphanCLIHarness(t)
	caseID := previewAirOrphanCase(t, &h).CaseID
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "case", args: []string{"route", "recover-orphan", "air", "--confirm-host-idle", "--ack-unset-external-selection"}, want: "--case-id"},
		{name: "idle", args: []string{"route", "recover-orphan", "air", "--case-id", caseID, "--ack-unset-external-selection"}, want: "--confirm-host-idle"},
		{name: "ack", args: []string{"route", "recover-orphan", "air", "--case-id", caseID, "--confirm-host-idle"}, want: "--ack-unset-external-selection"},
		{name: "wrong case", args: []string{"route", "recover-orphan", "air", "--case-id", "air-000001-000000000000", "--confirm-host-idle", "--ack-unset-external-selection"}, want: "case ID"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h.app.Out.(*bytes.Buffer).Reset()
			if err := Execute(h.app, tt.args); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
	if err := Execute(h.app, []string{"route", "recover-orphan", "air", "--force"}); err == nil {
		t.Fatal("unsupported --force was accepted")
	}
	if _, err := os.Stat(filepath.Join(h.dataDir, "recovery")); !os.IsNotExist(err) {
		t.Fatalf("rejected recovery wrote storage: %v", err)
	}
}

func TestAirRecoverOrphanCleansExactProjectionWithoutClientOrCredentials(t *testing.T) {
	h := newAirOrphanCLIHarness(t)
	caseID := previewAirOrphanCase(t, &h).CaseID
	h.app.Out.(*bytes.Buffer).Reset()
	if err := Execute(h.app, []string{
		"route", "recover-orphan", "air", "--case-id", caseID,
		"--confirm-host-idle", "--ack-unset-external-selection", "--json",
	}); err != nil {
		t.Fatal(err)
	}
	cleaned, err := os.ReadFile(h.air)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"managed by AIGW", "[model_providers.aigw]", "orphan.test", "gpt-test"} {
		if bytes.Contains(cleaned, []byte(forbidden)) {
			t.Fatalf("cleaned Air retains %q", forbidden)
		}
	}
	for _, want := range []string{"[jetbrains]", "host_only = true"} {
		if !bytes.Contains(cleaned, []byte(want)) {
			t.Fatalf("cleaned Air lost %q", want)
		}
	}
	if len(h.runner.plans) != 0 {
		t.Fatalf("recovery executed a client: %#v", h.runner.plans)
	}
	output := h.app.Out.(*bytes.Buffer).String()
	for _, forbidden := range []string{h.air, h.standalone, h.dataDir, "orphan.test", "gateway.test", "gpt-test"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("recovery leaked %q: %s", forbidden, output)
		}
	}
	if !strings.Contains(output, `"state": "awaiting-host-roundtrip"`) {
		t.Fatalf("recovery output = %s", output)
	}
}

func TestAirRecoverOrphanRejectsMirrorPartialSidecarAndListedTarget(t *testing.T) {
	t.Run("mirror", func(t *testing.T) {
		h := newAirOrphanCLIHarness(t)
		projected, _ := os.ReadFile(h.standalone)
		if err := os.WriteFile(h.air, projected, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := Execute(h.app, []string{"route", "recover-orphan", "air", "--dry-run", "--json"}); err == nil {
			t.Fatal("host mirror admitted")
		}
	})
	t.Run("partial", func(t *testing.T) {
		h := newAirOrphanCLIHarness(t)
		if err := os.WriteFile(h.air, []byte("# >>> AIGW managed provider >>>\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := Execute(h.app, []string{"route", "recover-orphan", "air", "--dry-run", "--json"}); err == nil {
			t.Fatal("partial residue admitted")
		}
	})
	t.Run("sidecar", func(t *testing.T) {
		h := newAirOrphanCLIHarness(t)
		if err := os.WriteFile(h.air+".aigw-state.json", []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := Execute(h.app, []string{"route", "recover-orphan", "air", "--dry-run", "--json"}); err == nil {
			t.Fatal("sidecar-backed Air projection admitted")
		}
	})
	t.Run("listed target", func(t *testing.T) {
		h := newAirOrphanCLIHarness(t)
		cfg, err := h.app.Config.Load()
		if err != nil {
			t.Fatal(err)
		}
		adapter := cfg.Adapters[domain.ClientCodex]
		adapter.Targets = append(adapter.Targets, h.air)
		cfg.Adapters[domain.ClientCodex] = adapter
		if err := h.app.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		if err := Execute(h.app, []string{"route", "recover-orphan", "air", "--dry-run", "--json"}); err == nil || !strings.Contains(err.Error(), "target") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestAirSettleWaitsForRoundtripThenSettlesWithoutWritingAir(t *testing.T) {
	h := newAirOrphanCLIHarness(t)
	caseID := previewAirOrphanCase(t, &h).CaseID
	if err := Execute(h.app, []string{
		"route", "recover-orphan", "air", "--case-id", caseID,
		"--confirm-host-idle", "--ack-unset-external-selection",
	}); err != nil {
		t.Fatal(err)
	}
	h.app.Out.(*bytes.Buffer).Reset()
	if err := Execute(h.app, []string{"route", "settle", "air", "--case-id", caseID, "--dry-run", "--json"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.app.Out.(*bytes.Buffer).String(), `"action": "wait"`) {
		t.Fatalf("settle preview = %s", h.app.Out.(*bytes.Buffer).String())
	}
	if err := Execute(h.app, []string{"route", "settle", "air", "--case-id", caseID}); err == nil {
		t.Fatal("unchanged cleaned postimage settled")
	}
	if err := os.WriteFile(h.air, []byte("model_provider = \"jetbrains\"\nhost_roundtrip = true\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(h.air)
	h.app.Out.(*bytes.Buffer).Reset()
	if err := Execute(h.app, []string{"route", "settle", "air", "--case-id", caseID, "--json"}); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(h.air)
	if !bytes.Equal(before, after) {
		t.Fatal("settle changed Air")
	}
	if !strings.Contains(h.app.Out.(*bytes.Buffer).String(), `"state": "settled"`) {
		t.Fatalf("settle output = %s", h.app.Out.(*bytes.Buffer).String())
	}
}

func TestAirSettleAcceptsRecreatedReferenceMirrorAndQuarantinesPartialResidue(t *testing.T) {
	t.Run("reference mirror", func(t *testing.T) {
		h := newAirOrphanCLIHarness(t)
		caseID := previewAirOrphanCase(t, &h).CaseID
		if err := Execute(h.app, []string{"route", "recover-orphan", "air", "--case-id", caseID, "--confirm-host-idle", "--ack-unset-external-selection"}); err != nil {
			t.Fatal(err)
		}
		projection, _ := os.ReadFile(h.standalone)
		if err := os.WriteFile(h.air, projection, 0o640); err != nil {
			t.Fatal(err)
		}
		if err := Execute(h.app, []string{"route", "settle", "air", "--case-id", caseID, "--json"}); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("partial", func(t *testing.T) {
		h := newAirOrphanCLIHarness(t)
		caseID := previewAirOrphanCase(t, &h).CaseID
		if err := Execute(h.app, []string{"route", "recover-orphan", "air", "--case-id", caseID, "--confirm-host-idle", "--ack-unset-external-selection"}); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(h.air, []byte("# >>> AIGW managed provider >>>\n"), 0o640); err != nil {
			t.Fatal(err)
		}
		err := Execute(h.app, []string{"route", "settle", "air", "--case-id", caseID, "--json"})
		if err == nil {
			t.Fatal("partial residue settlement unexpectedly succeeded")
		}
		if !strings.Contains(h.app.Out.(*bytes.Buffer).String(), `"state": "quarantined"`) {
			t.Fatalf("settle output = %s", h.app.Out.(*bytes.Buffer).String())
		}
	})
}

func TestAirSettleRejectsChangedQuarantineAndIsIdempotentAfterSettlement(t *testing.T) {
	t.Run("changed quarantine", func(t *testing.T) {
		h := newAirOrphanCLIHarness(t)
		caseID := previewAirOrphanCase(t, &h).CaseID
		if err := Execute(h.app, []string{"route", "recover-orphan", "air", "--case-id", caseID, "--confirm-host-idle", "--ack-unset-external-selection"}); err != nil {
			t.Fatal(err)
		}
		quarantine := filepath.Join(h.dataDir, "recovery", "air", "quarantine", caseID, "config.toml")
		if err := os.WriteFile(quarantine, []byte("tampered"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(h.air, []byte("model_provider = \"jetbrains\"\nhost_roundtrip = true\n"), 0o640); err != nil {
			t.Fatal(err)
		}
		before, _ := os.ReadFile(h.air)
		if err := Execute(h.app, []string{"route", "settle", "air", "--case-id", caseID, "--json"}); err == nil {
			t.Fatal("changed quarantine unexpectedly settled")
		}
		after, _ := os.ReadFile(h.air)
		if !bytes.Equal(before, after) {
			t.Fatal("rejected settlement changed Air")
		}
	})

	t.Run("already settled", func(t *testing.T) {
		h := newAirOrphanCLIHarness(t)
		caseID := previewAirOrphanCase(t, &h).CaseID
		if err := Execute(h.app, []string{"route", "recover-orphan", "air", "--case-id", caseID, "--confirm-host-idle", "--ack-unset-external-selection"}); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(h.air, []byte("model_provider = \"jetbrains\"\nhost_roundtrip = true\n"), 0o640); err != nil {
			t.Fatal(err)
		}
		for attempt := 0; attempt < 2; attempt++ {
			h.app.Out.(*bytes.Buffer).Reset()
			if err := Execute(h.app, []string{"route", "settle", "air", "--case-id", caseID, "--json"}); err != nil {
				t.Fatalf("settle attempt %d: %v", attempt+1, err)
			}
			assertAirRouteJSONKeys(t, h.app.Out.(*bytes.Buffer).Bytes())
		}
	})
}

func TestAirOrphanMutationClassification(t *testing.T) {
	for _, args := range [][]string{
		{"route", "recover-orphan", "air", "--dry-run"},
		{"route", "settle", "air", "--case-id", "air-000001-000000000000", "--dry-run"},
	} {
		if mutationCommand(&App{}, args) {
			t.Fatalf("dry-run unexpectedly mutates: %#v", args)
		}
	}
	for _, args := range [][]string{
		{"route", "recover-orphan", "air"},
		{"route", "settle", "air", "--case-id", "air-000001-000000000000"},
	} {
		if !mutationCommand(&App{}, args) {
			t.Fatalf("apply unexpectedly lock-free: %#v", args)
		}
	}
	if mutationCommand(&App{}, []string{"route", "attest", "air", "--json"}) {
		t.Fatal("attest unexpectedly mutates")
	}
}

func TestRouteDoctorMapsExactAirOrphanAndRecoveryLifecycle(t *testing.T) {
	h := newAirOrphanCLIHarness(t)
	report, err := buildRouteDoctorReport(h.app)
	if err != nil {
		t.Fatal(err)
	}
	air := routeDoctorSurface(report, discovery.SurfaceAirCodex)
	if air.State != "orphaned-exact-full-selection" || report.OK {
		t.Fatalf("report = %#v", report)
	}
	out := new(bytes.Buffer)
	h.app.Out, h.app.Err = out, out
	if err := Execute(h.app, []string{"route", "doctor"}); err == nil {
		t.Fatal("exact orphan passed doctor")
	}
	if !strings.Contains(out.String(), "aigw route recover-orphan air --dry-run --json") {
		t.Fatalf("doctor guidance = %s", out.String())
	}

	caseID := previewAirOrphanCase(t, &h).CaseID
	if err := Execute(h.app, []string{"route", "recover-orphan", "air", "--case-id", caseID, "--confirm-host-idle", "--ack-unset-external-selection"}); err != nil {
		t.Fatal(err)
	}
	report, err = buildRouteDoctorReport(h.app)
	if err != nil {
		t.Fatal(err)
	}
	air = routeDoctorSurface(report, discovery.SurfaceAirCodex)
	if air.RecoveryState != "awaiting-host-roundtrip" {
		t.Fatalf("Air status = %#v", air)
	}
	if err := os.WriteFile(h.air, h.orphan, 0o640); err != nil {
		t.Fatal(err)
	}
	report, err = buildRouteDoctorReport(h.app)
	if err != nil {
		t.Fatal(err)
	}
	air = routeDoctorSurface(report, discovery.SurfaceAirCodex)
	if air.State != "reappeared-after-recovery" || air.RecoveryState != "awaiting-host-roundtrip" {
		t.Fatalf("Air status = %#v", air)
	}
}
