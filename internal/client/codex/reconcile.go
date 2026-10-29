package codex

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/store"

	"github.com/pelletier/go-toml/v2"
)

const (
	// ProjectionFullSelection identifies a projection that owns both provider and model selection.
	ProjectionFullSelection = "full-selection"
	// ProjectionWriterID identifies AIGW as the writer of its bounded Codex projection.
	ProjectionWriterID = "aigw-cli"
)

// TargetRef identifies a persistent configuration file together with the
// host surface and ownership mode that authorizes AIGW to change it.
// Executable is the client this target's configuration is read by. It is the
// only source of client-specific facts, such as the bundled model catalog, so a
// target without one is projected without them rather than against a guess.
type TargetRef struct {
	SurfaceID      string
	Authority      string
	ProjectionMode string
	Path           string
	Executable     string
	CreateIfAbsent bool
	statePath      string
}

// ReconciliationReceipt owns compensation for an applied client projection.
type ReconciliationReceipt struct {
	committed []committedCodexArtifact
}

// CopyProjection copies one complete AIGW-owned Codex projection to a private
// target, rebasing only the catalog reference that is tied to its path: the
// owned catalog, or a user-owned catalog named relative to the configuration.
// The copy keeps the operator's settings and leaves out its MCP servers,
// plugins, hooks, memories and notifications, so a verification home never
// starts extensions it cannot authorize.
func CopyProjection(source, target string) error {
	suffixes := [...]string{"", ".aigw-state.json", ".aigw-model-catalog.json"}
	var snapshots [len(suffixes)]store.FileSnapshot
	for index, suffix := range suffixes {
		snapshot, err := store.CaptureFileSnapshot(source + suffix)
		if err != nil {
			return err
		}
		snapshots[index] = snapshot
	}
	if snapshots[0].Exists {
		settings, err := withoutCodexExtensions(string(snapshots[0].Data))
		if err != nil {
			return err
		}
		snapshots[0].Data = []byte(settings)
	}
	var err error
	snapshots[0], err = rebaseCopiedCodexCatalog(source, target, snapshots[0], snapshots[1])
	if err != nil {
		return err
	}
	for index, suffix := range suffixes {
		snapshot := snapshots[index]
		if !snapshot.Exists {
			continue
		}
		if err := store.WriteFileAtomicExactMode(target+suffix, snapshot.Data, snapshot.Mode); err != nil {
			return err
		}
	}
	return nil
}

// Rollback restores the exact preimages captured by this reconciliation while
// each artifact still equals the postimage written by this transaction.
func (r ReconciliationReceipt) Rollback() error {
	return rollbackCodexArtifacts(r.committed)
}

type codexReconciliationTarget struct {
	ref     TargetRef
	desired bool
}

type codexPreparedArtifact struct {
	path    string
	before  store.FileSnapshot
	desired store.FileSnapshot
	// exactMode writes the desired mode instead of inheriting the one already on
	// disk. It is set for artifacts whose permissions AIGW owns.
	exactMode bool
}

type codexPreparedTarget struct {
	plan      ProjectionPlan
	artifacts []codexPreparedArtifact
}

// effects lists the files that applying the target's plan may write or remove:
// the configuration target itself, and every prepared artifact.
func (target codexPreparedTarget) effects() []string {
	effects := make([]string, 0, len(target.artifacts)+1)
	if target.plan.Target != "" {
		effects = append(effects, target.plan.Target)
	}
	for _, artifact := range target.artifacts {
		if artifact.path != target.plan.Target {
			effects = append(effects, artifact.path)
		}
	}
	return effects
}

type committedCodexArtifact struct {
	prepared codexPreparedArtifact
	post     store.FileSnapshot
}

// These seams let the reconciliation tests inject deterministic write failure
// and concurrent-edit scenarios. Production calls the store's file transactions.
var writeFileAtomicIfUnchanged = store.WriteFileAtomicIfUnchanged
var writeFileAtomicExactModeIfUnchanged = store.WriteFileAtomicExactModeIfUnchanged
var removeFileIfUnchanged = store.RemoveFileIfUnchanged
var restoreFileAtomicIfPostimage = store.RestoreFileAtomicIfPostimage

