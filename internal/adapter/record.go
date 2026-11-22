package adapter

import (
	"context"
	"fmt"
	"slices"

	"aigw-cli/internal/domain"
)

// ActionRecord is the action of a plan that brings AIGW's record of a client
// file to the selection that the file shows, and writes no client file.
const ActionRecord = "record"

// RecordOnly reports whether the plan brings AIGW's record of the client's
// file to what the file shows and leaves the file itself as it is.
func (plan ProjectionPlan) RecordOnly() bool { return plan.Action == ActionRecord }

// SelectionRecorder is implemented by an adapter whose client records its
// selection in a file of AIGW's beside the client's own. AIGW's record of the
// client's files must follow the selection that the files show at every
// commit: a record that stayed behind reads the client's next pick of the
// recorded model as no choice at all, and the next synchronization then writes
// the saved model over it. The recorder decides whether the record is behind
// and brings it up with a write of the record alone, which needs no Token and
// leaves every client file as it is.
type SelectionRecorder interface {
	// RecordPending reports whether AIGW's record of the client's files is
	// behind the selection that the files show, which followed names as the
	// client's, and can be brought to it without writing those files. It
	// reads the files and their records first, and nothing else while no
	// record is behind. A record that is behind is pending exactly when the
	// ordinary projection of followed would be admitted, so that one rule
	// decides for a client with a Token and for one without: a configured
	// file that is missing, unreadable or edited outside leaves the whole
	// client as it is stored, and a model chosen in it in its file alone.
	RecordPending(deps Dependencies, followed domain.Config) bool
	// PlanRecord describes ApplyRecord's effects at observation time: one
	// plan, whose action is ActionRecord and whose effects name the record
	// alone, for each file whose record changes.
	PlanRecord(deps Dependencies, followed domain.Config) ([]ProjectionPlan, error)
	ApplyRecord(ctx context.Context, deps Dependencies, followed domain.Config) (ProjectionReceipt, error)
}

// recorder returns client's SelectionRecorder when deps marks the client as
// one of which only AIGW's record is written. A client that is marked and
// records no selection is a wiring defect of the caller.
func recorder(deps Dependencies, client Adapter) (SelectionRecorder, bool, error) {
	if !slices.Contains(deps.RecordOnly, client.Spec().ID) {
		return nil, false, nil
	}
	records, implemented := client.(SelectionRecorder)
	if !implemented {
		return nil, false, fmt.Errorf("client %q keeps no record of a selection chosen in it", client.Spec().ID)
	}
	return records, true, nil
}

// planClient plans client's part of a transition from before to after: its
// projection, or, for a client of which only AIGW's record is written, the
// record of the selection that before follows; a client whose files are
// withdrawn alone plans the dropped files' restoration, and it too may need
// its record.
func planClient(deps Dependencies, client Adapter, before, after domain.Config) ([]ProjectionPlan, error) {
	records, only, err := recorder(deps, client)
	if err != nil {
		return nil, err
	}
	withdraw := slices.Contains(deps.WithdrawOnly, client.Spec().ID)
	if !only && !withdraw {
		return client.Plan(deps, before, after)
	}
	plans := make([]ProjectionPlan, 0)
	if withdraw {
		restored, err := client.Plan(deps, before, after)
		if err != nil {
			return nil, err
		}
		plans = append(plans, restored...)
	}
	if only {
		recorded, err := records.PlanRecord(deps, before)
		if err != nil {
			return nil, err
		}
		plans = append(plans, recorded...)
	}
	return plans, nil
}

// applyClient applies what planClient plans for client, the record last, so
// that AIGW's record follows the files as the transition left them.
func applyClient(ctx context.Context, deps Dependencies, client Adapter, before, after domain.Config) (ProjectionReceipt, error) {
	records, only, err := recorder(deps, client)
	if err != nil {
		return nil, err
	}
	withdraw := slices.Contains(deps.WithdrawOnly, client.Spec().ID)
	if !only && !withdraw {
		return client.Apply(ctx, deps, before, after)
	}
	var receipts ProjectionReceipts
	if withdraw {
		receipt, err := client.Apply(ctx, deps, before, after)
		if err != nil {
			return nil, err
		}
		if receipt != nil {
			receipts = append(receipts, receipt)
		}
	}
	if only {
		receipt, err := records.ApplyRecord(ctx, deps, before)
		if err != nil {
			return nil, err
		}
		if receipt != nil {
			receipts = append(receipts, receipt)
		}
	}
	return receipts, nil
}
