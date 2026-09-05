package presentation

import "testing"

func TestFixedLabelPreservesLongLabel(t *testing.T) {
	renderer := New(nil, false)
	label := "this label is longer than the fixed column"
	if got, want := renderer.fixedLabel(renderer.styles.rowKey, label, rowKeyWidth), label+" "; got != want {
		t.Fatalf("fixedLabel() = %q, want %q", got, want)
	}
}
