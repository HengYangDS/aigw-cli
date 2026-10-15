package configuration

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"slices"

	"aigw-cli/internal/transaction"

	"go.yaml.in/yaml/v3"
)

// Selection is Hermes' own selected provider and model.
type Selection struct {
	Provider string
	Model    string
}

// ObserveSelection reports Hermes' own selection when the operator changed it to
// another model in AIGW's projected catalogue and nothing else that AIGW owns
// changed. It reports nothing when the owned settings are unchanged or absent,
// and refuses any other managed drift.
func ObserveSelection(path string, current Desired) (Selection, bool, error) {
	if err := current.Validate(); err != nil {
		return Selection{}, false, err
	}
	config, err := transaction.CaptureFileSnapshot(path)
	if err != nil {
		return Selection{}, false, err
	}
	stateSnapshot, err := transaction.CaptureFileSnapshot(path + stateSuffix)
	if err != nil || !stateSnapshot.Exists {
		return Selection{}, false, err
	}
	root, err := parse(config.Data)
	if err != nil {
		return Selection{}, false, err
	}
	state, err := decodeOwnership(stateSnapshot.Data)
	if err != nil {
		return Selection{}, false, err
	}
	owned, err := managedBytes(root, state.ProviderIDs)
	if err != nil {
		return Selection{}, false, err
	}
	if digest(owned) == state.ManagedDigest {
		return Selection{}, false, nil
	}
	if !selectionOnlyDrift(root, state, &current) {
		return Selection{}, false, errExternalEdit
	}
	model := field(root, "model")
	return Selection{Provider: field(model, "provider").Value, Model: field(model, "default").Value}, true, nil
}

var errExternalEdit = errors.New("managed Hermes settings changed outside AIGW; preserve the edit before synchronization")

func decodeOwnership(data []byte) (ownership, error) {
	var state ownership
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return state, errors.Join(errors.New("invalid Hermes ownership record"), err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) || state.Version != 2 || state.ManagedDigest == "" || len(state.ProviderIDs) == 0 {
		return state, errors.New("invalid Hermes ownership record")
	}
	return state, nil
}

// selectionOnlyDrift proves that Hermes' selection names a model of an
// AIGW-owned provider and that replacing it with some owned provider and model
// reproduces the recorded owned settings, so only the selection moved. The
// proof does not depend on the current binding, which may already name the
// native choice. previous only authorizes the proof.
func selectionOnlyDrift(root *yaml.Node, state ownership, previous *Desired) bool {
	if previous == nil {
		return false
	}
	model := field(root, "model")
	selected := field(model, "provider")
	if selected == nil || !slices.Contains(state.ProviderIDs, selected.Value) {
		return false
	}
	provider := field(field(root, "providers"), selected.Value)
	for _, keys := range [][2]string{{"base_url", "base_url"}, {"api_mode", "transport"}} {
		actual, owned := field(model, keys[0]), field(provider, keys[1])
		if actual == nil || owned == nil || actual.Value != owned.Value {
			return false
		}
	}
	defaultModel, models := field(model, "default"), field(provider, "models")
	if defaultModel == nil || models == nil || !slices.ContainsFunc(models.Content, func(node *yaml.Node) bool { return node.Value == defaultModel.Value }) {
		return false
	}
	defer setField(root, "model", model)
	for _, providerID := range state.ProviderIDs {
		owned := field(field(root, "providers"), providerID)
		baseURL, transport, candidates := field(owned, "base_url"), field(owned, "transport"), field(owned, "models")
		if baseURL == nil || transport == nil || candidates == nil {
			continue
		}
		for _, candidate := range candidates.Content {
			replacement := mapping()
			setString(replacement, "provider", providerID)
			setString(replacement, "default", candidate.Value)
			setString(replacement, "base_url", baseURL.Value)
			setString(replacement, "api_mode", transport.Value)
			setField(root, "model", replacement)
			if proven, err := managedBytes(root, state.ProviderIDs); err == nil && digest(proven) == state.ManagedDigest {
				return true
			}
		}
	}
	return false
}

// selects reports whether Hermes' selection already names the desired provider
// and model.
func (desired Desired) selects(root *yaml.Node) bool {
	model := field(root, "model")
	provider, defaultModel := field(model, "provider"), field(model, "default")
	return provider != nil && defaultModel != nil && provider.Value == desired.SelectedProvider && defaultModel.Value == desired.SelectedModel
}
