package hermes

import (
	"context"
	"fmt"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
)

// RecordPlan is a prepared write of AIGW's record of Hermes' file alone: it
// brings the record to the selection that the file shows. Hermes' file is
// read and never written, not even with the bytes that it holds, so the
// write needs neither a Token nor a credential command.
type RecordPlan struct {
	path   string
	before platform.FileSnapshot
	after  []byte
}

// PrepareRecord prepares the write of AIGW's record of the file at path for
// shown, the selection that the configuration follows: the file's own
// selection is that listed model of a provider entry that AIGW owns, and
// nothing else that AIGW owns differs from the record. A record that already
// holds what the file shows is left. Any other file is refused as
// ObserveSelection refuses it, and a choice of another model than shown as
// an outside edit, because no configuration follows it yet.
func PrepareRecord(path string, shown Selection) (RecordPlan, error) {
	plan := RecordPlan{path: path + adapter.StateSuffix}
	file, err := observe(path)
	if err != nil || !file.changed {
		return plan, err
	}
	if file.selection != shown {
		return plan, errExternalEdit
	}
	owned, err := managedBytes(file.root, file.state.ProviderIDs)
	if err != nil {
		return plan, err
	}
	file.state.ManagedDigest = platform.SHA256(owned)
	plan.before = file.record
	plan.after, err = adapter.EncodeJSON(file.state)
	return plan, err
}

// ChangesState reports whether applying the plan changes the record.
func (plan RecordPlan) ChangesState() bool { return plan.after != nil }

// Effects returns the one file that applying the plan may write: the record.
func (plan RecordPlan) Effects() []string { return []string{plan.path} }

// Apply writes the record while it still holds what the plan read.
func (plan RecordPlan) Apply() (RecordReceipt, error) {
	if !plan.ChangesState() {
		return RecordReceipt{}, nil
	}
	after, err := platform.WriteFileAtomicIfUnchanged(plan.path, plan.before, plan.after, 0o600)
	if err != nil {
		return RecordReceipt{}, err
	}
	return RecordReceipt{path: plan.path, before: plan.before, after: after, changed: true}, nil
}

// RecordReceipt restores the record that a RecordPlan replaced.
type RecordReceipt struct {
	path          string
	before, after platform.FileSnapshot
	changed       bool
}

// Rollback restores the record while it still holds what the plan wrote.
func (receipt RecordReceipt) Rollback() error { return receipt.Restore().Err() }

// Restore rolls the record back as Rollback does and reports its file.
func (receipt RecordReceipt) Restore() platform.Restoration {
	if !receipt.changed {
		return nil
	}
	return platform.Restoration{platform.Restore(receipt.path, receipt.before, receipt.after)}
}

// hermesRecord prepares the record of the selection that followed names for
// Hermes, and returns the file that the record is of. A client that is not
// enabled on one file has nothing to record.
func hermesRecord(followed domain.Config) (RecordPlan, string, error) {
	binding := followed.Clients[ID]
	if !binding.Enabled || len(binding.Targets) != 1 {
		return RecordPlan{}, "", nil
	}
	selected, err := followed.ResolveRuntime(ID, domain.ModelRef{})
	if err != nil {
		return RecordPlan{}, "", err
	}
	target := binding.Targets[0]
	plan, err := PrepareRecord(target, Selection{Provider: hermesProviderID(selected.ProviderID, selected.Protocol), Model: selected.UpstreamModel})
	return plan, target, err
}

// RecordPending reports whether AIGW's record of Hermes' file is behind the
// model that the file selects, which followed names, and can be brought to it
// without writing the file. While the file shows what the record holds it
// reads the file and the record alone. A record that is behind is pending
// exactly when the ordinary projection of followed would be admitted: the
// file is there, nothing else that AIGW owns in it changed, and it selects the
// model that followed names.
func (clientAdapter hermesAdapter) RecordPending(deps adapter.Dependencies, followed domain.Config) bool {
	binding := followed.Clients[ID]
	if !binding.Enabled || len(binding.Targets) != 1 {
		return false
	}
	if _, changed, err := ObserveSelection(binding.Targets[0]); err != nil || !changed {
		return false
	}
	if _, err := clientAdapter.Plan(deps, followed, followed); err != nil {
		return false
	}
	plan, _, err := hermesRecord(followed)
	return err == nil && plan.ChangesState()
}

// PlanRecord plans the write that brings AIGW's record of Hermes' file to the
// model chosen in Hermes, which followed names as its selection.
func (hermesAdapter) PlanRecord(_ adapter.Dependencies, followed domain.Config) ([]adapter.ProjectionPlan, error) {
	plan, target, err := hermesRecord(followed)
	if err != nil || !plan.ChangesState() {
		return nil, err
	}
	return []adapter.ProjectionPlan{{Client: ID, Target: target, Action: adapter.ActionRecord, Surface: hermesSurface, Effects: plan.Effects(), ChangesState: true}}, nil
}

// ApplyRecord writes what PlanRecord plans.
func (hermesAdapter) ApplyRecord(_ context.Context, _ adapter.Dependencies, followed domain.Config) (adapter.ProjectionReceipt, error) {
	plan, _, err := hermesRecord(followed)
	if err != nil {
		return nil, err
	}
	receipt, err := plan.Apply()
	if err != nil {
		return nil, fmt.Errorf("record the model chosen in Hermes in %s: %w", plan.path, err)
	}
	return receipt, nil
}
