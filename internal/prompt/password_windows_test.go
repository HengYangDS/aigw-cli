//go:build windows

package prompt

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestReadHiddenTokenRejectsNonConsoleStdinOnWindows(t *testing.T) {
	input, output, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	original := os.Stdin
	os.Stdin = input
	t.Cleanup(func() {
		os.Stdin = original
		if err := input.Close(); err != nil {
			t.Error(err)
		}
	})

	var prompt bytes.Buffer
	_, err = ReadHiddenToken(&prompt, false)
	if err == nil || !strings.Contains(err.Error(), "interactive terminal") {
		t.Fatalf("readToken error = %v, want interactive-terminal diagnostic", err)
	}
	if got := prompt.String(); got != "" {
		t.Fatalf("non-console input unexpectedly emitted a prompt: %q", got)
	}
}
