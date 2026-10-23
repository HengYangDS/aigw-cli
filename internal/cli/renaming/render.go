package renaming

import (
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/renaming"
	"fmt"
)

func writeResult(runtime invocation.Context, plan renaming.Plan, jsonMode bool) error {
	if jsonMode {
		return presentation.WriteJSON(runtime.Out, plan)
	}

	r := invocation.Renderer(runtime)
	isPlan := plan.Status != renaming.StatusApplied
	referenceLabel := "Client Binding references"
	var rows []presentation.Field
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
		rows = []presentation.Field{
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
		rows = []presentation.Field{
			{Label: "Previous route", Value: plan.OldID},
			{Label: "New route", Value: plan.NewID},
			{Label: "Account", Value: plan.Route.Account},
		}
	}
	if len(plan.AffectedReferences) > 0 {
		rows = append(rows, presentation.Field{Label: referenceLabel, Value: fmt.Sprintf("%d", len(plan.AffectedReferences))})
	}
	for _, todo := range plan.ExternalTODOs {
		rows = append(rows, presentation.Field{Label: "External action", Value: todo})
	}
	if plan.Status == renaming.StatusBlocked {
		rows = append(rows, presentation.Field{State: presentation.Warn, Label: "Plan", Value: "Blocked; no changes were made"})
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
