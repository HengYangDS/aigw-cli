package attestation_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/attestation"
)

func TestInspectAirRuntimeRejectsInvalidConfiguredRoutes(t *testing.T) {
	for _, endpoint := range []string{
		"postgres://invalid-scheme",
		":invalid-url",
		"https:///no-host",
		"https://:443/no-hostname",
	} {
		t.Run(endpoint, func(t *testing.T) {
			_, err := attestation.InspectAirRuntime(attestation.AirOptions{AIGWEndpoint: endpoint})
			if err == nil || err.Error() != "configured Codex route is unavailable" {
				t.Fatalf("InspectAirRuntime() error = %v", err)
			}
		})
	}
}

func TestInspectAirRuntimeReturnsUnknownWhenCurrentLogExceedsBudget(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	logDir := t.TempDir()
	line := airForwardingLine(now.Add(-time.Minute), "6000:WS:selected", "trace", "https://gateway.test/v1/responses")
	writeAirLog(t, logDir, "air.log", line)

	report, err := attestation.InspectAirRuntime(attestation.AirOptions{
		LogDir:             logDir,
		AIGWEndpoint:       "https://gateway.test/v1",
		ConfigurationState: "external-host-mirror",
		Now:                now,
		ScanByteLimit:      int64(len(line) - 1),
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.RuntimeAuthority != "unknown" || report.RequestCount != 0 || report.State != "host-mirror-runtime-unattested" {
		t.Fatalf("budget-limited report = %#v", report)
	}
}

func TestInspectAirRuntimeUsesEarliestCurrentRecordAsRotationCutoff(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	logDir := t.TempDir()
	writeAirLog(t, logDir, "air.log",
		airForwardingLine(now.Add(-time.Hour), "6100:WS:selected", "latest", "https://gateway.test/v1/responses"),
		airForwardingLine(now.Add(-2*time.Hour), "6100:WS:selected", "earliest", "https://gateway.test/v1/responses"),
	)

	report, err := attestation.InspectAirRuntime(attestation.AirOptions{
		LogDir: logDir, AIGWEndpoint: "https://gateway.test/v1", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.RequestCount != 2 || report.WindowStart != now.Add(-2*time.Hour).Format(time.RFC3339) || report.WindowEnd != now.Add(-time.Hour).Format(time.RFC3339) {
		t.Fatalf("current-generation window = %#v", report)
	}
}

func TestInspectAirRuntimeRejectsNonRegularRotationWithoutLeaks(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	logDir := t.TempDir()
	writeAirLog(t, logDir, "air.log", airForwardingLine(now.Add(-time.Hour), "6200:WS:selected", "trace", "https://gateway.test/v1/responses"))
	if err := os.Mkdir(filepath.Join(logDir, "air1.log"), 0o700); err != nil {
		t.Fatal(err)
	}

	_, err := attestation.InspectAirRuntime(attestation.AirOptions{
		LogDir: logDir, AIGWEndpoint: "https://gateway.test/v1", Now: now,
	})
	if err == nil || err.Error() != "Air router log is unavailable" {
		t.Fatalf("InspectAirRuntime() error = %v", err)
	}
	if strings.Contains(err.Error(), logDir) || strings.Contains(err.Error(), "air1.log") {
		t.Fatalf("error leaked rotation path: %v", err)
	}
}

func TestInspectAirRuntimeIgnoresForwardingRecordWithInvalidTarget(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	logDir := t.TempDir()
	writeAirLog(t, logDir, "air.log", airForwardingLine(now.Add(-time.Hour), "6300:WS:selected", "trace", "ftp://gateway.test/responses"))

	report, err := attestation.InspectAirRuntime(attestation.AirOptions{
		LogDir: logDir, AIGWEndpoint: "https://gateway.test/v1", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.RequestCount != 0 || report.RuntimeAuthority != "unknown" || len(report.HostHashes) != 0 {
		t.Fatalf("invalid-target report = %#v", report)
	}
}

func TestInspectAirRuntimeTreatsConfiguredHostRootAsAIGW(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	logDir := t.TempDir()
	writeAirLog(t, logDir, "air.log", airForwardingLine(now.Add(-time.Hour), "6400:WS:selected", "trace", "https://gateway.test/responses"))

	report, err := attestation.InspectAirRuntime(attestation.AirOptions{
		LogDir: logDir, AIGWEndpoint: "https://gateway.test", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.RequestCount != 1 || report.AIGWRequestCount != 1 || report.RuntimeAuthority != "aigw" {
		t.Fatalf("root-route report = %#v", report)
	}
}
