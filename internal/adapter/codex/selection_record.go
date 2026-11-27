package codex

import (
	"context"
	"fmt"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
)

// codexRecord is the prepared write of AIGW's record of one Codex file alone:
// the record names the model that the file selects in place of the recorded
// one. target is the file, as a projection's plan names it.
type codexRecord struct {
	target, surface string
	record          codexPreparedArtifact
}

// prepareCodexRecords prepares, for each of Codex's files that selects model
// in place of the model that AIGW recorded for it, the write that brings
// AIGW's record of that file to model. A file that still shows the recorded
// model is left, record and all: the next projection writes it. A file that
// selects another model, or whose other managed settings changed, refuses
// every record, because no configuration follows such files.
func prepareCodexRecords(targets []TargetRef, model string) ([]codexRecord, error) {
	normalized, err := normalizeCodexTargets(targets, true)
	if err != nil {
		return nil, err
	}
	records := make([]codexRecord, 0, len(normalized))
	for _, target := range normalized {
		record, err := prepareCodexRecord(target, model)
		if err != nil {
			return nil, &adapter.TargetError{Desired: true, Path: target.reportName(target.Path), Err: err}
		}
		if record != nil {
			records = append(records, *record)
		}
	}
	return records, nil
}

// prepareCodexRecord prepares the record of one target, or nothing for a
// target that still shows the recorded model.
func prepareCodexRecord(target TargetRef, model string) (*codexRecord, error) {
	file, record, err := readSelection(target.configured)
	if err != nil {
		return nil, err
	}
	switch {
	case file.state == selectionRecorded:
		return nil, nil
	case file.state == selectionDrifted:
		return nil, newConflictError("Codex config conflict: AIGW-managed settings changed; refusing to record a model selection over user edits", nil)
	case file.model != model:
		return nil, newConflictError(fmt.Sprintf("Codex config conflict: the file selects model %q, which the configuration does not follow; refusing to record it", file.model), nil)
	}
	state, err := codexStateForTarget(record.file)
	if err != nil {
		return nil, err
	}
	state.ProjectedModel = model
	data, err := adapter.EncodeJSON(state)
	if err != nil {
		return nil, err
	}
	return &codexRecord{target: target.Path, surface: target.SurfaceID, record: codexPreparedArtifact{
		path: record.path, before: record.file, desired: platform.NewFileSnapshot(data, record.file.Mode), name: target.reportName(record.path),
	}}, nil
}

// followedRecords prepares the records of the model that followed names as
// Codex's selection, which Codex's own files chose.
func followedRecords(deps adapter.Dependencies, followed domain.Config) ([]codexRecord, error) {
	binding := followed.Clients[ID]
	if !binding.Enabled || len(binding.Targets) == 0 {
		return nil, nil
	}
	selected, err := followed.ResolveRuntime(ID, domain.ModelRef{})
	if err != nil {
		return nil, err
	}
	discovered, err := discover(deps)
	if err != nil {
		return nil, err
	}
	targets, err := targetRefs(discovered, binding.Targets, programPath(deps, followed, discovered))
	if err != nil {
		return nil, err
	}
	return prepareCodexRecords(targets, selected.UpstreamModel)
}

// RecordPending reports whether a record of one of Codex's files is behind
// the model that the file selects, which followed names, and can be brought
// to it without writing the files. While every file shows its recorded model
// it reads the files and their records alone. A record that is behind is
// pending exactly when the ordinary projection of followed would be admitted:
// every configured file is there with its record, none was edited outside,
// and each selects the recorded model or the one that followed names.
func (client codexAdapter) RecordPending(deps adapter.Dependencies, followed domain.Config) bool {
	binding := followed.Clients[ID]
	if !binding.Enabled || len(binding.Targets) == 0 {
		return false
	}
	if observed, err := observeChoice(binding.Targets); err != nil || observed.drifted || len(observed.models) == 0 {
		return false
	}
	if _, err := client.Plan(deps, followed, followed); err != nil {
		return false
	}
	records, err := followedRecords(deps, followed)
	return err == nil && len(records) > 0
}

// PlanRecord plans the writes that bring AIGW's records of Codex's files to
// the model chosen in Codex, which followed names as its selection: one plan
// for each file that shows the choice, naming its record alone.
func (codexAdapter) PlanRecord(deps adapter.Dependencies, followed domain.Config) ([]adapter.ProjectionPlan, error) {
	records, err := followedRecords(deps, followed)
	if err != nil {
		return nil, err
	}
	plans := make([]adapter.ProjectionPlan, 0, len(records))
	for _, record := range records {
		plans = append(plans, adapter.ProjectionPlan{Client: ID, Target: record.target, Action: adapter.ActionRecord, Surface: record.surface, Effects: []string{record.record.path}, ChangesState: true})
	}
	return plans, nil
}

// ApplyRecord writes what PlanRecord plans: each record while it still holds
// what the plan read, and none when one of them cannot be written.
func (codexAdapter) ApplyRecord(_ context.Context, deps adapter.Dependencies, followed domain.Config) (adapter.ProjectionReceipt, error) {
	records, err := followedRecords(deps, followed)
	if err != nil {
		return nil, err
	}
	committed := make([]committedCodexArtifact, 0, len(records))
	for _, record := range records {
		post, err := commitCodexArtifact(record.record)
		if err != nil {
			err = fmt.Errorf("record the model chosen in Codex in %s: %w", record.record.reported(), record.record.namedFailure(err))
			return nil, platform.FailedWrite(err, restoreCodexArtifacts(committed))
		}
		committed = append(committed, committedCodexArtifact{prepared: record.record, post: post})
	}
	return ReconciliationReceipt{committed: committed}, nil
}
