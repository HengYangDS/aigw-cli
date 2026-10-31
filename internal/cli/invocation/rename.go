package invocation

import (
	"aigw-cli/internal/app/renaming"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/terminal"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// renameSchemaVersion is the version of the rename commands' JSON document.
const renameSchemaVersion = 1

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
// is set, with the next commands that the plan's status calls for.
func WriteRenamePlan(runtime Context, plan renaming.Plan, jsonMode bool) error {
	if jsonMode {
		return terminal.WriteDocument(runtime.Out, renameSchemaVersion, true, plan)
	}

	r := Renderer(runtime)
	isPlan := plan.Status != renaming.StatusApplied
	referenceLabel := "Client Binding references"
	var rows []terminal.Field
	if plan.Resource == renaming.ResourceAccount {
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
			{Label: "Label", Value: plan.Account.Label},
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
			{Label: "Account", Value: plan.Route.Account},
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
	case renaming.StatusPlanned:
		r.Success("Dry run complete; no changes were made")
	case renaming.StatusAlreadyFinalized:
		r.Success("The rollback baseline and source credential cleanup are already complete")
	case renaming.StatusFinalized:
		r.Success("The rollback baseline was converged and source credential cleanup is complete")
	case renaming.StatusApplied:
		if plan.Resource == renaming.ResourceAccount {
			r.Success("Configuration and target credentials are ready; source credential slots were retained for rollback")
			if len(plan.Config.EnabledClientIDs()) > 0 {
				r.Next("aigw verify --for all")
			}
			r.Next("aigw account rename " + plan.OldID + " " + plan.NewID + " --finalize")
		} else {
			r.Success("The account token remains in place and Client Bindings were synchronized")
		}
	}
	return nil
}

// ResolveRenameIDs returns the old and new resource IDs from args and asks
// the prompt for the ones that args does not supply.
func ResolveRenameIDs(runtime Context, resource string, args []string) (string, string, error) {
	if len(args) == 2 {
		return args[0], args[1], nil
	}
	cfg, err := runtime.Config.Load()
	if err != nil {
		return "", "", err
	}
	choices := routeChoices(cfg)
	if resource == "account" {
		choices = accountChoices(cfg)
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
			return "", "", fmt.Errorf("Select %s to rename: %w", resource, err)
		}
		oldID = selected
	}
	newID, err := runtime.Prompt.Text("New " + resource + " ID: ")
	if err != nil {
		return "", "", fmt.Errorf("Read new %s ID: %w", resource, err)
	}
	return oldID, strings.TrimSpace(newID), nil
}

func routeChoices(cfg domain.Config) []terminal.Choice {
	names := make([]string, 0, len(cfg.Routes))
	for name := range cfg.Routes {
		names = append(names, name)
	}
	sort.Strings(names)
	choices := make([]terminal.Choice, 0, len(names))
	for _, name := range names {
		route := cfg.Routes[name]
		label := cfg.RouteLabel(name)
		if purpose := strings.TrimSpace(route.Purpose); purpose != "" {
			label += " · " + purpose
		}
		choices = append(choices, terminal.Choice{Value: name, Label: label})
	}
	return choices
}

func accountChoices(cfg domain.Config) []terminal.Choice {
	names := make([]string, 0, len(cfg.Accounts))
	for name := range cfg.Accounts {
		names = append(names, name)
	}
	sort.Strings(names)
	choices := make([]terminal.Choice, 0, len(names))
	for _, name := range names {
		choices = append(choices, terminal.Choice{Value: name, Label: cfg.Accounts[name].Label})
	}
	return choices
}
