package self

import (
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/program"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// statusSchemaVersion is the version of `aigw self status`'s JSON document,
// which continues from `aigw installation`'s 1.
const statusSchemaVersion = 2

// statusDocument heads the program's installation report with schema_version
// and ok, as every command's document starts, shadowing the report's own
// schema version.
type statusDocument struct {
	SchemaVersion int  `json:"schema_version"`
	OK            bool `json:"ok"`
	program.Installation
}

// newStatusCommand constructs `aigw self status`, which reports the running
// portable program and its retained predecessor without changing anything.
func newStatusCommand(runtime invocation.Context) *cobra.Command {
	var jsonMode bool
	command := &cobra.Command{
		Use: "status", Short: "Show this program's files and the retained predecessor",
		Long:    "Observe the program's paths, byte counts and SHA-256 without changing files or reading the configuration. This does not verify release trust, rollback readiness or client connectivity.",
		Example: "aigw self status\naigw self status --json",
		Args:    cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			result, err := program.InspectInstallation(runtime.Executable, runtime.Version)
			if err != nil {
				return err
			}
			if jsonMode {
				return terminal.WriteJSON(runtime.Out, statusDocument{SchemaVersion: statusSchemaVersion, OK: true, Installation: result})
			}
			render := invocation.Renderer(runtime)
			render.ProductTitle("Program status")
			rows := []terminal.Field{
				{Label: "Version", Value: result.Version},
				{Label: "Command", Value: result.CommandPath},
				{Label: "Program file", Value: result.Payload.Path},
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
	command.Flags().BoolVar(&jsonMode, "json", false, "Write the program's files as JSON")
	return command
}