// PlanReconciliation prepares a before-to-after target transition
// without writing configuration, sidecars, credentials, or sessions.
func PlanReconciliation(before, after []TargetRef, runtime domain.Runtime) ([]ProjectionPlan, error) {
	return PlanReconciliationTransition(before, after, runtime, runtime)
}

// PlanReconciliationTransition prepares a before-to-after Route transition.
// The previous runtime attributes legacy sidecars that predate projected model
// identity without weakening semantic drift checks.
func PlanReconciliationTransition(before, after []TargetRef, previous, runtime domain.Runtime) ([]ProjectionPlan, error) {
	return planCodexReconciliationTransition(before, after, previous, runtime, false)
}

// PlanReconciliationAuthorizedTransition prepares an explicit Route selection
// that may replace root provider and model values after all other owned state
// still matches its sidecar.
func PlanReconciliationAuthorizedTransition(before, after []TargetRef, previous, runtime domain.Runtime) ([]ProjectionPlan, error) {
	return planCodexReconciliationTransition(before, after, previous, runtime, true)
}

func planCodexReconciliationTransition(before, after []TargetRef, previous, runtime domain.Runtime, replaceRootSelections bool) ([]ProjectionPlan, error) {
	prepared, err := prepareCodexReconciliationTransition(before, after, previous, runtime, replaceRootSelections)
	if err != nil {
		return nil, err
	}
	plans := make([]ProjectionPlan, 0, len(prepared))
	for _, target := range prepared {
		plan := target.plan
		plan.Effects = target.effects()
		plans = append(plans, plan)
	}
	return plans, nil
}

// ReconcileConfigs applies a prepared before-to-after target transition.
// It guards every write against its captured preimage and compensates prior
// writes in reverse order only while their postimages remain unchanged.
func ReconcileConfigs(before, after []TargetRef, runtime domain.Runtime) (ReconciliationReceipt, error) {
	return ReconcileConfigsTransition(before, after, runtime, runtime)
}

// ReconcileConfigsTransition applies one before-to-after Route transition.
func ReconcileConfigsTransition(before, after []TargetRef, previous, runtime domain.Runtime) (ReconciliationReceipt, error) {
	return reconcileCodexConfigsTransition(before, after, previous, runtime, false)
}

// ReconcileConfigsAuthorizedTransition applies an explicit Route selection
// after the sidecar, provider block, scheduler, and catalogue establish the
// existing projection's ownership.
func ReconcileConfigsAuthorizedTransition(before, after []TargetRef, previous, runtime domain.Runtime) (ReconciliationReceipt, error) {
	return reconcileCodexConfigsTransition(before, after, previous, runtime, true)
}

func reconcileCodexConfigsTransition(before, after []TargetRef, previous, runtime domain.Runtime, replaceRootSelections bool) (ReconciliationReceipt, error) {
	prepared, err := prepareCodexReconciliationTransition(before, after, previous, runtime, replaceRootSelections)
	if err != nil {
		return ReconciliationReceipt{}, err
	}
	committed := make([]committedCodexArtifact, 0)
	for _, target := range prepared {
		for _, artifact := range target.artifacts {
			post, commitErr := commitCodexArtifact(artifact)
			if commitErr != nil {
				rollbackErr := rollbackCodexArtifacts(committed)
				if rollbackErr != nil {
					return ReconciliationReceipt{}, store.CompensationFailed(fmt.Errorf("commit Codex reconciliation %s: %w", artifact.path, commitErr), rollbackErr)
				}
				return ReconciliationReceipt{}, fmt.Errorf("commit Codex reconciliation %s: %w; all artifacts rolled back", artifact.path, commitErr)
			}
			committed = append(committed, committedCodexArtifact{prepared: artifact, post: post})
		}
	}
	return ReconciliationReceipt{committed: committed}, nil
}

