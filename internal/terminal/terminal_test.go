package terminal

import (
	"bytes"
	"errors"
	"os"
	"testing"
)

func TestColorEnabledFromEnvironment(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		term bool
		want bool
	}{
		{"non terminal", nil, false, false},
		{"NO_COLOR", map[string]string{"NO_COLOR": "1"}, true, false},
		{"terminal", nil, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ColorEnabled(tt.env, tt.term); got != tt.want {
				t.Fatalf("ColorEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestDevNullIsNotInteractiveTerminal(t *testing.T) {
	file, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Errorf("close dev null: %v", err)
		}
	})
	if Interactive(file) {
		t.Fatal("os.DevNull must not trigger the interactive wizard")
	}
}

func TestPresentationWidthUsesOnlyPositiveColumnsOverride(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  map[string]string
		want int
	}{
		{name: "unset", env: map[string]string{}, want: 0},
		{name: "zero", env: map[string]string{"COLUMNS": "0"}, want: 0},
		{name: "invalid", env: map[string]string{"COLUMNS": "wide"}, want: 0},
		{name: "positive", env: map[string]string{"COLUMNS": "72"}, want: 72},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := widthFromEnvironment(tc.env); got != tc.want {
				t.Fatalf("presentation width = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestPresentationWidthUsesOverrideAndNonTerminalFallback(t *testing.T) {
	if got := PresentationWidth(&bytes.Buffer{}, map[string]string{"COLUMNS": "55"}); got != 55 {
		t.Fatalf("PresentationWidth override = %d", got)
	}
	if got := PresentationWidth(&bytes.Buffer{}, map[string]string{}); got != 0 {
		t.Fatalf("PresentationWidth buffer = %d", got)
	}
	file, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Errorf("close dev null: %v", err)
		}
	})
	if got := PresentationWidth(file, map[string]string{}); got != 0 {
		t.Fatalf("PresentationWidth non-terminal = %d", got)
	}
}

func TestPresentationWidthTerminalSizeBranches(t *testing.T) {
	file, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Errorf("close dev null: %v", err)
		}
	})
	interactive := func(*os.File) bool { return true }
	if got := presentationWidth(file, nil, interactive, func(int) (int, int, error) { return 91, 24, nil }); got != 91 {
		t.Fatalf("terminal width = %d", got)
	}
	if got := presentationWidth(file, nil, interactive, func(int) (int, int, error) { return 0, 24, nil }); got != 0 {
		t.Fatalf("non-positive width = %d", got)
	}
	if got := presentationWidth(file, nil, interactive, func(int) (int, int, error) { return 0, 0, errors.New("size failed") }); got != 0 {
		t.Fatalf("failed width = %d", got)
	}
}
