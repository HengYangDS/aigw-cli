package presentation_test

import (
	"bytes"
	"errors"
	"testing"

	"aigw-cli/internal/presentation"
)

func TestRendererStopsAfterFirstLineWriteFailure(t *testing.T) {
	want := errors.New("line output is unavailable")
	renderer := presentation.New(failingWriter{err: want}, false)
	renderer.Section("First")
	renderer.Section("Second")
	renderer.Row("Label", "Value")

	if !errors.Is(renderer.Err(), want) {
		t.Fatalf("Renderer.Err() = %v, want %v", renderer.Err(), want)
	}
}

func TestRendererWritesNonCompactRowsAtConfiguredWidth(t *testing.T) {
	var out bytes.Buffer
	renderer := presentation.NewWithWidth(&out, false, 30)
	renderer.Row("Label", "Value")
	renderer.Status(presentation.Info, "State", "Value")
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
	if want := "    \n"; detail.String() != want {
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
