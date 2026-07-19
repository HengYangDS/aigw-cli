package cli

import (
	"os"
	"testing"
)

func TestDevNullIsNotInteractiveTerminal(t *testing.T) {
	file, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	if isTerminal(file) {
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
			if got := presentationWidthFromEnvironment(tc.env); got != tc.want {
				t.Fatalf("presentation width = %d, want %d", got, tc.want)
			}
		})
	}
}
