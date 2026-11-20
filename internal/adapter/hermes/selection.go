package hermes

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/platform"

	"go.yaml.in/yaml/v4"
)

// Selection is Hermes' own selected provider entry and model.
type Selection struct {
	Provider string
	Model    string
}

// selectionKind says what Hermes' own model block selects among the provider
// entries that AIGW owns.
type selectionKind int

const (
	// kindListed is an owned entry, with that entry's address and mode, and a
	// model that the entry lists: a choice that AIGW can follow.
	kindListed selectionKind = iota
	// kindUnlisted is an owned entry, with that entry's address and mode, and
	// a model that the entry does not list.
	kindUnlisted
	// kindEdited is an owned entry whose address or mode the block changed:
	// an edit of an endpoint, not a choice of a model.
	kindEdited
	// kindForeign is no owned entry: another provider's selection, or no
	// selection that AIGW can read.
	kindForeign
)

// classifySelection reads Hermes' own selection and says what it selects.
func classifySelection(root *yaml.Node, state ownership) (Selection, selectionKind) {
	model := field(root, "model")
	selection := Selection{Provider: scalar(field(model, "provider")), Model: scalar(field(model, "default"))}
	if !slices.Contains(state.ProviderIDs, selection.Provider) {
		return selection, kindForeign
	}
	provider := field(field(root, "providers"), selection.Provider)
	for _, keys := range [][2]string{{"base_url", "base_url"}, {"api_mode", "transport"}} {
		actual, owned := field(model, keys[0]), field(provider, keys[1])
		if actual == nil || owned == nil || actual.Value != owned.Value {
			return selection, kindEdited
		}
	}
	models := field(provider, "models")
	if field(model, "default") == nil || models == nil {
		return selection, kindEdited
	}
	if slices.ContainsFunc(models.Content, func(node *yaml.Node) bool { return node.Value == selection.Model }) {
		return selection, kindListed
	}
	return selection, kindUnlisted
}

// ObserveSelection reports Hermes' own selection when the operator changed it
// to another listed model of a provider entry that AIGW owns and nothing else
// that AIGW owns changed. It reports nothing when the owned settings are
// unchanged or absent. A selection that is the only difference and that AIGW
// cannot follow is an unfollowedSelectionError; any other difference, an
// edited address or mode of the selection included, is an outside edit.
func ObserveSelection(path string) (Selection, bool, error) {
	config, err := adapter.CaptureTarget(path, true)
	if err != nil {
		return Selection{}, false, err
	}
	stateSnapshot, err := adapter.CaptureTarget(path+adapter.StateSuffix, true)
	if err != nil || !stateSnapshot.Exists {
		return Selection{}, false, err
	}
	if !config.Exists {
		return Selection{}, false, missingWithRecord(path)
	}
	root, err := parse(config.Data)
	if err != nil {
		return Selection{}, false, err
	}
	state, err := readOwnershipRecord(path, stateSnapshot.Data)
	if err != nil {
		return Selection{}, false, err
	}
	owned, err := managedBytes(root, state.ProviderIDs)
	if err != nil {
		return Selection{}, false, err
	}
	if platform.SHA256(owned) == state.ManagedDigest {
		return Selection{}, false, nil
	}
	if !onlySelectionDiffers(root, state) {
		return Selection{}, false, errExternalEdit
	}
	switch selection, kind := classifySelection(root, state); kind {
	case kindListed:
		return selection, true, nil
	case kindUnlisted, kindForeign:
		return Selection{}, false, &unfollowedSelectionError{Selection: selection, owned: kind == kindUnlisted}
	case kindEdited:
	}
	return Selection{}, false, errExternalEdit
}

// unfollowedSelectionError reports that Hermes' own selection is the only
// owned value that differs from the record, and that AIGW cannot follow it:
// owned says that it names a provider entry that AIGW owns and a model that
// the entry does not list; otherwise it names no such entry.
type unfollowedSelectionError struct {
	Selection
	owned bool
}

func (e *unfollowedSelectionError) Error() string {
	return fmt.Sprintf("Hermes selects model %q of provider %q, which AIGW's catalog does not list", e.Model, e.Provider)
}

// scalar is a node's value, or "" for an absent node.
func scalar(node *yaml.Node) string {
	if node == nil {
		return ""
	}
	return node.Value
}

var errExternalEdit = adapter.NewConflictError(
	"managed Hermes settings changed outside AIGW; refusing to overwrite user edits",
	adapter.UndoOutsideEdit("Hermes"), nil)

// missingWithRecord refuses a Hermes configuration file at path that was
// deleted while AIGW's record of it remains.
func missingWithRecord(path string) error {
	return adapter.MissingWithRecord("Hermes", path, path+adapter.StateSuffix, "")
}

// readOwnershipRecord decodes AIGW's record of the Hermes configuration at path.
func readOwnershipRecord(path string, data []byte) (ownership, error) {
	state, err := decodeOwnership(data)
	if err != nil {
		return state, adapter.UnreadableRecord(path+adapter.StateSuffix, err)
	}
	return state, nil
}

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

// onlySelectionDiffers proves that nothing but Hermes' selection differs from
// the record: replacing the selection with some owned provider and model
// reproduces the recorded owned settings. It says nothing about what the
// selection itself names, which classifySelection reads.
func onlySelectionDiffers(root *yaml.Node, state ownership) bool {
	model := field(root, "model")
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
			if proven, err := managedBytes(root, state.ProviderIDs); err == nil && platform.SHA256(proven) == state.ManagedDigest {
				return true
			}
		}
	}
	return false
}

// selects reports whether the file's own selection names this provider entry
// and model.
func (selection Selection) selects(root *yaml.Node) bool {
	model := field(root, "model")
	return scalar(field(model, "provider")) == selection.Provider && scalar(field(model, "default")) == selection.Model
}
