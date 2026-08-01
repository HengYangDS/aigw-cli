package presentation_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"aigw-cli/internal/presentation"
)

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestRendererProducesAlignedHumanReadableLayout(t *testing.T) {
	var out bytes.Buffer
	r := presentation.New(&out, false)
	r.Title("AIGW", "Health check")
	r.Section("Configuration")
	r.Row("Configuration file", "Healthy")
	r.Row("Current service", "DMXAPI")
	r.Section("Connection")
	r.Status(presentation.OK, "API Token", "Healthy")
	r.Status(presentation.Warn, "Precise balance", "Disabled")
	r.Detail("aigw account connect")
	r.Section("Result")
	r.Success("Everything is healthy")
	r.Next("aigw balance")
	want := "AIGW  Health check\n" +
		"────────────────────────────────────────\n\n" +
		"Configuration\n" +
		"  Configuration file   Healthy\n" +
		"  Current service      DMXAPI\n\n" +
		"Connection\n" +
		"  ✓ API Token          Healthy\n" +
		"  ! Precise balance    Disabled\n" +
		"                       aigw account connect\n\n" +
		"Result\n" +
		"  ✓ Everything is healthy\n\n" +
		"Next\n" +
		"  aigw balance\n"
	if out.String() != want {
		t.Fatalf("layout mismatch\n--- want ---\n%s--- got ---\n%s", want, out.String())
	}
}

func TestRendererRecordsOutputWriteFailure(t *testing.T) {
	want := errors.New("output is unavailable")
	r := presentation.New(failingWriter{err: want}, false)
	r.Title("AIGW", "Health check")

	if !errors.Is(r.Err(), want) {
		t.Fatalf("Renderer.Err() = %v, want %v", r.Err(), want)
	}
}

func TestRowsAndStatusesShareValueColumnAndRetainSeparator(t *testing.T) {
	var out bytes.Buffer
	r := presentation.New(&out, false)
	r.Row("Configuration file", "VALUE")
	r.Row("Current service", "VALUE")
	r.Status(presentation.OK, "Configuration file", "VALUE")
	r.Status(presentation.OK, "API Token", "VALUE")
	r.Status(presentation.Warn, "Precise balance", "VALUE")
	for index, line := range strings.Split(strings.TrimRight(out.String(), "\n"), "\n") {
		prefix, _, ok := strings.Cut(line, "VALUE")
		if !ok {
			t.Fatalf("line lacks value: %q", line)
		}
		if got := presentation.DisplayWidth(prefix); got != 23 {
			t.Fatalf("line %d value column = %d, want 23: %q", index, got, line)
		}
	}
	if !strings.Contains(out.String(), "✓ Configuration file VALUE\n") {
		t.Fatalf("status output joined a full-width label to its value: %q", out.String())
	}
}

func TestStatusKeepsLongLabelsOnOneLine(t *testing.T) {
	var out bytes.Buffer
	r := presentation.New(&out, false)
	r.Status(presentation.OK, "environment:client-token", "Healthy")
	got := out.String()
	if strings.Count(got, "\n") != 1 || strings.Contains(got, "environment:client\n-token") {
		t.Fatalf("long status label wrapped: %q", got)
	}
}

func TestDisplayWidthTreatsWideUnicodeAsTwoColumnsAndANSICodesAsZero(t *testing.T) {
	wide := string([]rune{0xFF21, 0xFF22, 0xFF23, 0xFF24})
	if got := presentation.DisplayWidth(wide); got != 8 {
		t.Fatalf("wide Unicode width = %d, want 8", got)
	}
	coloredWide := "\x1b[32m" + string([]rune{0xFF21, 0xFF22}) + "\x1b[0m"
	if got := presentation.DisplayWidth(coloredWide); got != 4 {
		t.Fatalf("colored wide Unicode width = %d, want 4", got)
	}
}

func TestRendererColorIsOptionalAndNeverAffectsSpacing(t *testing.T) {
	var plain, colored bytes.Buffer
	presentation.New(&plain, false).Status(presentation.OK, "Token", "Healthy")
	presentation.New(&colored, true).Status(presentation.OK, "Token", "Healthy")
	if strings.Contains(plain.String(), "\x1b[") {
		t.Fatalf("plain output contains ANSI: %q", plain.String())
	}
	stripped := presentation.StripANSI(colored.String())
	if stripped != plain.String() {
		t.Fatalf("color changed layout: plain=%q color=%q stripped=%q", plain.String(), colored.String(), stripped)
	}
}

func TestProblemUsesConsistentProblemEvidenceImpactFixOrder(t *testing.T) {
	var out bytes.Buffer
	r := presentation.New(&out, false)
	r.Problem(presentation.Problem{
		Title:    "Token quota is exhausted",
		Evidence: "HTTP 403 · token quota is insufficient",
		Impact:   "Claude and Codex cannot continue requests",
		Fix:      "aigw rotate",
	})
	want := `AIGW  Action required
────────────────────────────────────────

Problem
  Token quota is exhausted

Evidence
  HTTP 403 · token quota is insufficient

Impact
  Claude and Codex cannot continue requests

Recommended action
  aigw rotate
`
	if out.String() != want {
		t.Fatalf("problem layout mismatch\nwant:\n%s\ngot:\n%s", want, out.String())
	}
}
