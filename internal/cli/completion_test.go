package cli

import (
	"aigw-cli/internal/clients"
	"bytes"
	"testing"
)

func TestCompletionSupportsDocumentedShells(t *testing.T) {
	out := new(bytes.Buffer)
	app := &App{Clients: clients.Registry(), Out: out, Err: out}
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		out.Reset()
		if err := Execute(app, []string{"completion", shell}); err != nil {
			t.Fatalf("%s completion: %v", shell, err)
		}
		if out.Len() == 0 {
			t.Fatalf("%s completion produced no output", shell)
		}
	}
	if err := Execute(app, []string{"completion", "unsupported"}); err == nil {
		t.Fatal("expected unsupported shell error")
	}
}
