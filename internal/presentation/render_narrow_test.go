package presentation_test

import (
	"bytes"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/presentation"
)

func TestRendererUsesCompactLayoutForNarrowRows(t *testing.T) {
	var out bytes.Buffer
	r := presentation.NewWithWidth(&out, false, 24)
	r.Row("Current profile", "GPT-5.6 Terra")
	r.Status(presentation.OK, "Precise balance", "Disabled until connected")
	r.Detail("Run aigw account connect team-gateway")
	r.Command("aigw config import team-profiles.toml")

	got := out.String()
	for _, want := range []string{
		"  Current profile\n",
		"    GPT-5.6 Terra\n",
		"  ✓ Precise balance\n",
		"    Disabled until\n",
		"    connected\n",
		"    Run aigw account\n",
		"    connect team-gateway\n",
		"  aigw config import\n",
		"  team-profiles.toml\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("narrow output missing %q:\n%s", want, got)
		}
	}
	for _, forbidden := range []string{"Current profileGPT", "connec\nted", "team-profiles.tom\nl"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("narrow output contains %q:\n%s", forbidden, got)
		}
	}
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("plain narrow output contains ANSI: %q", got)
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

	for _, line := range strings.Split(strings.TrimRight(out.String(), "\n"), "\n") {
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

	for _, line := range strings.Split(strings.TrimRight(out.String(), "\n"), "\n") {
		if got := presentation.DisplayWidth(line); got > width {
			t.Fatalf("line width = %d, want <= %d: %q", got, width, line)
		}
	}
}
