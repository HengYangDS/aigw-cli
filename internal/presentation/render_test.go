package presentation_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"aigw-cli/internal/presentation"

	"github.com/charmbracelet/x/ansi"
)

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestRendererProducesAlignedHumanReadableLayout(t *testing.T) {
	var out bytes.Buffer
	r := presentation.New(&out, false)
	r.Title("AIGW", "Health check")
	r.Section("Configuration")
	r.Rows(
		presentation.Field{Label: "Configuration file", Value: "Healthy"},
		presentation.Field{Label: "Current Account", Value: "DMXAPI"},
	)
	r.Section("Connection")
	r.Rows(
		presentation.Field{State: presentation.OK, Label: "API Token", Value: "Healthy"},
		presentation.Field{State: presentation.Warn, Label: "Precise balance", Value: "Disabled", Details: []string{"aigw account diagnostics enable"}},
	)
	r.Section("Result")
	r.Success("Everything is healthy")
	r.Next("aigw balance")
	want := "AIGW  Health check\n" +
		"────────────────────────────────────────\n\n" +
		"Configuration\n" +
		"  Configuration file   Healthy\n" +
		"  Current Account      DMXAPI\n\n" +
		"Connection\n" +
		"  ✓ API Token          Healthy\n" +
		"  ! Precise balance    Disabled\n" +
		"                       aigw account diagnostics enable\n\n" +
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

// valueColumns returns the display column at which each value or detail starts, keyed by that text.
func valueColumns(t *testing.T, output string, texts ...string) map[string]int {
	t.Helper()
	columns := map[string]int{}
	lines := strings.Split(ansi.Strip(output), "\n")
	for _, text := range texts {
		for _, line := range lines {
			if prefix, _, found := strings.Cut(line, text); found {
				columns[text] = presentation.DisplayWidth(prefix)
				break
			}
		}
		if _, found := columns[text]; !found {
			t.Fatalf("output lacks %q:\n%s", text, output)
		}
	}
	return columns
}

func TestGroupDerivesOneValueColumnFromItsLongestLabel(t *testing.T) {
	checks := []presentation.Field{
		{State: presentation.OK, Label: "Client token environment", Value: "No global client token environment variables detected"},
		{State: presentation.OK, Label: "Local configuration", Value: "Configuration is valid"},
		{State: presentation.OK, Label: "Account Token", Value: "dmxapi · available"},
		{State: presentation.Fail, Label: "Codex configuration target 1", Value: "Differs from the current binding", Details: []string{"Fix: aigw repair"}},
		{Label: "Unclassified row", Value: "Plain value"},
	}
	texts := []string{"No global client", "Configuration is valid", "dmxapi · available", "Differs from", "Fix: aigw repair", "Plain value"}
	for _, width := range []int{0, 100} {
		for _, color := range []bool{false, true} {
			var out bytes.Buffer
			presentation.NewWithWidth(&out, color, width).Rows(checks...)
			// "✗ Codex configuration target 1" occupies 30 cells after the two-cell indent, then a two-cell gap.
			for text, column := range valueColumns(t, out.String(), texts...) {
				if column != 34 {
					t.Fatalf("width=%d color=%t: %q starts at column %d, want 34:\n%s", width, color, text, column, out.String())
				}
			}
			plain := ansi.Strip(out.String())
			for _, prefix := range []string{"  ✓ Client token environment ", "  ✗ Codex configuration target 1  Differs", "  Unclassified row "} {
				if !strings.Contains(plain, prefix) {
					t.Fatalf("width=%d color=%t: output lacks %q:\n%s", width, color, prefix, plain)
				}
			}
		}
	}
}

func TestGroupKeepsTheDefaultColumnForShortLabels(t *testing.T) {
	var out bytes.Buffer
	presentation.New(&out, false).Rows(
		presentation.Field{Label: "Configuration file", Value: "VALUE"},
		presentation.Field{State: presentation.OK, Label: "API Token", Value: "VALUE"},
		presentation.Field{State: presentation.Warn, Label: "Precise balance", Value: "VALUE"},
	)
	for index, line := range strings.Split(strings.TrimRight(out.String(), "\n"), "\n") {
		prefix, _, ok := strings.Cut(line, "VALUE")
		if !ok || presentation.DisplayWidth(prefix) != 23 {
			t.Fatalf("line %d value column differs from 23: %q", index, line)
		}
	}
}

func TestValuesAndDetailsContinueUnderTheValueColumn(t *testing.T) {
	var out bytes.Buffer
	presentation.NewWithWidth(&out, false, 60).Rows(
		presentation.Field{
			State:   presentation.Info,
			Label:   "Claude",
			Value:   "Configured · Claude Code uses a native model preference; its AIGW endpoint and credential helper remain configured",
			Details: []string{"Verify the native model: aigw verify --for claude"},
		},
		presentation.Field{State: presentation.Info, Label: "Claude Desktop", Value: "Configured"},
	)
	want := "  · Claude             Configured · Claude Code uses a\n" +
		"                       native model preference; its AIGW\n" +
		"                       endpoint and credential helper remain\n" +
		"                       configured\n" +
		"                       Verify the native model: aigw verify\n" +
		"                       --for claude\n" +
		"  · Claude Desktop     Configured\n"
	if out.String() != want {
		t.Fatalf("continuation layout mismatch\n--- want ---\n%s--- got ---\n%s", want, out.String())
	}
}

func TestGroupKeepsMultilineValuesUnderTheValueColumn(t *testing.T) {
	var out bytes.Buffer
	presentation.New(&out, false).Rows(
		presentation.Field{Label: "Schema", Value: "6 → 6"},
		presentation.Field{Label: "Recommendations", Value: "claude=team-claude\ncodex=team-gpt"},
	)
	want := "  Schema               6 → 6\n" +
		"  Recommendations      claude=team-claude\n" +
		"                       codex=team-gpt\n"
	if out.String() != want {
		t.Fatalf("multiline layout mismatch\n--- want ---\n%s--- got ---\n%s", want, out.String())
	}
}

func TestGroupStacksOnlyWhenValuesLackRoom(t *testing.T) {
	fields := []presentation.Field{
		{Label: "Account", Value: "team-gateway"},
		{State: presentation.OK, Label: "Token", Value: "Validated and securely stored"},
	}
	for _, tc := range []struct {
		width int
		want  string
	}{
		{width: 30, want: "  Account\n    team-gateway\n  ✓ Token\n    Validated and securely\n    stored\n"},
		{width: 43, want: "  Account              team-gateway\n  ✓ Token              Validated and\n                       securely stored\n"},
		{width: 60, want: "  Account              team-gateway\n  ✓ Token              Validated and securely stored\n"},
	} {
		var out bytes.Buffer
		presentation.NewWithWidth(&out, false, tc.width).Rows(fields...)
		if out.String() != tc.want {
			t.Fatalf("width=%d layout mismatch\n--- want ---\n%s--- got ---\n%s", tc.width, tc.want, out.String())
		}
	}

	var fitting bytes.Buffer
	presentation.NewWithWidth(&fitting, false, 30).Rows(presentation.Field{Label: "Label", Value: "Value"})
	if want := "  Label                Value\n"; fitting.String() != want {
		t.Fatalf("a value that fits stacked: %q, want %q", fitting.String(), want)
	}
}

func TestNotesKeepOneIndentAtEveryWidth(t *testing.T) {
	note := "Local diagnostics passed; no client endpoint or model was checked"
	for _, width := range []int{0, 80, 120} {
		var out bytes.Buffer
		r := presentation.NewWithWidth(&out, false, width)
		r.Success("No problems found")
		r.Detail(note)
		if want := "  ✓ No problems found\n    " + note + "\n"; out.String() != want {
			t.Fatalf("width=%d note = %q, want %q", width, out.String(), want)
		}
	}
}

func TestSuccessContinuesUnderItsText(t *testing.T) {
	var out bytes.Buffer
	presentation.NewWithWidth(&out, false, 30).Success("Preview did not write configuration or client files")
	if want := "  ✓ Preview did not write\n    configuration or client\n    files\n"; out.String() != want {
		t.Fatalf("wrapped success = %q, want %q", out.String(), want)
	}
}

func TestWrappingKeepsHyphenatedIdentifiersWhole(t *testing.T) {
	var out bytes.Buffer
	r := presentation.NewWithWidth(&out, false, 26)
	r.Text("Clients claude, claude-desktop, hermes")
	r.Text("https://example.test/a/very/long/path")
	want := "  Clients claude,\n" +
		"  claude-desktop, hermes\n" +
		"  https://example.test/a/v\n" +
		"  ery/long/path\n"
	if out.String() != want {
		t.Fatalf("wrapped text mismatch\n--- want ---\n%s--- got ---\n%s", want, out.String())
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
	render := func(color bool) string {
		var out bytes.Buffer
		r := presentation.NewWithWidth(&out, color, 40)
		r.Rows(
			presentation.Field{State: presentation.OK, Label: "Token", Value: "Healthy"},
			presentation.Field{State: presentation.Warn, Label: "Precise balance", Value: "Disabled until a diagnostic credential is connected", Details: []string{"aigw account diagnostics enable team-gateway"}},
		)
		r.Success("Client configuration is aligned; authentication was unchanged")
		r.StatusLine(presentation.Info, "Route", "team-gateway-claude")
		r.Detail("Team Gateway · Claude · Available")
		return out.String()
	}
	plain, colored := render(false), render(true)
	if strings.Contains(plain, "\x1b[") {
		t.Fatalf("plain output contains ANSI: %q", plain)
	}
	if stripped := ansi.Strip(colored); stripped != plain {
		t.Fatalf("color changed layout:\nplain:\n%s\nstripped:\n%s", plain, stripped)
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

func TestRendererStopsAfterFirstLineWriteFailure(t *testing.T) {
	want := errors.New("line output is unavailable")
	renderer := presentation.New(failingWriter{err: want}, false)
	renderer.Section("First")
	renderer.Section("Second")
	renderer.Rows(presentation.Field{Label: "Label", Value: "Value"})

	if !errors.Is(renderer.Err(), want) {
		t.Fatalf("Renderer.Err() = %v, want %v", renderer.Err(), want)
	}
}

func TestRendererWritesNonCompactRowsAtConfiguredWidth(t *testing.T) {
	var out bytes.Buffer
	renderer := presentation.NewWithWidth(&out, false, 30)
	renderer.Rows(presentation.Field{Label: "Label", Value: "Value"})
	renderer.Rows(presentation.Field{State: presentation.Info, Label: "State", Value: "Value"})
	renderer.StatusLine(presentation.Warn, "State", "Value")
	renderer.Text("short")
	renderer.Command("aigw ok")
	renderer.Success("done")

	want := "  Label                Value\n" +
		"  · State              Value\n" +
		"  ! State  Value\n" +
		"  short\n" +
		"  aigw ok\n" +
		"  ✓ done\n"
	if out.String() != want {
		t.Fatalf("configured-width output = %q, want %q", out.String(), want)
	}
}

func TestRendererWrapsTextAtConfiguredWidth(t *testing.T) {
	var out bytes.Buffer
	presentation.NewWithWidth(&out, false, 12).Text("alpha beta gamma")
	if want := "  alpha beta\n  gamma\n"; out.String() != want {
		t.Fatalf("wrapped text = %q, want %q", out.String(), want)
	}
}

func TestRendererHandlesEmptyNarrowContent(t *testing.T) {
	var title bytes.Buffer
	presentation.NewWithWidth(&title, false, 8).Title("", "")
	if want := "\n────────\n"; title.String() != want {
		t.Fatalf("empty title = %q, want %q", title.String(), want)
	}

	var detail bytes.Buffer
	presentation.NewWithWidth(&detail, false, 3).Detail("")
	if want := "  \n"; detail.String() != want {
		t.Fatalf("empty detail = %q, want %q", detail.String(), want)
	}
}

func TestRendererOmitsEmptyProblemSections(t *testing.T) {
	var out bytes.Buffer
	presentation.New(&out, false).Problem(presentation.Problem{Title: "Token needs attention"})
	want := "AIGW  Action required\n" +
		"────────────────────────────────────────\n\n" +
		"Problem\n" +
		"  Token needs attention\n"
	if out.String() != want {
		t.Fatalf("partial problem output = %q, want %q", out.String(), want)
	}
}

func TestWriteJSONPreservesEncodingAndOutputFailures(t *testing.T) {
	var out bytes.Buffer
	err := presentation.WriteJSON(&out, make(chan string))
	if _, ok := errors.AsType[*json.UnsupportedTypeError](err); !ok || out.Len() != 0 {
		t.Fatalf("encoding failure=%v output=%q", err, out.String())
	}
	cause := errors.New("JSON output unavailable")
	if err := presentation.WriteJSON(failingWriter{err: cause}, map[string]bool{"ok": true}); !errors.Is(err, cause) {
		t.Fatalf("output failure=%v, want %v", err, cause)
	}
}
