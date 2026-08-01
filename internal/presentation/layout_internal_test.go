package presentation

import "testing"

func TestMin(t *testing.T) {
	for _, tt := range []struct {
		left  int
		right int
		want  int
	}{
		{left: 5, right: 10, want: 5},
		{left: 10, right: 5, want: 5},
	} {
		if got := min(tt.left, tt.right); got != tt.want {
			t.Fatalf("min(%d, %d) = %d, want %d", tt.left, tt.right, got, tt.want)
		}
	}
}

func TestFixedLabelPreservesLongLabel(t *testing.T) {
	renderer := New(nil, false)
	label := "this label is longer than the fixed column"
	if got, want := renderer.fixedLabel(renderer.styles.rowKey, label, rowKeyWidth), label+" "; got != want {
		t.Fatalf("fixedLabel() = %q, want %q", got, want)
	}
}
