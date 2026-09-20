package presentation_test

import (
	"bytes"
	"strings"
	"testing"

	"aigw-cli/internal/presentation"

	"github.com/charmbracelet/x/ansi"
)

func TestRendererUsesCompactLayoutForNarrowRows(t *testing.T) {
	var out bytes.Buffer
	r := presentation.NewWithWidth(&out, false, 24)
	r.Row("Current profile", "GPT-5.6 Terra")
	r.Status(presentation.OK, "Precise balance", "Disabled until connected")
	r.Detail("Run aigw account diagnostics enable team-gateway")
	r.Command("aigw config import configuration.toml")

	got := out.String()
	for _, want := range []string{
		"  Current profile\n",
		"    GPT-5.6 Terra\n",
		"  ✓ Precise balance\n",
		"    Disabled until\n",
		"    connected\n",
		"    Run aigw account\n",
		"    diagnostics enable\n",
		"    team-gateway\n",
		"  aigw config import configuration.toml\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("narrow output missing %q:\n%s", want, got)
		}
	}
	for _, forbidden := range []string{"Current profileGPT", "connec\nted", "configuration.tom\nl"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("narrow output contains %q:\n%s", forbidden, got)
		}
	}
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("plain narrow output contains ANSI: %q", got)
	}
}

func TestExplanatoryElementsBoundLongAndMultilineContent(t *testing.T) {
	for _, width := range []int{8, 24, 48} {
		for _, color := range []bool{false, true} {
			var out bytes.Buffer
			r := presentation.NewWithWidth(&out, color, width)
			label := "configuration-with-an-unusually-long-name"
			value := "https://example.test/a/very/long/unbroken/path\nSecond line"
			r.Title(label, value)
			r.Section(label)
			r.Row(label, value)
			r.Status(presentation.Warn, label, value)
			r.StatusLine(presentation.Info, label, value)
			r.Text(value)
			r.Detail(value)
			r.Success(value)
			for line := range strings.SplitSeq(out.String(), "\n") {
				if got := presentation.DisplayWidth(line); got > width {
					t.Fatalf("width=%d color=%t overflow=%d: %q", width, color, got, line)
				}
			}
			if !strings.Contains(ansi.Strip(out.String()), "Second") {
				t.Fatal("multiline content was lost")
			}
		}
	}
}

func TestRendererKeepsCompactOutputWithinTerminalWidth(t *testing.T) {
	const width = 24
	var out bytes.Buffer
	r := presentation.NewWithWidth(&out, false, width)
	r.Title("AIGW", "Action required")
	r.Section("Recommended action")
	r.StatusLine(presentation.Warn, "Precise balance", "Disabled until connected")
	r.Success("Client configuration synchronized")
	r.Problem(presentation.Problem{
		Title:    "Token quota is exhausted",
		Evidence: "HTTP 403 token quota is insufficient",
		Impact:   "Claude and Codex cannot continue requests",
		Fix:      "aigw rotate team-gateway",
	})

	for line := range strings.SplitSeq(strings.TrimRight(out.String(), "\n"), "\n") {
		if got := presentation.DisplayWidth(line); got > width {
			t.Fatalf("line width = %d, want <= %d: %q\n%s", got, width, line, out.String())
		}
	}
}

func TestRendererNeverExceedsACompactWidth(t *testing.T) {
	const width = 8
	var out bytes.Buffer
	r := presentation.NewWithWidth(&out, false, width)
	r.Text("aigw")
	r.Detail("go")

	for line := range strings.SplitSeq(strings.TrimRight(out.String(), "\n"), "\n") {
		if got := presentation.DisplayWidth(line); got > width {
			t.Fatalf("line width = %d, want <= %d: %q", got, width, line)
		}
	}
}
