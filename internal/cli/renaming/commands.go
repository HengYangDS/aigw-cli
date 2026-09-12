// Package renaming projects identity-migration workflows onto the CLI.
package renaming

import (
	"fmt"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/renaming"

	"github.com/spf13/cobra"
)

// NewProfileCommand constructs the Profile identity-migration command.
func NewProfileCommand(runtime invocation.Context) *cobra.Command {
	var dryRun, jsonMode bool
	cmd := &cobra.Command{Use: "rename [old] [new]", Short: "Rename a profile", Args: renameArguments(runtime, "profile")}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		oldID, newID, err := resolveIDs(runtime, "profile", args)
		if err != nil {
			return err
		}
		plan, err := service(runtime).RenameProfile(cmd.Context(), oldID, newID, dryRun)
		if err != nil {
			return err
		}
		return writeResult(runtime, plan, jsonMode)
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the rename plan without writing configuration")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the rename plan or result as JSON")
	return cmd
}

// NewAccountCommand constructs the Account migration and verified-finalization command.
func NewAccountCommand(runtime invocation.Context) *cobra.Command {
	var dryRun, jsonMode, finalize bool
	var options renaming.FinalizeOptions
	cmd := &cobra.Command{Use: "rename [old] [new]", Short: "Rename an account and update its profile references"}
	cmd.Args = cobra.MatchAll(func(cmd *cobra.Command, args []string) error {
		if finalize && len(args) != 2 {
			return fmt.Errorf("account rename --finalize requires explicit <old> <new> arguments; run `%s --help`", cmd.CommandPath())
		}
		if !finalize && (options.ConfirmAPITokenRotation || options.ConfirmAccountProbeRotation) {
			return fmt.Errorf("credential rotation confirmations require --finalize; run `%s --help`", cmd.CommandPath())
		}
		return nil
	}, renameArguments(runtime, "account"))
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		var plan renaming.Plan
		var err error
		if finalize {
			plan, err = service(runtime).FinalizeAccount(cmd.Context(), args[0], args[1], dryRun, options)
		} else {
			oldID, newID, resolveErr := resolveIDs(runtime, "account", args)
			if resolveErr != nil {
				return resolveErr
			}
			plan, err = service(runtime).RenameAccount(cmd.Context(), oldID, newID, dryRun)
		}
		if err != nil {
			return err
		}
		return writeResult(runtime, plan, jsonMode)
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the rename plan without writing configuration or credentials")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the rename plan or result as JSON")
	cmd.Flags().BoolVar(&finalize, "finalize", false, "Converge the verified rollback baseline and remove source credential slots")
	cmd.Flags().BoolVar(&options.ConfirmAPITokenRotation, "confirm-api-token-rotation", false, "Confirm that a different target API token was intentionally rotated and fully verified")
	cmd.Flags().BoolVar(&options.ConfirmAccountProbeRotation, "confirm-account-probe-rotation", false, "Confirm that different target account probe credentials were intentionally rotated")
	return cmd
}

func renameArguments(runtime invocation.Context, resource string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := cobra.MaximumNArgs(2)(cmd, args); err != nil {
			return err
		}
		for _, id := range args {
			if !configuration.ValidIdentifier(id) {
				return fmt.Errorf("Invalid %s ID %q; run `%s --help`", resource, id, cmd.CommandPath())
			}
		}
		if len(args) == 2 {
			return nil
		}
		if !runtime.Interactive {
			return fmt.Errorf("%s rename requires <old> <new> in non-interactive mode; run `%s --help`", resource, cmd.CommandPath())
		}
		if runtime.Prompt == nil {
			return fmt.Errorf("%s rename requires an interactive prompt or explicit <old> <new> arguments; run `%s --help`", resource, cmd.CommandPath())
		}
		return nil
	}
}

func service(runtime invocation.Context) renaming.Service {
	return renaming.Service{
		Config: runtime.Config, Secrets: runtime.Secrets, Accounts: runtime.Accounts,
		HTTP: runtime.HTTP, Synchronizer: invocation.Synchronizer(runtime),
	}
}