func prepareCodexReconciliation(before, after []TargetRef, runtime domain.Runtime) ([]codexPreparedTarget, error) {
	return prepareCodexReconciliationTransition(before, after, runtime, runtime, false)
}

func prepareCodexReconciliationTransition(before, after []TargetRef, previous, runtime domain.Runtime, replaceRootSelections bool) ([]codexPreparedTarget, error) {
	targets, err := codexTargetUnion(before, after)
	if err != nil {
		return nil, err
	}
	transactionID := newCodexTransactionID()
	endpoint := ""
	needsEndpoint := false
	for _, target := range targets {
		if target.desired {
			needsEndpoint = true
			break
		}
	}
	if needsEndpoint {
		endpoint, err = codexEndpoint(runtime)
		if err != nil {
			return nil, err
		}
	}
	prepared := make([]codexPreparedTarget, 0, len(targets))
	for _, target := range targets {
		candidate, err := prepareCodexReconciliationTarget(target, previous, runtime, endpoint, transactionID, replaceRootSelections)
		if err != nil {
			return nil, fmt.Errorf("prepare Codex target %s: %w", target.ref.Path, err)
		}
		prepared = append(prepared, candidate)
	}
	return prepared, nil
}

func prepareCodexReconciliationTarget(target codexReconciliationTarget, previous, runtime domain.Runtime, endpoint, transactionID string, replaceRootSelections bool) (codexPreparedTarget, error) {
	configSnapshot, err := store.CaptureFileSnapshot(target.ref.Path)
	if err != nil {
		return codexPreparedTarget{}, err
	}
	if !configSnapshot.Exists && (!target.desired || !target.ref.CreateIfAbsent) {
		return codexPreparedTarget{}, fmt.Errorf("Codex config does not exist")
	}
	statePath := targetCodexStatePath(target.ref)
	stateSnapshot, err := store.CaptureFileSnapshot(statePath)
	if err != nil {
		return codexPreparedTarget{}, err
	}
	catalogSnapshot, err := store.CaptureFileSnapshot(codexCatalogPath(target.ref.Path))
	if err != nil {
		return codexPreparedTarget{}, err
	}
	if err := validateCodexCatalogPreflight(target, configSnapshot, stateSnapshot); err != nil {
		return codexPreparedTarget{}, err
	}
	if !target.desired {
		return prepareCodexRestoreTransition(target.ref, configSnapshot, stateSnapshot, catalogSnapshot, previous, replaceRootSelections)
	}
	block := codexManagedBlock(runtime, endpoint)
	base, state, err := codexUserConfigTransition(configSnapshot, stateSnapshot, previous, runtime.Model, replaceRootSelections)
	if err != nil {
		return codexPreparedTarget{}, err
	}
	// The hash AIGW recorded writing is read before the state is updated: it is
	// the only proof of which catalog bytes are AIGW's own, and therefore the only
	// safe authorization to remove the file.
	ownedCatalogHash := state.CatalogHash
	provider := codexRuntimeProvider(runtime)
	catalogModel := runtime.Model
	if provider != modelProviderAIGW {
		catalogModel = ""
	}
	catalog := codexCatalogProjection(target.ref, catalogModel, runtime.CanonicalModelID, base, state, catalogSnapshot)
	projection, err := projectCodex(base, block, runtime.Model, catalog.path, provider)
	if err != nil {
		return codexPreparedTarget{}, err
	}
	projected := []byte(projection)
	applyCodexCatalogState(&state, catalog)
	catalogDesired, err := codexCatalogDesiredSnapshot(catalog, catalogSnapshot, ownedCatalogHash)
	if err != nil {
		return codexPreparedTarget{}, err
	}
	state.ManagedBlockHash = hashText(block)
	state.ProjectedModel = runtime.Model
	if provider == modelProviderAIGW {
		state.ProjectedProvider = ""
	} else {
		state.ProjectedProvider = provider
	}
	state.ProjectedSchedulerHash = codexSchedulerHash(string(projected))
	state.ProjectionMode = ProjectionFullSelection
	state.WriterID = ProjectionWriterID
	stateData := encodeCodexState(state)
	converged := stateSnapshot.Exists && bytes.Equal(stateSnapshot.Data, stateData) && catalogSnapshot.Equal(catalogDesired)
	if converged && !bytes.Equal(configSnapshot.Data, projected) {
		current := string(configSnapshot.Data)
		converged = !replaceRootSelections && strings.Contains(current, codexBegin) &&
			strings.Contains(current, codexEnd) && sameCodexTOMLValues(configSnapshot.Data, projected)
	}
	if converged {
		projected = configSnapshot.Data
	}
	if !converged {
		state.TransactionID = transactionID
		stateData = encodeCodexState(state)
	}
	action := ProjectionActionUpdate
	if converged {
		action = ProjectionActionAlreadyConverged
	} else if !stateSnapshot.Exists {
		action = ProjectionActionInitialProject
	}
	return codexPreparedTarget{
		plan:      ProjectionPlan{Target: target.ref.Path, Action: action, SurfaceID: target.ref.SurfaceID},
		artifacts: codexArtifactsForDesiredState(target.ref, configSnapshot, projected, stateSnapshot, stateData, catalogSnapshot, catalogDesired),
	}, nil
}

