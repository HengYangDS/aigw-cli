package attestation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func FuzzParseAirRecord(f *testing.F) {
	f.Add([]byte("[20260719 12:00:00.000 INFO  4102:WS:selected f.a.a.c.w.CodexOpenAiApiRouterServer][workspace/AgentId(id=codex)] Forwarding CallTraceId(id=trace)/POST:/responses to https://api.jetbrains.ai/responses"))
	f.Add([]byte("Headers: Authorization=Bearer secret-token https://secret.example/private"))
	f.Add([]byte(strings.Repeat("x", maxAirLogLineBytes+1)))
	f.Add([]byte{0, 0xff, '\r', '\n'})

	f.Fuzz(func(t *testing.T, line []byte) {
		record, ok := parseAirRecord(line, time.UTC)
		if !ok {
			return
		}
		if record.generation == "" || record.target.scheme == "" || record.target.hostname == "" {
			t.Fatal("accepted record is incomplete")
		}
		if record.target.scheme != "http" && record.target.scheme != "https" {
			t.Fatal("accepted record uses an unsupported scheme")
		}
	})
}

func FuzzParseRouteIdentity(f *testing.F) {
	for _, seed := range []string{
		"https://api.jetbrains.ai/responses",
		"http://127.0.0.1:8791/v1/responses",
		"http://0//",
		"https://user:secret@example.test/private#fragment",
		"file:///tmp/session.jsonl",
		"https://example.test:99999/private",
		"\x00https://secret.example/private",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		route, ok := parseRouteIdentity(raw)
		if !ok {
			return
		}
		if route.scheme != "http" && route.scheme != "https" {
			t.Fatal("accepted route uses an unsupported scheme")
		}
		port, err := strconv.Atoi(route.port)
		if err != nil || port < 1 || port > 65535 {
			t.Fatal("accepted route uses an invalid normalized port")
		}
		if route.hostname == "" || !strings.HasPrefix(route.path, "/") {
			t.Fatal("accepted route identity is incomplete")
		}
	})
}

func FuzzInspectAirRuntimeIsBoundedAndRedacted(f *testing.F) {
	f.Add([]byte("[20260719 11:00:00.000 INFO  4102:WS:selected f.a.a.c.w.CodexOpenAiApiRouterServer][workspace/AgentId(id=codex)] Forwarding CallTraceId(id=session-secret)/POST:/responses to https://api.jetbrains.ai/responses\n"))
	f.Add([]byte("Headers: Authorization=Bearer top-secret https://secret.example/private\n"))
	f.Add([]byte(strings.Repeat("x", maxAirLogLineBytes+1) + "\n"))
	f.Add([]byte{0, 0xff, '\r', '\n'})

	f.Fuzz(func(t *testing.T, body []byte) {
		logDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(logDir, "air.log"), body, 0o600); err != nil {
			t.Fatal("fuzz fixture write failed")
		}
		report, err := InspectAirRuntime(AirOptions{
			LogDir:             logDir,
			AIGWEndpoint:       "https://secret.example/private",
			ConfigurationState: "external-host-mirror",
			Now:                time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC),
		})
		forbidden := []string{logDir, "air.log", "https://", "secret.example", "top-secret", "session-secret", "Authorization", "Bearer"}
		if err != nil {
			for _, value := range forbidden {
				if strings.Contains(err.Error(), value) {
					t.Fatal("attestation error disclosed private input")
				}
			}
			return
		}
		encoded, err := json.Marshal(report)
		if err != nil {
			t.Fatal("attestation encoding failed")
		}
		for _, value := range forbidden {
			if strings.Contains(string(encoded), value) {
				t.Fatal("attestation output disclosed private input")
			}
		}
	})
}
