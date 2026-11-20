package codex

import (
	"aigw-cli/internal/adapter"
	"aigw-cli/internal/platform"
)

// selectionState says how a Codex target's root model stands against AIGW's
// record of the target.
type selectionState int

const (
	// selectionRecorded: the file shows the recorded model, or AIGW holds no
	// record that could attribute a difference.
	selectionRecorded selectionState = iota
	// selectionChosen: the root model is the only value that AIGW owns in
	// the target that differs from the record: Codex's own choice.
	selectionChosen
	// selectionDrifted: another value that AIGW owns differs from the
	// record, with or without the root model: an outside edit, which a plan
	// refuses under its own name.
	selectionDrifted
)

// observation is one target's root model, the model that AIGW recorded for
// it, and how the two stand.
type observation struct {
	model, recorded string
	state           selectionState
}

// observeSelection reads how the target at path stands against AIGW's record
// of it. It is the one owner of what counts as Codex's own choice.
func observeSelection(path string) (observation, error) {
	config, err := adapter.CaptureTarget(path, true)
	if err != nil || !config.Exists {
		return observation{}, err
	}
	stateSnapshot, err := adapter.CaptureTarget(codexRecordPath(path), true)
	if err != nil || !stateSnapshot.Exists {
		return observation{}, err
	}
	canonical, err := canonicalCodexTargetPath(path)
	if err != nil {
		return observation{}, nil
	}
	catalog, err := platform.CaptureFileSnapshot(codexCatalogPath(canonical))
	if err != nil {
		return observation{}, err
	}
	state, err := codexStateForTarget(stateSnapshot)
	if err != nil || state.ProjectedModel == "" {
		return observation{}, nil
	}
	text := string(config.Data)
	if !ownedSelectionIntact(path, text, state, catalog) {
		return observation{recorded: state.ProjectedModel, state: selectionDrifted}, nil
	}
	modelLine, err := codexSelectionLine(text, "model")
	model := selectionStringValue(modelLine)
	if err != nil || model == "" || selectionHasValue(modelLine, "model", state.ProjectedModel) {
		return observation{recorded: state.ProjectedModel}, nil
	}
	return observation{model: model, recorded: state.ProjectedModel, state: selectionChosen}, nil
}

// ownedSelectionIntact reports whether the provider selection, provider table,
// scheduler and catalog still match the sidecar.
func ownedSelectionIntact(path, text string, state codexState, catalog platform.FileSnapshot) bool {
	provider := codexStateProvider(state)
	block, err := codexManagedBlockForProviderIn(text, provider)
	if err != nil || state.ManagedBlockHash != platform.SHA256([]byte(block)) {
		return false
	}
	providerLine, err := codexSelectionLine(text, "model_provider")
	if err != nil || !selectionHasValue(providerLine, "model_provider", provider) {
		return false
	}
	if validateCodexSchedulerOwnership(state, text) != nil || validateCodexCatalogReference(path, text, state) != nil {
		return false
	}
	return state.CatalogHash == "" || catalog.Exists && platform.SHA256(catalog.Data) == state.CatalogHash
}
