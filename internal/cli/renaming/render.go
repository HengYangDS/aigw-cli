package renaming

import (
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/renaming"
	"encoding/json"
	"fmt"
)

func writeResult(runtime invocation.Context, plan renaming.Plan, jsonMode bool) error {
	if jsonMode {
		enc := json.NewEncoder(runtime.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(plan)
	}

	r := invocation.Renderer(runtime)
	isPlan := plan.Status != "applied"
	referenceLabel := "Route references"
	if plan.Resource == "account" {
		referenceLabel = "Profile references"
		switch {
		case plan.Finalize && plan.Status == "finalized":
			r.ProductTitle("Account finalization complete")
		case plan.Finalize && plan.Status == "already-finalized":
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
