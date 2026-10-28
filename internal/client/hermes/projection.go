// Package hermes owns reversible Hermes provider and model projection.
package hermes

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"aigw-cli/internal/transaction"

	"go.yaml.in/yaml/v3"
)

// Provider is one AIGW-owned Hermes provider projection.
type Provider struct {
	ID                string
	Name              string
	Models            []string
	Endpoint          string
	Protocol          string
	CredentialCommand string
}

// Desired is the complete non-secret AIGW projection into Hermes.
type Desired struct {
	SelectedProvider string
	SelectedModel    string
	Providers        []Provider
}

// Validate checks that the selected model belongs to one complete provider.
func (desired Desired) Validate() error {
	if strings.TrimSpace(desired.SelectedProvider) == "" || strings.TrimSpace(desired.SelectedModel) == "" {
		return errors.New("Hermes selected provider and model are required")
	}
	providerIDs := make(map[string]bool, len(desired.Providers))
	selected := false
	for _, provider := range desired.Providers {
		if err := provider.validate(); err != nil {
			return err
		}
		if providerIDs[provider.ID] {
			return fmt.Errorf("Hermes provider %q is duplicated", provider.ID)
		}
		providerIDs[provider.ID] = true
		if provider.ID == desired.SelectedProvider && slices.Contains(provider.Models, desired.SelectedModel) {
			selected = true
		}
	}
	if !selected {
		return errors.New("Hermes selected model is absent from its provider catalogue")
	}
	return nil
}

func (provider Provider) validate() error {
	for name, value := range map[string]string{"provider ID": provider.ID, "endpoint": provider.Endpoint, "credential command": provider.CredentialCommand} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("Hermes %s is required", name)
		}
	}
	if len(provider.Models) == 0 {
		return fmt.Errorf("Hermes provider %q requires at least one model", provider.ID)
	}
	_, err := nativeProtocol(provider.Protocol)
	return err
}

// Plan describes a prepared configuration change without applying it.
type Plan struct {
	Action       string
	path         string
	configBefore transaction.FileSnapshot
	stateBefore  transaction.FileSnapshot
	configAfter  []byte
	stateAfter   []byte
	removeConfig bool
	keepConfig   bool
	commands     map[string]string
}

// ChangesState reports whether applying the plan changes owned configuration.
func (plan *Plan) ChangesState() bool { return plan.Action == "write" || plan.Action == "remove" }

// Effects returns every file that applying the plan may write or remove: the
// configuration and its ownership state.
func (plan *Plan) Effects() []string { return []string{plan.path, plan.path + stateSuffix} }

// ObservedCredentialCommand returns one sidecar-validated managed command.
func (plan *Plan) ObservedCredentialCommand(providerID string) (string, bool) {
	command, ok := plan.commands[providerID]
	return command, ok
}

// Receipt retains the exact file snapshots needed for guarded compensation.
type Receipt struct {
	path         string
	configBefore transaction.FileSnapshot
	stateBefore  transaction.FileSnapshot
	configAfter  transaction.FileSnapshot
	stateAfter   transaction.FileSnapshot
	changed      bool
}

type ownership struct {
	Version          int        `json:"version"`
	OriginalModel    *yaml.Node `json:"original_model"`
	ModelPresent     bool       `json:"model_present"`
	ProvidersPresent bool       `json:"providers_present"`
	ProviderIDs      []string   `json:"provider_ids"`
	FilePresent      bool       `json:"file_present"`
	ManagedDigest    string     `json:"managed_digest"`
}

const stateSuffix = ".aigw-state.json"

// Prepare validates and prepares one projection or its withdrawal.
func Prepare(path string, desired *Desired) (Plan, error) {
	return prepare(path, desired, nil)
}

// PrepareAuthorizedSelection admits an explicit selection only when replacing
// the selected tuple with the prior tuple proves all other managed state.
func PrepareAuthorizedSelection(path string, desired, previous Desired) (Plan, error) {
	return prepareSelection(path, desired, previous, false)
}

// PrepareAdoptedSelection prepares the projection after synchronization adopted
// Hermes' own selection. It admits proved selection drift only when the same
// observed file already selects the desired model, so it never replaces the
// native choice.
func PrepareAdoptedSelection(path string, desired, previous Desired) (Plan, error) {
	return prepareSelection(path, desired, previous, true)
}

