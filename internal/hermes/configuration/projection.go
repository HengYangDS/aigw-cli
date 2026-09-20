// Package configuration owns reversible Hermes provider and model projection.
package configuration

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"aigw-cli/internal/transaction"

	"go.yaml.in/yaml/v3"
)

// Route is the non-secret input to Hermes's native provider configuration.
type Route struct {
	Model             string
	Endpoint          string
	Protocol          string
	CredentialCommand string
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
	FilePresent      bool       `json:"file_present"`
	ManagedDigest    string     `json:"managed_digest"`
}

const stateSuffix = ".aigw-state.json"

// Prepare validates and prepares one projection or its withdrawal.
func Prepare(path string, route *Route) (Plan, error) {
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
	if route == nil && !plan.stateBefore.Exists {
		return plan, nil
	}
	root, err := parse(plan.configBefore.Data)
	if err != nil {
		return Plan{}, err
	}
	before, err := managedBytes(root)
	if err != nil {
		return Plan{}, err
	}
	state, err := plan.readOwnership(root, before)
	if err != nil {
		return Plan{}, err
	}
	if route == nil {
		if err := restore(root, state); err != nil {
			return Plan{}, err
		}
		plan.Action = "remove"
		plan.removeConfig = !state.FilePresent && len(root.Content) == 0
	} else if err := plan.prepareRoute(root, *route, state, before); err != nil {
		return Plan{}, err
	}
	if plan.Action == "unchanged" {
		return plan, nil
	}
	plan.configAfter, err = encode(root)
	if err != nil {
		return Plan{}, err
	}
	return plan, nil
}

func (plan *Plan) prepareRoute(root *yaml.Node, route Route, state ownership, before []byte) error {
	protocol, err := route.nativeProtocol()
	if err != nil {
		return err
	}
	project(root, route, protocol)
	after, err := managedBytes(root)
	if err != nil {
		return err
	}
	if plan.stateBefore.Exists && bytes.Equal(before, after) {
		return nil
	}
	state.ManagedDigest = digest(after)
	plan.stateAfter, err = json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	plan.stateAfter = append(plan.stateAfter, '\n')
	plan.Action = "write"
	return nil
}

func (plan *Plan) readOwnership(root *yaml.Node, current []byte) (ownership, error) {
	if plan.stateBefore.Exists {
		var state ownership
		decoder := json.NewDecoder(bytes.NewReader(plan.stateBefore.Data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&state); err != nil {
			return state, fmt.Errorf("invalid Hermes ownership record: %w", err)
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) || state.Version != 1 || state.ManagedDigest == "" {
			return state, errors.New("invalid Hermes ownership record")
		}
		if digest(current) != state.ManagedDigest {
			return state, errors.New("managed Hermes settings changed outside AIGW; preserve the edit before synchronization")
		}
		return state, nil
	}
	if field(field(root, "providers"), "aigw") != nil {
		return ownership{}, errors.New("Hermes provider aigw already exists without AIGW ownership")
	}
	return ownership{
		Version: 1, OriginalModel: selectedModel(root), ModelPresent: field(root, "model") != nil,
		ProvidersPresent: field(root, "providers") != nil, FilePresent: plan.configBefore.Exists,
	}, nil
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
	setField(providers, "aigw", nil)
	if !state.ProvidersPresent && len(providers.Content) == 0 {
		setField(root, "providers", nil)
	}
	return nil
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
		return Receipt{}, errors.Join(err, transaction.RestoreFileAtomicIfPostimage(plan.path, plan.configBefore, receipt.configAfter))
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
