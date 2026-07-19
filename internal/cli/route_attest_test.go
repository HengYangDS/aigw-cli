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
	"time"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/attestation"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/config"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/discovery"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

type rejectingAttestationRunner struct{ calls int }

func (r *rejectingAttestationRunner) Run(_ context.Context, _ adapters.ProcessPlan) error {
	r.calls++
	return errors.New("route attest must not execute a client")
}

type airAttestationHarness struct {
	app        *App
	runner     *rejectingAttestationRunner
	home       string
	standalone string
	air        string
	log        string
}

func newAirAttestationHarness(t *testing.T, target string) airAttestationHarness {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home-private")
	standalone := filepath.Join(root, "standalone-private", "config.toml")
	air := filepath.Join(root, "air-private", "config.toml")
	for _, path := range []string{standalone, air} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(standalone, []byte("model_provider = \"native\"\nmodel = \"native-private\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := reconciliationConfig(standalone)
	runtime, _, err := cfg.ResolveRuntime(domain.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := adapters.SyncCodexConfig(standalone, runtime); err != nil {
		t.Fatal(err)
	}
	projection, err := os.ReadFile(standalone)
	if err != nil {
		t.Fatal(err)
	}
	projection = append(projection, []byte("\n[jetbrains]\nhost_private = true\n")...)
	if err := os.WriteFile(air, projection, 0o600); err != nil {
		t.Fatal(err)
	}
	logDir := filepath.Join(home, "Library", "Logs", "JetBrains", "Air")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(logDir, "air.log")
	observedAt := time.Now().Add(-time.Minute)
	line := "[" + observedAt.Format("20060102 15:04:05.000") + " INFO  6700:WS:privateGeneration f.a.a.c.w.CodexOpenAiApiRouterServer][private-workspace/AgentId(id=codex)] Forwarding CallTraceId(id=private-trace-id)/POST:/responses to " + target + "\n" +
		"[" + observedAt.Format("20060102 15:04:05.000") + " INFO  6700:WS:privateGeneration f.a.a.c.w.CodexOpenAiApiRouterServer][private-workspace/AgentId(id=codex)] Headers: Authorization=Bearer private-token\n" +
		"[" + observedAt.Format("20060102 15:04:05.000") + " INFO  6700:WS:privateGeneration f.a.a.c.w.CodexOpenAiApiRouterServer][private-workspace/AgentId(id=codex)] Request body: private-prompt-body\n"
	if err := os.WriteFile(logPath, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}

	output := new(bytes.Buffer)
	runner := &rejectingAttestationRunner{}
	app := &App{
		Config: config.NewStore(filepath.Join(root, "aigw-private.toml")),
		Env:    []string{"HOME=" + home},
		GOOS:   "darwin",
		Out:    output,
		Err:    output,
		Runner: runner,
		Discovery: reconciliationDiscovery{result: discovery.Result{Surfaces: []discovery.Surface{
			{ID: discovery.SurfaceCodexCLIStandalone, Authority: discovery.AuthorityAIGW, ConfigPath: standalone, Present: true, AutoManaged: true},
			{ID: discovery.SurfaceAirCodex, Authority: discovery.AuthorityJetBrainsAI, ConfigPath: air, Present: true, ManualFallbackAllowed: true},
		}}},
	}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	return airAttestationHarness{app: app, runner: runner, home: home, standalone: standalone, air: air, log: logPath}
}

func TestRouteAttestAirIsLockFreeReadOnlyAndPathFree(t *testing.T) {
	h := newAirAttestationHarness(t, "https://api.jetbrains.ai/responses")
	if mutationCommand(h.app, []string{"route", "attest", "air", "--json"}) {
		t.Fatal("route attest must not acquire a mutation lock")
	}
	before := snapshotAttestationFiles(t, h)

	if err := Execute(h.app, []string{"route", "attest", "air", "--json"}); err != nil {
		t.Fatal(err)
	}
	var report attestation.AirRuntimeAttestation
	output := h.app.Out.(*bytes.Buffer).Bytes()
	if err := json.Unmarshal(output, &report); err != nil {
		t.Fatalf("attestation output is not JSON: %v\n%s", err, output)
	}
	if report.ConfigurationState != "external-host-mirror" || report.State != "host-mirror-runtime-attested" || report.RuntimeAuthority != "jetbrains-ai" {
		t.Fatalf("report = %#v", report)
	}
	if report.RequestCount != 1 || report.JetBrainsRequestCount != 1 || !report.ReadOnly {
		t.Fatalf("report = %#v", report)
	}
	if h.runner.calls != 0 {
		t.Fatalf("runner calls = %d", h.runner.calls)
	}
	if _, err := os.Stat(h.app.Config.Path() + ".lock"); !os.IsNotExist(err) {
		t.Fatalf("route attest created a mutation lock: %v", err)
	}
	after := snapshotAttestationFiles(t, h)
	if !equalAttestationSnapshots(before, after) {
		t.Fatal("route attest changed persistent files")
	}
	for _, forbidden := range []string{
		h.home, h.standalone, h.air, h.log, "gateway.test", "api.jetbrains.ai", "https://", "6700", "privateGeneration",
		"private-workspace", "private-trace-id", "private-token", "private-prompt-body", "gpt-test", "native-private",
	} {
		if bytes.Contains(output, []byte(forbidden)) {
			t.Fatalf("attestation leaked %q: %s", forbidden, output)
		}
	}
}

func TestRouteAttestAirUnattestedJSONRemainsMachineReadable(t *testing.T) {
	tests := []struct {
		name          string
		target        string
		wantAuthority string
		wantAIGW      int
		wantOther     int
	}{
		{name: "configured AIGW route", target: "https://gateway.test/v1/responses", wantAuthority: "aigw", wantAIGW: 1},
		{name: "JetBrains lookalike", target: "https://jetbrains.ai.evil.test/responses", wantAuthority: "unknown", wantOther: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newAirAttestationHarness(t, tt.target)
			err := Execute(h.app, []string{"route", "attest", "air", "--json"})
			if err == nil {
				t.Fatal("unattested runtime unexpectedly succeeded")
			}
			var report attestation.AirRuntimeAttestation
			output := h.app.Out.(*bytes.Buffer).Bytes()
			if decodeErr := json.Unmarshal(output, &report); decodeErr != nil {
				t.Fatalf("conflict JSON is not parseable: %v\n%s", decodeErr, output)
			}
			if report.State != "host-mirror-runtime-unattested" || report.RuntimeAuthority != tt.wantAuthority || report.AIGWRequestCount != tt.wantAIGW || report.OtherRequestCount != tt.wantOther {
				t.Fatalf("report = %#v", report)
			}
			for _, forbidden := range []string{tt.target, "gateway.test", "jetbrains.ai.evil.test", h.home, h.air, "private-token", "private-prompt-body"} {
				if bytes.Contains(output, []byte(forbidden)) || strings.Contains(err.Error(), forbidden) {
					t.Fatalf("unattested result leaked %q: %v\n%s", forbidden, err, output)
				}
			}
		})
	}
}

func TestRouteAttestAirReportsNotAHostMirror(t *testing.T) {
	h := newAirAttestationHarness(t, "https://api.jetbrains.ai/responses")
	if err := os.WriteFile(h.air, []byte("model_provider = \"jetbrains\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := Execute(h.app, []string{"route", "attest", "air", "--json"})
	if err == nil {
		t.Fatal("non-mirror Air configuration unexpectedly passed attestation")
	}
	var report attestation.AirRuntimeAttestation
	if decodeErr := json.Unmarshal(h.app.Out.(*bytes.Buffer).Bytes(), &report); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if report.ConfigurationState != "external-clean" || report.State != "not-a-host-mirror" {
		t.Fatalf("report = %#v", report)
	}
}

func TestRouteAttestAirRequiresPresentDiscoveredSurfaceWithoutLeaks(t *testing.T) {
	h := newAirAttestationHarness(t, "https://api.jetbrains.ai/responses")
	discovered := h.app.Discovery.(reconciliationDiscovery).result
	for index := range discovered.Surfaces {
		if discovered.Surfaces[index].ID == discovery.SurfaceAirCodex {
			discovered.Surfaces[index].Present = false
		}
	}
	h.app.Discovery = reconciliationDiscovery{result: discovered}
	err := Execute(h.app, []string{"route", "attest", "air", "--json"})
	if err == nil {
		t.Fatal("missing Air surface unexpectedly passed attestation")
	}
	for _, forbidden := range []string{h.home, h.air, h.log, "api.jetbrains.ai", "private-token"} {
		if strings.Contains(err.Error(), forbidden) || strings.Contains(h.app.Out.(*bytes.Buffer).String(), forbidden) {
			t.Fatalf("missing-surface error leaked %q: %v\n%s", forbidden, err, h.app.Out.(*bytes.Buffer).String())
		}
	}
}

type attestationFileSnapshot map[string][]byte

func snapshotAttestationFiles(t *testing.T, h airAttestationHarness) attestationFileSnapshot {
	t.Helper()
	paths := []string{h.app.Config.Path(), h.standalone, h.standalone + ".aigw-state.json", h.air, h.log}
	result := make(attestationFileSnapshot, len(paths))
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("snapshot file: %v", err)
		}
		result[path] = body
	}
	return result
}

func equalAttestationSnapshots(left, right attestationFileSnapshot) bool {
	if len(left) != len(right) {
		return false
	}
	for path, body := range left {
		if !bytes.Equal(body, right[path]) {
			return false
		}
	}
	return true
}