func sameCodexTOMLValues(current, projected []byte) bool {
	var left, right map[string]any
	if toml.Unmarshal(current, &left) != nil || toml.Unmarshal(projected, &right) != nil || !reflect.DeepEqual(left, right) {
		return false
	}
	for _, key := range []string{"model_provider", "model", "model_catalog_json"} {
		wanted, err := codexSelectionLine(string(projected), key)
		if err != nil {
			return false
		}
		if !strings.HasSuffix(strings.TrimSpace(wanted), "# managed by AIGW") {
			continue
		}
		actual, err := codexSelectionLine(string(current), key)
		value, ok := right[key].(string)
		if err != nil || !ok || !isManagedSelection(actual, key, value) {
			return false
		}
	}
	return true
}

func prepareCodexRestore(target TargetRef, configSnapshot, stateSnapshot, catalogSnapshot store.FileSnapshot) (codexPreparedTarget, error) {
	return prepareCodexRestoreTransition(target, configSnapshot, stateSnapshot, catalogSnapshot, domain.Runtime{}, false)
}

func prepareCodexRestoreTransition(target TargetRef, configSnapshot, stateSnapshot, catalogSnapshot store.FileSnapshot, previous domain.Runtime, replaceRootSelections bool) (codexPreparedTarget, error) {
	if !stateSnapshot.Exists {
		return codexPreparedTarget{plan: ProjectionPlan{Target: target.Path, Action: ProjectionActionAlreadyRestored, SurfaceID: target.SurfaceID}}, nil
	}
	state, err := codexStateForTarget(stateSnapshot)
	if err != nil {
		return codexPreparedTarget{}, err
	}
	restored, err := removeCodexProjectionTransition(string(configSnapshot.Data), state, previous, "", replaceRootSelections)
	if err != nil {
		return codexPreparedTarget{}, err
	}
	catalogDesired, err := codexCatalogDesiredSnapshot(codexCatalogPlan{}, catalogSnapshot, state.CatalogHash)
	if err != nil {
		return codexPreparedTarget{}, err
	}
	artifacts := codexArtifactsForDesiredState(target, configSnapshot, []byte(restored), stateSnapshot, nil, catalogSnapshot, catalogDesired)
	if state.ConfigAbsentBeforeProjection && strings.TrimSpace(restored) == "" {
		for index := range artifacts {
			if artifacts[index].path == target.Path {
				artifacts[index].desired = store.FileSnapshot{}
				break
			}
		}
	}
	return codexPreparedTarget{
		plan:      ProjectionPlan{Target: target.Path, Action: ProjectionActionRestoreExternal, SurfaceID: target.SurfaceID},
		artifacts: artifacts,
	}, nil
}

