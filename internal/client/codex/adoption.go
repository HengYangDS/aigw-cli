package codex

import (
	"aigw-cli/internal/platform"
)

// ObserveSelection returns Codex's own root model when it is the only value AIGW
// owns in the target at path that differs from the recorded projection. Any
// other drift, or a sidecar that cannot attribute the projected model, reports
// no change, so ordinary synchronization keeps its refusal.
func ObserveSelection(path string) (string, bool, error) {
	config, err := platform.CaptureFileSnapshot(path)
	if err != nil || !config.Exists {
		return "", false, err
	}
	stateSnapshot, err := platform.CaptureFileSnapshot(codexStatePath(path))
	if err != nil || !stateSnapshot.Exists {
		return "", false, err
	}
	canonical, err := canonicalCodexTargetPath(path)
	if err != nil {
		return "", false, nil
	}
	catalog, err := platform.CaptureFileSnapshot(codexCatalogPath(canonical))
	if err != nil {
		return "", false, err
	}
	state, err := codexStateForTarget(stateSnapshot)
	if err != nil || state.ProjectedModel == "" {
		return "", false, nil
	}
	text := string(config.Data)
	if !ownedSelectionIntact(path, text, state, catalog) {
		return "", false, nil
	}
	modelLine, err := codexSelectionLine(text, "model")
	model := selectionStringValue(modelLine)
	if err != nil || model == "" || selectionHasValue(modelLine, "model", state.ProjectedModel) {
		return "", false, nil
	}
	return model, true, nil
}

// ownedSelectionIntact reports whether the provider selection, provider table,
// scheduler and catalogue still match the sidecar.
func ownedSelectionIntact(path, text string, state codexState, catalog platform.FileSnapshot) bool {
	provider := codexStateProvider(state)
	block, err := codexManagedBlockForProviderIn(text, provider)
	if err != nil || !managedBlockHashMatches(state.ManagedBlockHash, block) {
		return false
	}
	providerLine, err := codexSelectionLine(text, "model_provider")
	if err != nil || !selectionHasValue(providerLine, "model_provider", provider) {
		return false
	}
	if validateCodexSchedulerOwnership(state, text) != nil || validateCodexCatalogReference(path, text, state) != nil {
		return false
	}
	return state.CatalogHash == "" || catalog.Exists && hashBytes(catalog.Data) == state.CatalogHash
}