func prepareSelection(path string, desired, previous Desired, adopted bool) (Plan, error) {
	if err := previous.Validate(); err != nil {
		return Plan{}, err
	}
	return prepareObserved(path, &desired, &previous, adopted)
}

func prepare(path string, desired, previous *Desired) (Plan, error) {
	return prepareObserved(path, desired, previous, false)
}

func prepareObserved(path string, desired, previous *Desired, adopted bool) (Plan, error) {
	plan := Plan{path: path, Action: "unchanged"}
	var err error
	plan.configBefore, err = transaction.CaptureFileSnapshot(path)
	if err != nil {
		return Plan{}, err
	}
	plan.stateBefore, err = transaction.CaptureFileSnapshot(path + stateSuffix)
	if err != nil {
		return Plan{}, err
	}
	if desired == nil && !plan.stateBefore.Exists {
		return plan, nil
	}
	root, err := parse(plan.configBefore.Data)
	if err != nil {
		return Plan{}, err
	}
	if adopted && !desired.selects(root) {
		return Plan{}, errExternalEdit
	}
	state, before, err := plan.readOwnership(root, desired, previous)
	if err != nil {
		return Plan{}, err
	}
	if plan.stateBefore.Exists {
		plan.commands = make(map[string]string, len(state.ProviderIDs))
		providers := field(root, "providers")
		for _, id := range state.ProviderIDs {
			command := field(field(providers, id), "key_cmd")
			if command != nil && command.Kind == yaml.ScalarNode {
				plan.commands[id] = command.Value
			}
		}
	}
	if desired == nil {
		if err := restore(root, state); err != nil {
			return Plan{}, err
		}
		plan.Action = "remove"
		plan.removeConfig = !state.FilePresent && len(root.Content) == 0
	} else if err := plan.prepareDesired(root, *desired, state, before); err != nil {
		return Plan{}, err
	}
	if plan.Action == "unchanged" {
		return plan, nil
	}
	plan.configAfter, err = encode(root)
	if err != nil {
		return Plan{}, err
	}
	if plan.keepConfig {
		plan.configAfter = plan.configBefore.Data
	}
	return plan, nil
}

func (plan *Plan) prepareDesired(root *yaml.Node, desired Desired, state ownership, before []byte) error {
	if err := desired.Validate(); err != nil {
		return err
	}
	orderChanged := !slices.Equal(ownedProviderOrder(root, state.ProviderIDs), desired.providerIDs())
	providers := field(root, "providers")
	if providers != nil {
		setField(providers, legacyProviderID, nil)
		for _, providerID := range state.ProviderIDs {
			setField(providers, providerID, nil)
		}
	}
	for _, provider := range desired.Providers {
		if !slices.Contains(state.ProviderIDs, provider.ID) && field(providers, provider.ID) != nil {
			return fmt.Errorf("Hermes provider %s already exists without AIGW ownership", provider.ID)
		}
	}
	project(root, desired)
	state.ProviderIDs = desired.providerIDs()
	after, err := managedBytes(root, state.ProviderIDs)
	if err != nil {
		return err
	}
	afterDigest := digest(after)
	if plan.stateBefore.Exists && bytes.Equal(before, after) && !orderChanged && state.ManagedDigest == afterDigest {
		return nil
	}
	if plan.stateBefore.Exists && bytes.Equal(before, after) && !orderChanged {
		// Hermes already shows the desired projection; record it without
		// rewriting the operator's file.
		plan.keepConfig = true
	}
	state.ManagedDigest = afterDigest
	plan.stateAfter, err = json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	plan.stateAfter = append(plan.stateAfter, '\n')
	plan.Action = "write"
	return nil
}

func (plan *Plan) readOwnership(root *yaml.Node, desired, previous *Desired) (ownership, []byte, error) {
	if !plan.stateBefore.Exists {
		return initialOwnership(root, desired, plan.configBefore.Exists)
	}
	state, err := decodeOwnership(plan.stateBefore.Data)
	if err != nil {
		return state, nil, err
	}
	current, err := managedBytes(root, state.ProviderIDs)
	if err != nil {
		return state, nil, err
	}
	if digest(current) != state.ManagedDigest && !selectionOnlyDrift(root, state, previous) {
		return state, nil, errExternalEdit
	}
	return state, current, nil
}