// codexArtifactsForDesiredState orders one target's writes along their
// dependency direction. A configuration that names a catalog file must never be
// readable before that file exists, because the client refuses to start when the
// reference cannot be resolved; a withdrawal therefore runs the other way and
// deletes the file only after nothing refers to it.
func codexArtifactsForDesiredState(target TargetRef, configBefore store.FileSnapshot, configData []byte, stateBefore store.FileSnapshot, stateData []byte, catalogBefore, catalogDesired store.FileSnapshot) []codexPreparedArtifact {
	artifacts := make([]codexPreparedArtifact, 0, 3)
	catalog := codexPreparedArtifact{path: codexCatalogPath(target.Path), before: catalogBefore, desired: catalogDesired, exactMode: true}
	catalogChanged := !catalogBefore.Equal(catalogDesired)
	if catalogChanged && catalogDesired.Exists {
		artifacts = append(artifacts, catalog)
	}
	configMode := configBefore.Mode
	if !configBefore.Exists {
		configMode = 0o600
	}
	configDesired := store.NewFileSnapshot(configData, configMode)
	if !configBefore.Equal(configDesired) {
		artifacts = append(artifacts, codexPreparedArtifact{path: target.Path, before: configBefore, desired: configDesired})
	}
	stateDesired := store.FileSnapshot{}
	if stateData != nil {
		stateMode := os.FileMode(0o600)
		if stateBefore.Exists {
			stateMode = stateBefore.Mode
		}
		stateDesired = store.NewFileSnapshot(stateData, stateMode)
	}
	if !stateBefore.Equal(stateDesired) {
		artifacts = append(artifacts, codexPreparedArtifact{path: targetCodexStatePath(target), before: stateBefore, desired: stateDesired})
	}
	if catalogChanged && !catalogDesired.Exists {
		artifacts = append(artifacts, catalog)
	}
	return artifacts
}

func commitCodexArtifact(artifact codexPreparedArtifact) (store.FileSnapshot, error) {
	if artifact.desired.Exists {
		if artifact.exactMode {
			return writeFileAtomicExactModeIfUnchanged(artifact.path, artifact.before, artifact.desired.Data, artifact.desired.Mode)
		}
		return writeFileAtomicIfUnchanged(artifact.path, artifact.before, artifact.desired.Data, artifact.desired.Mode)
	}
	return removeFileIfUnchanged(artifact.path, artifact.before)
}

func rollbackCodexArtifacts(committed []committedCodexArtifact) error {
	var failures []error
	for _, artifact := range slices.Backward(committed) {
		if err := restoreFileAtomicIfPostimage(artifact.prepared.path, artifact.prepared.before, artifact.post); err != nil {
			failures = append(failures, fmt.Errorf("restore %s: %w", artifact.prepared.path, err))
		}
	}
	return errors.Join(failures...)
}

func hashBytes(data []byte) string {
	return hashText(string(data))
}

func codexStateForTarget(snapshot store.FileSnapshot) (codexState, error) {
	if !snapshot.Exists {
		return codexState{}, nil
	}
	var state codexState
	if err := json.Unmarshal(snapshot.Data, &state); err != nil {
		return codexState{}, fmt.Errorf("parse Codex adapter state: %w", err)
	}
	if err := validateCodexStateAttribution(state); err != nil {
		return codexState{}, err
	}
	return state, nil
}

func encodeCodexState(state codexState) []byte {
	data, _ := json.MarshalIndent(state, "", "  ")
	return append(data, '\n')
}

func validateCodexStateAttribution(state codexState) error {
	if state.ProjectionMode == "" || state.WriterID == "" || state.TransactionID == "" {
		return fmt.Errorf("Codex sidecar attribution is incomplete")
	}
	if state.ProjectionMode != ProjectionFullSelection {
		return fmt.Errorf("Codex sidecar has unsupported projection mode %q", state.ProjectionMode)
	}
	if state.WriterID != ProjectionWriterID {
		return fmt.Errorf("Codex sidecar is owned by foreign writer %q", state.WriterID)
	}
	return nil
}

func newCodexTransactionID() string {
	bytes := make([]byte, 16)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
