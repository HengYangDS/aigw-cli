package presentation

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestFixedLabelPreservesLongLabel(t *testing.T) {
	renderer := New(nil, false)
	label := "this label is longer than the fixed column"
	if got, want := renderer.fixedLabel(renderer.styles.rowKey, label, rowKeyWidth), label+" "; got != want {
		t.Fatalf("fixedLabel() = %q, want %q", got, want)
	}
}

func TestRowsShareOneLayoutForLongUnicodeAndMultilineFields(t *testing.T) {
	fields := []Field{
		{Label: "short", Value: "First description"},
		{Label: "configuration-with-a-long-label", Value: "Second description"},
		{Label: "配置 e\u0301", Value: "Third description"},
	}
	for _, color := range []bool{false, true} {
		var out bytes.Buffer
		renderer := NewWithWidth(&out, color, 100)
		renderer.Rows(fields...)
		column := -1
		for index, line := range strings.Split(strings.TrimSuffix(ansi.Strip(out.String()), "\n"), "\n") {
			prefix, _, found := strings.Cut(line, fields[index].Value)
			if !found || (column >= 0 && DisplayWidth(prefix) != column) {
				t.Fatalf("group columns differ: %q", out.String())
			}
			column = DisplayWidth(prefix)
		}
		out.Reset()
		fields[1].Value = "Second description\nContinuation"
		renderer.Rows(fields...)
		for _, field := range fields {
			if !strings.Contains(ansi.Strip(out.String()), "  "+field.Label+"\n    ") {
				t.Fatalf("multiline row did not select one compact group layout: %q", out.String())
			}
		}
		if !strings.Contains(ansi.Strip(out.String()), "    Continuation\n") {
			t.Fatal("multiline continuation lost its indentation")
		}
		fields[1].Value = "Second description"
	}
}

func TestHumanTextPreservesMultilineIndentation(t *testing.T) {
	for _, width := range []int{0, 80} {
		var out bytes.Buffer
		renderer := NewWithWidth(&out, false, width)
		renderer.Text("first\nsecond")
		renderer.Command("first\nsecond")
		if got, want := out.String(), "  first\n  second\n  first\n  second\n"; got != want {
			t.Fatalf("width=%d lost multiline indentation: %q", width, got)
		}
	}
}

func TestCommandsPreserveCopyableBytesAtEveryWidth(t *testing.T) {
	const command = "aigw profile add team --label 'two  spaces' --model very-long-model-identifier\naigw profile add tabbed --label 'tab\tvalue'  "
	for _, width := range []int{0, 8, 24, 120} {
		for _, color := range []bool{false, true} {
			var out bytes.Buffer
			renderer := NewWithWidth(&out, color, width)
			renderer.Command(command)
			want := "  " + strings.ReplaceAll(command, "\n", "\n  ") + "\n"
			if got := ansi.Strip(out.String()); got != want {
				t.Fatalf("width=%d color=%t rewrote executable input:\n%q\nwant:\n%q", width, color, got, want)
			}
		}
	}
}
