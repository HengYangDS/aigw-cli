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
	if plan.Resource == renaming.ResourceAccount {
		referenceLabel = "Profile references"
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
		r.Row("Previous account", plan.OldID)
		r.Row("New account", plan.NewID)
		r.Row("Label", plan.Account.Label)
	} else {
		if isPlan {
			r.ProductTitle("Profile rename plan")
		} else {
			r.ProductTitle("Profile renamed")
		}
		r.Row("Previous profile", plan.OldID)
		r.Row("New profile", plan.NewID)
		r.Row("Account", plan.Profile.Account)
	}
	if len(plan.AffectedReferences) > 0 {
		r.Row(referenceLabel, fmt.Sprintf("%d", len(plan.AffectedReferences)))
	}
	for _, todo := range plan.ExternalTODOs {
		r.Row("External action", todo)
	}
	switch plan.Status {
	case renaming.StatusBlocked:
		r.Status(presentation.Warn, "Plan", "Blocked; no changes were made")
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
