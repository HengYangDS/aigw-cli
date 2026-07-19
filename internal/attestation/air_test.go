package attestation_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/attestation"
)

const airTimestampLayout = "20060102 15:04:05.000"

func airForwardingLine(at time.Time, generation, traceID, target string) string {
	return "[" + at.Format(airTimestampLayout) + " INFO  " + generation + " f.a.a.c.w.CodexOpenAiApiRouterServer][workspace/AgentId(id=codex)] Forwarding CallTraceId(id=" + traceID + ")/POST:/responses to " + target
}

func writeAirLog(t *testing.T, dir, name string, lines ...string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	body := strings.Join(lines, "\n")
	if len(lines) > 0 {
		body += "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestInspectAirRuntimeReportsFreshAIGWOnlyGenerationWithoutLeaks(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	logDir := t.TempDir()
	selected := "4102:WS:selectedGeneration"
	current := airForwardingLine(now.Add(-time.Hour), selected, "trace-current-secret", "https://gateway.test:443/v1/responses")
	writeAirLog(t, logDir, "air.log",
		airForwardingLine(now.Add(-2*time.Hour), "3999:WS:olderGeneration", "trace-old", "https://api.jetbrains.ai/responses"),
		current,
		current,
	)
	writeAirLog(t, logDir, "air1.log",
		airForwardingLine(now.Add(-3*time.Hour), selected, "trace-rotation-secret", "https://gateway.test/v1/responses"),
		airForwardingLine(now.Add(-4*time.Hour), "3888:WS:foreignGeneration", "trace-foreign", "https://other.test/responses"),
	)

	report, err := attestation.InspectAirRuntime(attestation.AirOptions{
		LogDir:             logDir,
		AIGWEndpoint:       "https://gateway.test/v1",
		ConfigurationState: "external-host-mirror",
		Now:                now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.SurfaceID != "jetbrains-air-codex" || report.ConfigurationState != "external-host-mirror" {
		t.Fatalf("identity = %#v", report)
	}
	if report.State != "host-mirror-runtime-unattested" || report.RuntimeAuthority != "aigw" {
		t.Fatalf("attestation = %#v", report)
	}
	if report.RequestCount != 2 || report.AIGWRequestCount != 2 || report.JetBrainsRequestCount != 0 || report.OtherRequestCount != 0 {
		t.Fatalf("counts = %#v", report)
	}
	if report.WindowStart != now.Add(-3*time.Hour).Format(time.RFC3339) || report.WindowEnd != now.Add(-time.Hour).Format(time.RFC3339) {
		t.Fatalf("window = %q..%q", report.WindowStart, report.WindowEnd)
	}
	if report.ObservedProcessStart != "" {
		t.Fatalf("unsupported process start was invented: %q", report.ObservedProcessStart)
	}
	if len(report.HostHashes) != 1 || len(report.HostHashes[0]) != 64 {
		t.Fatalf("host hashes = %#v", report.HostHashes)
	}
	if report.HostAuthentication != "not-probed" || report.BillingEvidence != "unknown" || report.EvidenceSource != "air-log" || !report.ReadOnly {
		t.Fatalf("bounded claims = %#v", report)
	}

	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{logDir, "gateway.test", "https://", "4102", "selectedGeneration", "trace-current-secret", "trace-rotation-secret", "/v1"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("report leaked %q: %s", forbidden, encoded)
		}
	}
}

func TestInspectAirRuntimeSelectsLatestGenerationFromCurrentLogOnly(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	logDir := t.TempDir()
	writeAirLog(t, logDir, "air.log",
		airForwardingLine(now.Add(-90*time.Minute), "4102:WS:old", "old", "https://gateway.test/v1/responses"),
		airForwardingLine(now.Add(-30*time.Minute), "4103:WS:new", "new", "https://api.jetbrains.ai/responses"),
	)
	writeAirLog(t, logDir, "air1.log",
		airForwardingLine(now.Add(-2*time.Hour), "4102:WS:old", "old-rotation", "https://gateway.test/v1/responses"),
		airForwardingLine(now.Add(-time.Hour), "4103:WS:new", "new-rotation", "https://edge.jetbrains.ai/responses"),
		airForwardingLine(now.Add(-10*time.Minute), "4103:WS:new", "newer-than-current-must-be-ignored", "https://gateway.test/v1/responses"),
	)

	report, err := attestation.InspectAirRuntime(attestation.AirOptions{
		LogDir:             logDir,
		AIGWEndpoint:       "https://gateway.test/v1",
		ConfigurationState: "external-host-mirror",
		Now:                now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.RuntimeAuthority != "jetbrains-ai" || report.RequestCount != 2 || report.JetBrainsRequestCount != 2 || report.AIGWRequestCount != 0 {
		t.Fatalf("selected generation report = %#v", report)
	}
	if report.State != "host-mirror-runtime-attested" {
		t.Fatalf("state = %q", report.State)
	}
	if report.WindowStart != now.Add(-time.Hour).Format(time.RFC3339) || report.WindowEnd != now.Add(-30*time.Minute).Format(time.RFC3339) {
		t.Fatalf("window = %q..%q", report.WindowStart, report.WindowEnd)
	}
}

func TestInspectAirRuntimeClassifiesExactRoutesAndLookalikes(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		endpoint  string
		targets   []string
		authority string
		aigw      int
		jetbrains int
		other     int
		wantState string
	}{
		{
			name:      "AIGW default and explicit ports with path boundary",
			endpoint:  "https://gateway.test:443/v1",
			targets:   []string{"https://gateway.test/v1/responses", "https://gateway.test:443/v1/chat"},
			authority: "aigw", aigw: 2, wantState: "host-mirror-runtime-unattested",
		},
		{
			name:      "JetBrains exact and subdomain",
			endpoint:  "https://gateway.test/v1",
			targets:   []string{"https://jetbrains.ai/responses", "https://edge.jetbrains.ai/responses"},
			authority: "jetbrains-ai", jetbrains: 2, wantState: "host-mirror-runtime-attested",
		},
		{
			name:      "lookalikes and AIGW path confusion are other",
			endpoint:  "https://gateway.test/v1",
			targets:   []string{"https://jetbrains.ai.evil.test/responses", "https://notjetbrains.ai/responses", "http://jetbrains.ai/responses", "https://gateway.test/v10/responses"},
			authority: "unknown", other: 4, wantState: "host-mirror-runtime-unattested",
		},
		{
			name:      "recognized routes plus other are mixed",
			endpoint:  "http://127.0.0.1:8791/v1",
			targets:   []string{"http://127.0.0.1:8791/v1/responses", "https://api.jetbrains.ai/responses", "https://other.test/responses"},
			authority: "mixed", aigw: 1, jetbrains: 1, other: 1, wantState: "host-mirror-runtime-unattested",
		},
		{
			name:      "configured JetBrains host remains AIGW by priority",
			endpoint:  "https://api.jetbrains.ai/v1",
			targets:   []string{"https://api.jetbrains.ai/v1/responses"},
			authority: "aigw", aigw: 1, wantState: "host-mirror-runtime-unattested",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logDir := t.TempDir()
			lines := make([]string, 0, len(tt.targets))
			for index, target := range tt.targets {
				lines = append(lines, airForwardingLine(now.Add(time.Duration(index-len(tt.targets))*time.Minute), "5001:WS:selected", "trace", target))
			}
			writeAirLog(t, logDir, "air.log", lines...)
			report, err := attestation.InspectAirRuntime(attestation.AirOptions{
				LogDir: logDir, AIGWEndpoint: tt.endpoint, ConfigurationState: "external-host-mirror", Now: now,
			})
			if err != nil {
				t.Fatal(err)
			}
			if report.RuntimeAuthority != tt.authority || report.AIGWRequestCount != tt.aigw || report.JetBrainsRequestCount != tt.jetbrains || report.OtherRequestCount != tt.other || report.State != tt.wantState {
				t.Fatalf("report = %#v", report)
			}
		})
	}
}

func TestInspectAirRuntimeIgnoresHeadersBodiesPromptsOtherLoggersAndMalformedLines(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	logDir := t.TempDir()
	valid := airForwardingLine(now.Add(-time.Hour), "5100:WS:selected", "accepted", "https://gateway.test/v1/responses")
	writeAirLog(t, logDir, "air.log",
		"[20260719 10:00:00.000 INFO  5100:WS:selected f.a.a.c.w.CodexOpenAiApiRouterServer][workspace/AgentId(id=codex)] Headers: Authorization=Bearer secret-token https://evil.test",
		"[20260719 10:01:00.000 INFO  5100:WS:selected f.a.a.c.w.CodexOpenAiApiRouterServer][workspace/AgentId(id=codex)] Request body: prompt-body https://evil.test",
		"[20260719 10:02:00.000 INFO  5100:WS:selected f.a.a.c.w.OtherLogger][workspace/AgentId(id=codex)] Forwarding CallTraceId(id=wrong)/POST:/responses to https://evil.test",
		"Forwarding CallTraceId(id=unanchored)/POST:/responses to https://evil.test",
		"[20260719 10:03:00.000 INFO  5100:WS:selected f.a.a.c.w.CodexOpenAiApiRouterServer][workspace/AgentId(id=codex)] Forwarding prompt-body/POST:/responses to https://evil.test",
		valid,
	)

	report, err := attestation.InspectAirRuntime(attestation.AirOptions{
		LogDir: logDir, AIGWEndpoint: "https://gateway.test/v1", ConfigurationState: "external-host-mirror", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.RequestCount != 1 || report.AIGWRequestCount != 1 || report.RuntimeAuthority != "aigw" {
		t.Fatalf("report = %#v", report)
	}
}

func TestInspectAirRuntimeSkipsOversizedLines(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	logDir := t.TempDir()
	oversized := airForwardingLine(now.Add(-2*time.Hour), "5200:WS:selected", strings.Repeat("x", 2*1024*1024), "https://evil.test/responses")
	valid := airForwardingLine(now.Add(-time.Hour), "5200:WS:selected", "accepted", "https://gateway.test/v1/responses")
	writeAirLog(t, logDir, "air.log", oversized, valid)

	report, err := attestation.InspectAirRuntime(attestation.AirOptions{
		LogDir: logDir, AIGWEndpoint: "https://gateway.test/v1", ConfigurationState: "external-host-mirror", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.RequestCount != 1 || report.AIGWRequestCount != 1 {
		t.Fatalf("oversized line affected report: %#v", report)
	}
}

func TestInspectAirRuntimeKeepsCurrentEvidenceWhenRotationExceedsScanBudget(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	logDir := t.TempDir()
	current := airForwardingLine(now.Add(-time.Minute), "5250:WS:selected", "accepted", "https://api.jetbrains.ai/responses")
	writeAirLog(t, logDir, "air.log", current)
	writeAirLog(t, logDir, "air1.log", strings.Repeat("rotation-noise", 128))

	report, err := attestation.InspectAirRuntime(attestation.AirOptions{
		LogDir: logDir, AIGWEndpoint: "https://gateway.test/v1", ConfigurationState: "external-host-mirror", Now: now,
		ScanByteLimit: int64(len(current) + 32),
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.State != "host-mirror-runtime-attested" || report.RuntimeAuthority != "jetbrains-ai" || report.RequestCount != 1 {
		t.Fatalf("report = %#v", report)
	}
}

func TestInspectAirRuntimeReturnsUnknownForMissingStaleFutureOrNoisyEvidence(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		lines []string
	}{
		{name: "missing current log"},
		{name: "stale", lines: []string{airForwardingLine(now.Add(-24*time.Hour-time.Second), "5300:WS:selected", "stale", "https://gateway.test/v1/responses")}},
		{name: "future", lines: []string{airForwardingLine(now.Add(time.Second), "5300:WS:selected", "future", "https://gateway.test/v1/responses")}},
		{name: "only ignored noise", lines: []string{"Headers: https://gateway.test/v1/responses", "Request body: secret prompt"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logDir := t.TempDir()
			if tt.lines != nil {
				writeAirLog(t, logDir, "air.log", tt.lines...)
			}
			report, err := attestation.InspectAirRuntime(attestation.AirOptions{
				LogDir: logDir, AIGWEndpoint: "https://gateway.test/v1", ConfigurationState: "external-host-mirror", Now: now,
			})
			if err != nil {
				t.Fatal(err)
			}
			if report.RuntimeAuthority != "unknown" || report.State != "host-mirror-runtime-unattested" || report.RequestCount != 0 || report.WindowStart != "" || report.WindowEnd != "" || len(report.HostHashes) != 0 {
				t.Fatalf("unknown report = %#v", report)
			}
		})
	}
}

func TestInspectAirRuntimeUsesNotAHostMirrorState(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	logDir := t.TempDir()
	writeAirLog(t, logDir, "air.log", airForwardingLine(now.Add(-time.Hour), "5400:WS:selected", "trace", "https://gateway.test/v1/responses"))
	report, err := attestation.InspectAirRuntime(attestation.AirOptions{
		LogDir: logDir, AIGWEndpoint: "https://gateway.test/v1", ConfigurationState: "orphaned-aigw-marker", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.State != "not-a-host-mirror" || report.RuntimeAuthority != "aigw" {
		t.Fatalf("report = %#v", report)
	}
}

func TestInspectAirRuntimeReadErrorDoesNotExposePath(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	logDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(logDir, "air.log"), 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := attestation.InspectAirRuntime(attestation.AirOptions{
		LogDir: logDir, AIGWEndpoint: "https://gateway.test/v1", ConfigurationState: "external-host-mirror", Now: now,
	})
	if err == nil {
		t.Fatal("unreadable Air log unexpectedly succeeded")
	}
	if strings.Contains(err.Error(), logDir) || strings.Contains(err.Error(), "air.log") || strings.Contains(err.Error(), "gateway.test") {
		t.Fatalf("error leaked private input: %v", err)
	}
}
