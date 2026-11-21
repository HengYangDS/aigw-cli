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

// SelectionRecorder is implemented by an adapter whose client's own choice of
// a model AIGW follows. It brings AIGW's record of the client's files to the
// selection that the files show, which followed names as the client's, and
// writes no client file, no catalog and no credential reader, so it needs no
// Token. A file that still shows the recorded selection is left; a file whose
// other managed settings changed is refused, as a projection refuses it.
type SelectionRecorder interface {
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
// record of the selection that before follows.
func planClient(deps Dependencies, client Adapter, before, after domain.Config) ([]ProjectionPlan, error) {
	records, only, err := recorder(deps, client)
	if err != nil {
		return nil, err
	}
	if only {
		return records.PlanRecord(deps, before)
	}
	return client.Plan(deps, before, after)
}

// applyClient applies what planClient plans for client.
func applyClient(ctx context.Context, deps Dependencies, client Adapter, before, after domain.Config) (ProjectionReceipt, error) {
	records, only, err := recorder(deps, client)
	if err != nil {
		return nil, err
	}
	if only {
		return records.ApplyRecord(ctx, deps, before)
	}
	return client.Apply(ctx, deps, before, after)
}
