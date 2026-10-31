// Package installation implements `aigw installation`, which reports the
// running portable program and its retained predecessor.
package installation

import (
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/program"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// installationDocument heads the program's versioned installation document
// with schema_version and ok, as every command's document starts. Its
// schema_version is the program's, and it shadows the embedded one.
type installationDocument struct {
	SchemaVersion int  `json:"schema_version"`
	OK            bool `json:"ok"`
	program.Installation
}

// NewCommand constructs read-only observation of the running installation.
func NewCommand(runtime invocation.Context) *cobra.Command {
	var jsonMode bool
	command := &cobra.Command{
		Use: "installation", Short: "Inspect the current portable program and retained predecessor",
		Long: "Observe program paths, byte counts and SHA-256 without changing files or reading Account configuration. This does not verify release trust, rollback readiness or client connectivity.",
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			result, err := program.InspectInstallation(runtime.Executable, runtime.Version)
			if err != nil {
				return err
			}
			if jsonMode {
				return terminal.WriteJSON(runtime.Out, installationDocument{SchemaVersion: result.SchemaVersion, OK: true, Installation: result})
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
