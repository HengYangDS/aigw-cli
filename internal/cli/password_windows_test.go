//go:build windows

package cli

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
	app := &App{Interactive: true, Out: &prompt}
	_, err = app.readToken(false, false)
	if err == nil || !strings.Contains(err.Error(), "read hidden token") {
		t.Fatalf("readToken error = %v, want non-console input diagnostic", err)
	}
	if got := prompt.String(); got != "Token: \n" {
		t.Fatalf("token prompt = %q, want %q", got, "Token: \n")
	}
}
