package codex

import (
	"errors"
	"slices"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
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

// choice is how Codex's files stand together against AIGW's records of them:
// the distinct models that files select in place of the recorded one, the
// recorded model, and whether a file was edited outside.
type choice struct {
	models   []string
	recorded string
	drifted  bool
}

// observeChoice reads every target against AIGW's record of it. With
// observeSelection it is the one owner of what counts as Codex's own choice,
// for following it, for inspecting a file and for running the client on it.
func observeChoice(targets []string) (choice, error) {
	var observed choice
	for _, target := range targets {
		file, err := observeSelection(target)
		if err != nil {
			return choice{}, err
		}
		if observed.recorded == "" {
			observed.recorded = file.recorded
		}
		switch file.state {
		case selectionDrifted:
			observed.drifted = true
		case selectionChosen:
			if !slices.Contains(observed.models, file.model) {
				observed.models = append(observed.models, file.model)
			}
		case selectionRecorded:
		}
	}
	return observed, nil
}

// followedChoice returns the Route that AIGW last wrote for Codex, when
// Codex's own files select the model that selected names in its place: one
// or more files chose it, none chose another, and none was edited outside.
// selected carries the credential reader that the files retain, which does
// not depend on the model.
func followedChoice(cfg domain.Config, selected domain.Runtime) (domain.Runtime, bool) {
	observed, err := observeChoice(cfg.Clients[ID].Targets)
	if err != nil || observed.drifted || len(observed.models) != 1 || observed.models[0] != selected.UpstreamModel {
		return domain.Runtime{}, false
	}
	earlier := cfg.Clone()
	earlier.SetSelectedModel(ID, domain.NewModelRef(selected.ProviderID, observed.recorded), "")
	recorded, err := earlier.ResolveRuntime(ID, domain.ModelRef{})
	if err != nil {
		return domain.Runtime{}, false
	}
	recorded.CredentialCommand = selected.CredentialCommand
	return recorded, true
}

// showsSelection reports whether target holds AIGW's projection of selected:
// what AIGW writes for it, or, after Codex's own choice of that model, what
// AIGW last wrote with that one value exchanged, so that nothing else waits
// for a write. A conflict is returned as it is.
func showsSelection(cfg domain.Config, selected domain.Runtime, target string) error {
	err := ValidateConfig(target, selected)
	if err == nil {
		return nil
	}
	if _, conflict := errors.AsType[*adapter.ConflictError](err); conflict {
		return err
	}
	if recorded, followed := followedChoice(cfg, selected); followed && validateConfig(target, recorded, selected.UpstreamModel) == nil {
		return nil
	}
	return err
}

// awaitsRecord reports whether target still holds what AIGW last wrote
// while another of Codex's files selects the model that the configuration
// follows: the command that records the choice writes it here too.
func awaitsRecord(cfg domain.Config, selected domain.Runtime, target string) bool {
	recorded, followed := followedChoice(cfg, selected)
	return followed && ValidateConfig(target, recorded) == nil
}

// observeSelection reads how the target at path stands against AIGW's record
// of it.
func observeSelection(path string) (observation, error) {
	file, _, err := readSelection(path)
	return file, err
}

// selectionRecord is AIGW's record of a target as one observation read it,
// and where it is kept.
type selectionRecord struct {
	path string
	file platform.FileSnapshot
}

// readSelection observes the target at path as observeSelection does and
// returns the record that it read, for a write of the record alone.
func readSelection(path string) (observation, selectionRecord, error) {
	config, err := adapter.CaptureTarget(path, true)
	if err != nil || !config.Exists {
		return observation{}, selectionRecord{}, err
	}
	record := selectionRecord{path: codexRecordPath(path)}
	record.file, err = adapter.CaptureTarget(record.path, true)
	if err != nil || !record.file.Exists {
		return observation{}, selectionRecord{}, err
	}
	canonical, err := platform.CanonicalPath(path)
	if err != nil {
		return observation{}, selectionRecord{}, nil
	}
	catalog, err := platform.CaptureFileSnapshot(codexCatalogPath(canonical))
	if err != nil {
		return observation{}, selectionRecord{}, err
	}
	state, err := codexStateForTarget(record.file)
	if err != nil || state.ProjectedModel == "" {
		return observation{}, selectionRecord{}, nil
	}
	text := string(config.Data)
	if !ownedSelectionIntact(path, text, state, catalog) {
		return observation{recorded: state.ProjectedModel, state: selectionDrifted}, record, nil
	}
	modelLine, err := codexSelectionLine(text, "model")
	model, _ := selectionValue(modelLine, "model")
	if err != nil || model == "" || model == state.ProjectedModel {
		return observation{recorded: state.ProjectedModel}, record, nil
	}
	return observation{model: model, recorded: state.ProjectedModel, state: selectionChosen}, record, nil
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
