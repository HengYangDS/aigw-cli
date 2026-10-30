package account

import (
	"aigw-cli/internal/app/renaming"
	"aigw-cli/internal/cli/invocation"
	"fmt"

	"github.com/spf13/cobra"
)

// newRenameCommand constructs the Account migration and verified-finalization command.
func newRenameCommand(runtime invocation.Context) *cobra.Command {
	var dryRun, jsonMode, finalize bool
	var options renaming.FinalizeOptions
	cmd := &cobra.Command{Use: "rename [old] [new]", Short: "Rename an account and update its route references"}
	cmd.Args = cobra.MatchAll(func(cmd *cobra.Command, args []string) error {
		if finalize && len(args) != 2 {
			return fmt.Errorf("account rename --finalize requires explicit <old> <new> arguments; run `%s --help`", cmd.CommandPath())
		}
		if !finalize && (options.ConfirmAPITokenRotation || options.ConfirmAccountProbeRotation) {
			return fmt.Errorf("credential rotation confirmations require --finalize; run `%s --help`", cmd.CommandPath())
		}
		return nil
	}, invocation.RenameArguments(runtime, "account"))
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		var plan renaming.Plan
		var err error
		if finalize {
			plan, err = runtime.Renamer().FinalizeAccount(cmd.Context(), args[0], args[1], dryRun, options)
		} else {
			oldID, newID, resolveErr := invocation.ResolveRenameIDs(runtime, "account", args)
			if resolveErr != nil {
				return resolveErr
			}
			plan, err = runtime.Renamer().RenameAccount(cmd.Context(), oldID, newID, dryRun)
		}
		if err != nil {
			return err
		}
		return invocation.WriteRenamePlan(runtime, plan, jsonMode)
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the rename plan without writing configuration or credentials")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the rename plan or result as JSON")
	cmd.Flags().BoolVar(&finalize, "finalize", false, "Converge the verified rollback baseline and remove source credential slots")
	cmd.Flags().BoolVar(&options.ConfirmAPITokenRotation, "confirm-api-token-rotation", false, "Confirm that a different target API token was intentionally rotated and fully verified")
	cmd.Flags().BoolVar(&options.ConfirmAccountProbeRotation, "confirm-account-probe-rotation", false, "Confirm that different target account probe credentials were intentionally rotated")
	return cmd
}
