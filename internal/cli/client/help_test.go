package client

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestTargetHelpIsOneSentence pins the help of --target: it names what the
// flag takes and the clients that read a configuration file once, and says
// that only Codex takes several, without repeating itself.
func TestTargetHelpIsOneSentence(t *testing.T) {
	runtime, _, _, _ := adapterRuntime(t, adapterConfig())
	command := NewCommand(runtime)
	want := map[string]string{
		"enable": "The client's configuration `file`, for Claude Desktop, Codex and Hermes; repeat for multiple Codex homes",
		"edit": "The client's configuration `file`, for Claude Desktop, Codex and Hermes; repeat for multiple Codex homes; " +
			"the files given replace the current ones",
	}
	for name, usage := range want {
		subcommand, _, err := command.Find([]string{name})
		if err != nil {
			t.Fatal(err)
		}
		if got := targetUsage(subcommand); got != usage {
			t.Errorf("client %s --target help = %q, want %q", name, got, usage)
		}
	}
}

func targetUsage(cmd *cobra.Command) string {
	return cmd.Flags().Lookup("target").Usage
}