func initialOwnership(root *yaml.Node, desired *Desired, filePresent bool) (ownership, []byte, error) {
	if desired == nil {
		return ownership{}, nil, errors.New("Hermes ownership record is required for withdrawal")
	}
	providerIDs := desired.providerIDs()
	providers := field(root, "providers")
	if field(providers, legacyProviderID) != nil {
		return ownership{}, nil, errors.New("Hermes provider aigw already exists without AIGW ownership")
	}
	for _, providerID := range providerIDs {
		if field(providers, providerID) != nil {
			return ownership{}, nil, fmt.Errorf("Hermes provider %s already exists without AIGW ownership", providerID)
		}
	}
	current, err := managedBytes(root, providerIDs)
	return ownership{
		Version: 2, OriginalModel: selectedModel(root), ModelPresent: field(root, "model") != nil,
		ProvidersPresent: providers != nil, ProviderIDs: providerIDs, FilePresent: filePresent,
	}, current, err
}

func restore(root *yaml.Node, state ownership) error {
	model := field(root, "model")
	if state.OriginalModel != nil && state.OriginalModel.Kind == yaml.ScalarNode {
		for index := 0; index < len(model.Content); index += 2 {
			if field(selectedModel(root), model.Content[index].Value) == nil {
				return errors.New("Hermes model gained independent fields; preserve them before restoring the original scalar model")
			}
		}
		setField(root, "model", state.OriginalModel)
	} else {
		for _, key := range modelKeys {
			setField(model, key, field(state.OriginalModel, key))
		}
		if !state.ModelPresent && len(model.Content) == 0 {
			setField(root, "model", nil)
		}
	}
	providers := field(root, "providers")
	for _, providerID := range state.ProviderIDs {
		setField(providers, providerID, nil)
	}
	if !state.ProvidersPresent && len(providers.Content) == 0 {
		setField(root, "providers", nil)
	}
	return nil
}

func (desired Desired) providerIDs() []string {
	ids := make([]string, 0, len(desired.Providers))
	for _, provider := range desired.Providers {
		ids = append(ids, provider.ID)
	}
	return ids
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Apply commits the prepared projection while protecting newer edits.
func (plan *Plan) Apply() (Receipt, error) {
	if plan.Action == "unchanged" {
		return Receipt{}, nil
	}
	receipt := Receipt{path: plan.path, configBefore: plan.configBefore, stateBefore: plan.stateBefore, changed: true}
	var err error
	if plan.removeConfig {
		receipt.configAfter, err = transaction.RemoveFileIfUnchanged(plan.path, plan.configBefore)
	} else {
		receipt.configAfter, err = transaction.WriteFileAtomicIfUnchanged(plan.path, plan.configBefore, plan.configAfter, 0o600)
	}
	if err != nil {
		return Receipt{}, err
	}
	if plan.Action == "remove" {
		receipt.stateAfter, err = transaction.RemoveFileIfUnchanged(plan.path+stateSuffix, plan.stateBefore)
	} else {
		receipt.stateAfter, err = transaction.WriteFileAtomicIfUnchanged(plan.path+stateSuffix, plan.stateBefore, plan.stateAfter, 0o600)
	}
	if err != nil {
		return Receipt{}, transaction.CompensationFailed(err, transaction.RestoreFileAtomicIfPostimage(plan.path, plan.configBefore, receipt.configAfter))
	}
	return receipt, nil
}

// Rollback restores only the receipt's unchanged postimages.
func (receipt Receipt) Rollback() error {
	if !receipt.changed {
		return nil
	}
	// Retain ownership if configuration compensation is unsafe.
	if err := transaction.RestoreFileAtomicIfPostimage(receipt.path, receipt.configBefore, receipt.configAfter); err != nil {
		return err
	}
	return transaction.RestoreFileAtomicIfPostimage(receipt.path+stateSuffix, receipt.stateBefore, receipt.stateAfter)
}
