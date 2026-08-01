package renaming

import (
	"encoding/json"
	"errors"
	"fmt"

	"aigw-cli/internal/presentation"
	"github.com/spf13/cobra"
)

func NewProfileCommand(deps Dependencies) *cobra.Command {
	var dryRun, jsonMode bool
	cmd := &cobra.Command{Use: "rename [old] [new]", Short: "Rename a profile", Args: cobra.MaximumNArgs(2)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) < 2 && !deps.Interactive {
			return fmt.Errorf("profile rename requires <old> <new> in non-interactive mode")
		}
		cfg, err := deps.Config.Load()
		if err != nil {
			return err
		}
		oldID, newID, err := resolveIDs(deps, "profile", args, profileChoices(cfg))
		if err != nil {
			return err
		}
		plan, err := planProfile(cfg, oldID, newID)
		if err != nil {
			return err
		}
		if dryRun {
			return writeResult(deps, plan, jsonMode)
		}
		if err := deps.Synchronizer.Commit(cmd.Context(), cfg, plan.Config, "profile rename"); err != nil {
			return err
		}
		plan.Status, plan.Actions.Backup = "applied", "refreshed"
		return writeResult(deps, plan, jsonMode)
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the rename plan without writing configuration")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the rename plan or result as JSON")
	return cmd
}

func NewAccountCommand(deps Dependencies) *cobra.Command {
	var dryRun, jsonMode, finalize, confirmAPITokenRotation, confirmAccountProbeRotation bool
	cmd := &cobra.Command{Use: "rename [old] [new]", Short: "Rename an account and update its profile references", Args: cobra.MaximumNArgs(2)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if finalize {
			if len(args) != 2 {
				return errors.New("account rename --finalize requires explicit <old> <new> arguments")
			}
			plan, err := planFinalize(deps, args[0], args[1], FinalizeOptions{ConfirmAPITokenRotation: confirmAPITokenRotation, ConfirmAccountProbeRotation: confirmAccountProbeRotation})
			if err != nil {
				return err
			}
			if dryRun || plan.Status == "already-finalized" {
				return writeResult(deps, plan, jsonMode)
			}
			if plan.blockedReason != "" {
				return fmt.Errorf("Account finalization confirmation required: %s", plan.blockedReason)
			}
			plan, err = applyFinalize(cmd.Context(), deps, plan)
			if err != nil {
				return err
			}
			return writeResult(deps, plan, jsonMode)
		}
		if confirmAPITokenRotation || confirmAccountProbeRotation {
			return errors.New("credential rotation confirmations require --finalize")
		}
		if len(args) < 2 && !deps.Interactive {
			return errors.New("account rename requires <old> <new> in non-interactive mode")
		}
		cfg, err := deps.Config.Load()
		if err != nil {
			return err
		}
		oldID, newID, err := resolveIDs(deps, "account", args, accountChoices(cfg))
		if err != nil {
			return err
		}
		plan, err := planAccount(cfg, oldID, newID)
		if err != nil {
			return err
		}
		plan, err = planCredentialCopies(deps, plan)
		if err != nil {
			return err
		}
		if dryRun {
			return writeResult(deps, plan, jsonMode)
		}
		if plan.blockedReason != "" {
			return fmt.Errorf("Account rename is blocked: %s", plan.blockedReason)
		}
		if err := applyCredentialCopies(deps, plan); err != nil {
			return fmt.Errorf("Prepare target credential slots; source and target slots were retained: %w", err)
		}
		if err := deps.Synchronizer.Commit(cmd.Context(), cfg, plan.Config, "account rename"); err != nil {
			return fmt.Errorf("Account rename configuration commit failed; source and target credential slots were retained: %w", err)
		}
		plan.Status, plan.Actions.Backup = "applied", "refreshed"
		return writeResult(deps, plan, jsonMode)
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the rename plan without writing configuration or credentials")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the rename plan or result as JSON")
	cmd.Flags().BoolVar(&finalize, "finalize", false, "Converge the verified rollback baseline and remove source credential slots")
	cmd.Flags().BoolVar(&confirmAPITokenRotation, "confirm-api-token-rotation", false, "Confirm that a different target API token was intentionally rotated and fully verified")
	cmd.Flags().BoolVar(&confirmAccountProbeRotation, "confirm-account-probe-rotation", false, "Confirm that different target account probe credentials were intentionally rotated")
	return cmd
}

func writeResult(deps Dependencies, plan Plan, jsonMode bool) error {
	if jsonMode {
		enc := json.NewEncoder(deps.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(plan)
	}

	r := presentation.NewWithWidth(deps.Out, deps.Color, deps.Width)
	isPlan := plan.Status != "applied"
	if plan.Resource == "account" {
		if plan.Finalize && plan.Status == "finalized" {
			r.ProductTitle("Account finalization complete")
		} else if plan.Finalize && plan.Status == "already-finalized" {
			r.ProductTitle("Account already finalized")
		} else if plan.Finalize {
			r.ProductTitle("Account finalization plan")
		} else if isPlan {
			r.ProductTitle("Account rename plan")
		} else {
			r.ProductTitle("Account renamed")
		}
		r.Row("Previous account", plan.OldID)
		r.Row("New account", plan.NewID)
		r.Row("Label", plan.Account.Label)
		if len(plan.AffectedReferences) > 0 {
			r.Row("Profile references", fmt.Sprintf("%d", len(plan.AffectedReferences)))
		}
	} else {
		if isPlan {
			r.ProductTitle("Profile rename plan")
		} else {
			r.ProductTitle("Profile renamed")
		}
		r.Row("Previous profile", plan.OldID)
		r.Row("New profile", plan.NewID)
		r.Row("Account", plan.Profile.Account)
		if len(plan.AffectedReferences) > 0 {
			r.Row("Route references", fmt.Sprintf("%d", len(plan.AffectedReferences)))
		}
	}
	for _, todo := range plan.ExternalTODOs {
		r.Row("External action", todo)
	}
	switch plan.Status {
	case "blocked":
		r.Status(presentation.Warn, "Plan", "Blocked; no changes were made")
	case "planned":
		r.Success("Dry run complete; no changes were made")
	case "already-finalized":
		r.Success("The verified rollback baseline and source credential cleanup are already complete")
	case "finalized":
		r.Success("The verified rollback baseline was converged and source credential slots were removed")
	case "applied":
		if plan.Resource == "account" {
			r.Success("Configuration and target credentials are ready; source credential slots were retained for rollback")
			r.Next("aigw verify --for all")
			r.Next("aigw account rename " + plan.OldID + " " + plan.NewID + " --finalize")
		} else {
			r.Success("The account token remains in place and routes were synchronized")
		}
	}
	return nil
}
