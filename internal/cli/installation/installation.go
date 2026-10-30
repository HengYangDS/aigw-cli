// Package installation implements `aigw installation`, which reports the
// running portable program and its retained predecessor.
package installation

import (
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/terminal"
	"aigw-cli/internal/upgrade"

	"github.com/spf13/cobra"
)

// NewCommand constructs read-only observation of the running installation.
func NewCommand(runtime invocation.Context) *cobra.Command {
	var jsonMode bool
	command := &cobra.Command{
		Use: "installation", Short: "Inspect the current portable program and retained predecessor",
		Long: "Observe program paths, byte counts and SHA-256 without changing files or reading Account configuration. This does not verify release trust, rollback readiness or client connectivity.",
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			result, err := upgrade.InspectInstallation(runtime.Executable, runtime.Version)
			if err != nil {
				return err
			}
			if jsonMode {
				return terminal.WriteJSON(runtime.Out, result)
			}
			render := invocation.Renderer(runtime)
			render.ProductTitle("Portable installation")
			rows := []terminal.Field{
				{Label: "Version", Value: result.Version},
				{Label: "Command", Value: result.CommandPath},
				{Label: "Payload", Value: result.Payload.Path},
				{Label: "SHA-256", Value: result.Payload.SHA256},
			}
			if result.Rollback == nil {
				rows = append(rows, terminal.Field{Label: "Rollback", Value: "No retained predecessor"})
			} else {
				rows = append(rows,
					terminal.Field{Label: "Rollback", Value: result.Rollback.Path},
					terminal.Field{Label: "SHA-256", Value: result.Rollback.SHA256},
				)
			}
			render.Rows(rows...)
			return render.Err()
		},
	}
	command.Flags().BoolVar(&jsonMode, "json", false, "Write machine-readable JSON")
	return command
}
