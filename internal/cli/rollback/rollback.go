// Package rollback implements `aigw rollback`, which restores configuration
// and client projections from a verified checkpoint or the previous backup.
package rollback

import (
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/terminal"
	"errors"

	"github.com/spf13/cobra"
)

// rollbackFailureImpact reports the configuration outcome that a failed
// rollback commit verified, or the conservative state when it verified none.
func rollbackFailureImpact(err error) string {
	if impact, ok := terminal.ConfigurationRecoveryImpact(err); ok {
		return impact
	}
	return "Configuration or client projections may have changed; inspect current state before retrying."
}

// NewCommand constructs the command that restores the last verified AIGW configuration checkpoint.
func NewCommand(runtime invocation.Context) *cobra.Command {
	var lastChange bool
	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "Restore configuration and client projections, not the program",
		Long: "Restore configuration and client projections from the latest fully\n" +
			"verified checkpoint, or fall back to the previous backup.\n" +
			"--last-change selects only that backup. This does not replace the\n" +
			"AIGW program. Use aigw update --rollback for the retained executable.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			current, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			restored := domain.Config{}
			source := ""
			var checkpointErr error
			if !lastChange {
				checkpoint, loadErr := runtime.Config.LoadVerifiedCheckpoint()
				checkpointErr = loadErr
				if checkpointErr == nil {
					restored = checkpoint.Config
					source = "Latest fully verified configuration"
				}
			}
			if source == "" {
				var backupErr error
				restored, backupErr = runtime.Config.LoadBackup()
				if backupErr != nil {
					cause := backupErr
					if checkpointErr != nil {
						cause = errors.Join(checkpointErr, backupErr)
					}
					return invocation.Problem(
						runtime,
						"Configuration rollback is unavailable",
						"No valid recovery source is available for the current configuration.",
						"The current configuration remains active and unchanged.",
						"aigw doctor",
						cause,
					)
				}
				source = "Previous configuration"
			}
			if err := runtime.Synchronizer().Commit(cmd.Context(), current, restored, "rollback"); err != nil {
				return invocation.Problem(
					runtime,
					"Configuration rollback did not complete",
					"One rollback step failed; the cause identifies the affected boundary.",
					rollbackFailureImpact(err),
					"aigw doctor",
					err,
				)
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Rolled back safely")
			r.Section("Restore source")
			r.Rows(terminal.Field{Label: "Configuration", Value: source})
			r.Success("Client selections and projections were restored; clients were not restarted.")
			r.Next("aigw doctor")
			return nil
		},
	}
	cmd.Flags().BoolVar(&lastChange, "last-change", false, "Restore only the immediately previous configuration backup")
	return cmd
}
