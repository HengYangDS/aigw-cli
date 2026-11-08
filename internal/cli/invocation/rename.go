package invocation

import (
	"aigw-cli/internal/app/renaming"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/terminal"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

// renameSchemaVersion is the version of the rename commands' JSON document.
const renameSchemaVersion = 2

// renameDocument is the rename commands' JSON document: the plan or its
// result, whose status and actions are snake_case. A preview's plan says
// dry_run.
type renameDocument struct {
	DryRun bool `json:"dry_run,omitempty"`
	renaming.Plan
}

// errRenameBlocked fails a blocked rename plan, which changed nothing and
// whose output already says why.
var errRenameBlocked = terminal.Presented(errors.New("the rename plan is blocked; no changes were made"))

// RenameArguments accepts at most an old and a new valid resource ID. Fewer
// than two require an interactive prompt that can select the rest, which a
// JSON caller never gets.
func RenameArguments(runtime Context, resource string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := cobra.MaximumNArgs(2)(cmd, args); err != nil {
			return err
		}
		for _, id := range args {
			if !domain.ValidIdentifier(id) {
				return fmt.Errorf("Invalid %s ID %q; run `%s --help`", resource, id, cmd.CommandPath())
			}
		}
		if len(args) == 2 {
			return nil
		}
		if jsonMode, _ := cmd.Flags().GetBool("json"); jsonMode {
			return fmt.Errorf("%s rename with --json requires <old> <new>; run `%s --help`", resource, cmd.CommandPath())
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

// WriteRenamePlan writes a rename plan or its result, as JSON when jsonMode
// is set, with the next commands that the plan's status calls for; dryRun
// marks a preview's plan. A blocked plan reports itself and fails.
func WriteRenamePlan(runtime Context, plan renaming.Plan, dryRun, jsonMode bool) error {
	blocked := plan.Status == renaming.StatusBlocked
	if jsonMode {
		if err := terminal.WriteDocument(runtime.Out, renameSchemaVersion, !blocked, renameDocument{DryRun: dryRun, Plan: plan}); err != nil || !blocked {
			return err
		}
		return errRenameBlocked
	}

	r := Renderer(runtime)
	isPlan := plan.Status != renaming.StatusApplied
	referenceLabel := "Client Binding references"
	var rows []terminal.Field
	if plan.Resource == renaming.ResourceProvider {
		referenceLabel = "Route references"
		switch {
		case plan.Finalize && plan.Status == renaming.StatusFinalized:
			r.ProductTitle("Account finalization complete")
		case plan.Finalize && plan.Status == renaming.StatusAlreadyFinalized:
			r.ProductTitle("Account already finalized")
		case plan.Finalize:
			r.ProductTitle("Account finalization plan")
		case isPlan:
			r.ProductTitle("Account rename plan")
		default:
			r.ProductTitle("Account renamed")
		}
		rows = []terminal.Field{
			{Label: "Previous account", Value: plan.OldID},
			{Label: "New account", Value: plan.NewID},
			{Label: "Label", Value: plan.Provider.Label},
		}
	} else {
		if isPlan {
			r.ProductTitle("Route rename plan")
		} else {
			r.ProductTitle("Route renamed")
		}
		rows = []terminal.Field{
			{Label: "Previous route", Value: plan.OldID},
			{Label: "New route", Value: plan.NewID},
			{Label: "Account", Value: plan.Model.Provider},
		}
	}
	if len(plan.AffectedReferences) > 0 {
		rows = append(rows, terminal.Field{Label: referenceLabel, Value: fmt.Sprintf("%d", len(plan.AffectedReferences))})
	}
	for _, todo := range plan.ExternalTODOs {
		rows = append(rows, terminal.Field{Label: "External action", Value: todo})
	}
	if plan.Status == renaming.StatusBlocked {
		rows = append(rows, terminal.Field{State: terminal.Warn, Label: "Plan", Value: "Blocked; no changes were made"})
	}
	r.Rows(rows...)
	switch plan.Status {
	case renaming.StatusBlocked:
		// The blocked plan is the group's final row.
		return errRenameBlocked
	case renaming.StatusPlanned:
		r.Success("Dry run complete; no changes were made")
	case renaming.StatusAlreadyFinalized:
		r.Success("The rollback baseline and source credential cleanup are already complete")
	case renaming.StatusFinalized:
		r.Success("The rollback baseline was converged and source credential cleanup is complete")
	case renaming.StatusApplied:
		if plan.Resource == renaming.ResourceProvider {
			r.Success("Configuration and target credentials are ready; source credential slots were retained for rollback")
			if len(plan.Config.EnabledClientIDs()) > 0 {
				r.Next("aigw check --depth client")
			}
			r.Next("aigw account rename " + plan.OldID + " " + plan.NewID + " --finalize")
		} else {
			r.Success("The account token remains in place and Client Bindings were synchronized")
		}
	}
	return nil
}

// ResolveRenameIDs returns the old and new resource IDs from args and asks
// the prompt for the ones that args does not supply. A prompt that ends
// without an answer, before anything changed, names cmd again as its fix.
func ResolveRenameIDs(cmd *cobra.Command, runtime Context, resource string, args []string) (string, string, error) {
	oldID, newID, err := resolveRenameIDs(runtime, resource, args)
	return oldID, newID, CancelledPrompt(cmd, args, err)
}

func resolveRenameIDs(runtime Context, resource string, args []string) (string, string, error) {
	if len(args) == 2 {
		return args[0], args[1], nil
	}
	cfg, err := runtime.Config.Load()
	if err != nil {
		return "", "", err
	}
	choices := modelChoices(cfg)
	if resource == "account" {
		choices = providerChoices(cfg)
	}
	var oldID string
	if len(args) == 1 {
		oldID = args[0]
	} else {
		if len(choices) == 0 {
			return "", "", fmt.Errorf("No %ss are configured", resource)
		}
		selected, err := runtime.Prompt.Select("Select the "+resource+" to rename: ", choices)
		if err != nil {
			return "", "", Prompted("The "+resource+" selection", fmt.Errorf("Select %s to rename: %w", resource, err))
		}
		oldID = selected
	}
	newID, err := runtime.Prompt.Text("New " + resource + " ID: ")
	if err != nil {
		return "", "", Prompted("The new "+resource+" ID prompt", fmt.Errorf("Read new %s ID: %w", resource, err))
	}
	return oldID, strings.TrimSpace(newID), nil
}

func modelChoices(cfg domain.Config) []terminal.Choice {
	choices := make([]terminal.Choice, 0, len(cfg.Models))
	for _, name := range slices.Sorted(maps.Keys(cfg.Models)) {
		route := cfg.Models[name]
		label := cfg.ModelLabel(name)
		if purpose := strings.TrimSpace(route.Purpose); purpose != "" {
			label += " · " + purpose
		}
		choices = append(choices, terminal.Choice{Value: name, Label: label})
	}
	return choices
}

func providerChoices(cfg domain.Config) []terminal.Choice {
	choices := make([]terminal.Choice, 0, len(cfg.Providers))
	for _, name := range slices.Sorted(maps.Keys(cfg.Providers)) {
		choices = append(choices, terminal.Choice{Value: name, Label: cfg.Providers[name].Label})
	}
	return choices
}
