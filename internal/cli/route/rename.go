package route

import (
	"aigw-cli/internal/cli/invocation"

	"github.com/spf13/cobra"
)

// newRenameCommand constructs the Route identity-migration command.
func newRenameCommand(runtime invocation.Context) *cobra.Command {
	var dryRun, jsonMode bool
	cmd := &cobra.Command{Use: "rename [old] [new]", Short: "Rename a route", Args: invocation.RenameArguments(runtime, "route")}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		oldID, newID, err := invocation.ResolveRenameIDs(runtime, "route", args)
		if err != nil {
			return err
		}
		plan, err := runtime.Renamer().RenameRoute(cmd.Context(), oldID, newID, dryRun)
		if err != nil {
			return err
		}
		return invocation.WriteRenamePlan(runtime, plan, dryRun, jsonMode)
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the rename plan without writing configuration")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the rename plan or result as JSON")
	return cmd
}
