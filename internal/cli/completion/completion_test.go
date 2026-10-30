package completion

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

func TestCompletionSupportsDocumentedShells(t *testing.T) {
	out := new(bytes.Buffer)
	root := &cobra.Command{Use: "aigw", SilenceErrors: true, SilenceUsage: true}
	root.AddCommand(NewCommand())
	root.SetOut(out)
	root.SetErr(out)
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		out.Reset()
		root.SetArgs([]string{"completion", shell})
		if err := root.Execute(); err != nil {
			t.Fatalf("%s completion: %v", shell, err)
		}
		if out.Len() == 0 {
			t.Fatalf("%s completion produced no output", shell)
		}
	}
	root.SetArgs([]string{"completion", "unsupported"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected unsupported shell error")
	}
}
