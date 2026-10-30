// Package status implements `aigw status`, which shows each client's selected
// Route and local readiness without reading Token values.
package status

import (
	"aigw-cli/internal/cli/invocation"

	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// NewCommand constructs the read-only configuration and client readiness command.
func NewCommand(runtime invocation.Context) *cobra.Command {
	var jsonMode bool
	cmd := &cobra.Command{Use: "status", Short: "Show each client's selected route and local readiness", Args: cobra.NoArgs}
	cmd.RunE = func(_ *cobra.Command, _ []string) error { return RunStatus(runtime, jsonMode) }
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write machine-readable JSON")
	return cmd
}

// RunStatus observes client bindings and projections without reading secret values or mutating state.
func RunStatus(runtime invocation.Context, jsonMode bool) error {
	cfg, err := runtime.Config.Load()
	if err != nil {
		return err
	}
	result := collectStatus(runtime.Dependencies, cfg)
	if jsonMode {
		return terminal.WriteJSON(runtime.Out, result)
	}
	renderStatus(runtime, cfg, result)
	return nil
}
