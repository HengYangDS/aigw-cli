// Package status implements `aigw status`, which shows each client's model,
// named provider/model, and its local readiness without reading Token values.
package status

import (
	"aigw-cli/internal/cli/invocation"

	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// schemaVersion is the version of `aigw status`'s JSON document.
const schemaVersion = 3

// NewCommand constructs the read-only configuration and client readiness command.
func NewCommand(runtime invocation.Context) *cobra.Command {
	var jsonMode bool
	cmd := &cobra.Command{Use: "status", Short: "Show each client's model and local readiness", Args: cobra.NoArgs,
		Example: "aigw status\naigw status --json"}
	cmd.RunE = func(_ *cobra.Command, _ []string) error { return RunStatus(runtime, jsonMode) }
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write machine-readable JSON")
	return cmd
}

// RunStatus observes each client's model and readiness without reading
// secret values or changing state.
func RunStatus(runtime invocation.Context, jsonMode bool) error {
	cfg, err := runtime.Config.Load()
	if err != nil {
		return err
	}
	result := collectStatus(runtime.Dependencies, cfg)
	if jsonMode {
		result.Clients = clientStates(cfg, invocation.DocumentClients(result.observed))
		result.NextAction, result.NextDetail = terminal.NextSteps(result.NextAction)
		return terminal.WriteDocument(runtime.Out, schemaVersion, true, result)
	}
	renderStatus(runtime, cfg, result)
	return nil
}
