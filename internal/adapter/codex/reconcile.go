package codex

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"

	"github.com/pelletier/go-toml/v2"
)

const (
	// ProjectionFullSelection identifies a projection that owns both provider and model selection.
	ProjectionFullSelection = "full-selection"
	// ProjectionWriterID identifies AIGW as the writer of its bounded Codex projection.
	ProjectionWriterID = "aigw-cli"
)

// TargetRef identifies a persistent configuration file together with the
// host surface that it belongs to.
// Executable is the client this target's configuration is read by. It is the
// only source of client-specific facts, such as the bundled model catalog, so a
// target without one is projected without them rather than against a guess.
// Normalization resolves Path's symbolic links and keeps the path as
// configured, and the resolved folder that holds it, for reports.
type TargetRef struct {
	SurfaceID        string
	Path             string
	Executable       string
	CreateIfAbsent   bool
	statePath        string
	configured       string
	configuredFolder string
}

// reportName names path, the target's file or a file beside it, as the
// operator configured the target: the configured file itself, and a file in
// the folder that the configured path resolves to by that folder's
// configured path and the file's own name. Any other path, such as one beside
// a configured file that is itself a link, stays resolved.
func (target TargetRef) reportName(path string) string {
	switch {
	case target.configured == "":
		return path
	case path == target.Path:
		return target.configured
	case filepath.Dir(path) == target.configuredFolder:
		return filepath.Join(filepath.Dir(target.configured), filepath.Base(path))
	default:
		return path
	}
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
	suffixes := [...]string{"", adapter.StateSuffix, ".aigw-model-catalog.json"}
	var snapshots [len(suffixes)]platform.FileSnapshot
	for index, suffix := range suffixes {
		snapshot, err := platform.CaptureFileSnapshot(source + suffix)
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
		if err := platform.WriteFileAtomicExactMode(target+suffix, snapshot.Data, snapshot.Mode); err != nil {
			return err
		}
	}
	return nil
}

// Rollback restores the exact preimages captured by this reconciliation while
// each artifact still equals the postimage written by this transaction.
func (r ReconciliationReceipt) Rollback() error {
	return r.Restore().Err()
}

// Restore rolls the reconciliation back and reports each file that it wrote.
func (r ReconciliationReceipt) Restore() platform.Restoration {
	return restoreCodexArtifacts(r.committed)
}

type codexReconciliationTarget struct {
	ref     TargetRef
	desired bool
}

type codexPreparedArtifact struct {
	path    string
	before  platform.FileSnapshot
	desired platform.FileSnapshot
	// exactMode writes the desired mode instead of inheriting the one already on
	// disk. It is set for artifacts whose permissions AIGW owns.
	exactMode bool
	// name is path as a report names it, as the operator configured it.
	name string
}

// reported is the artifact's name in reports, its path without one.
func (artifact codexPreparedArtifact) reported() string {
	if artifact.name != "" {
		return artifact.name
	}
	return artifact.path
}

// namedFailure names the artifact's file in err, a failure of its write or
// removal that this commit created, as the operator configured it.
func (artifact codexPreparedArtifact) namedFailure(err error) error {
	if failed, atFile := errors.AsType[*platform.FileError](err); atFile && failed.Path == artifact.path {
		failed.Path = artifact.reported()
	}
	return err
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
	post     platform.FileSnapshot
}

// These seams let the reconciliation tests inject deterministic write failure
// and concurrent-edit scenarios. Production calls the store's file transactions.
var writeFileAtomicIfUnchanged = platform.WriteFileAtomicIfUnchanged
var writeFileAtomicExactModeIfUnchanged = platform.WriteFileAtomicExactModeIfUnchanged
var removeFileIfUnchanged = platform.RemoveFileIfUnchanged
var restoreFileAtomicIfPostimage = platform.RestoreFileAtomicIfPostimage

// planCodexReconciliationTransition prepares a before-to-after target
// transition without writing configuration, sidecars, credentials, or
// sessions. The previous runtime attributes legacy sidecars that predate
// projected model identity without weakening semantic drift checks. An
// explicit Route selection sets replaceRootSelections: it may replace root
// provider and model values after all other owned state still matches its
// sidecar.
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
// previous is the runtime before the transition. An explicit Route selection
// sets replaceRootSelections, and applies only after the sidecar, provider
// block, scheduler, and catalog establish the existing projection's ownership.
func ReconcileConfigs(before, after []TargetRef, previous, runtime domain.Runtime, replaceRootSelections bool) (ReconciliationReceipt, error) {
	prepared, err := prepareCodexReconciliationTransition(before, after, previous, runtime, replaceRootSelections)
	if err != nil {
		return ReconciliationReceipt{}, err
	}
	committed := make([]committedCodexArtifact, 0)
	for _, target := range prepared {
		for _, artifact := range target.artifacts {
			post, commitErr := commitCodexArtifact(artifact)
			if commitErr != nil {
				commitErr = artifact.namedFailure(commitErr)
				restored := restoreCodexArtifacts(committed)
				if restored.Err() != nil {
					return ReconciliationReceipt{}, platform.FailedWrite(fmt.Errorf("commit Codex reconciliation %s: %w", artifact.reported(), commitErr), restored)
				}
				return ReconciliationReceipt{}, platform.FailedWrite(fmt.Errorf("commit Codex reconciliation %s: %w; all artifacts rolled back", artifact.reported(), commitErr), restored)
			}
			committed = append(committed, committedCodexArtifact{prepared: artifact, post: post})
		}
	}
	return ReconciliationReceipt{committed: committed}, nil
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
			return nil, &adapter.TargetError{Desired: target.desired, Err: fmt.Errorf("prepare Codex target %s: %w", target.ref.Path, err)}
		}
		prepared = append(prepared, candidate)
	}
	return prepared, nil
}

func prepareCodexReconciliationTarget(target codexReconciliationTarget, previous, runtime domain.Runtime, endpoint, transactionID string, replaceRootSelections bool) (codexPreparedTarget, error) {
	configSnapshot, err := platform.CaptureFileSnapshot(target.ref.Path)
	if err != nil {
		return codexPreparedTarget{}, err
	}
	statePath := targetCodexStatePath(target.ref)
	stateSnapshot, err := platform.CaptureFileSnapshot(statePath)
	if err != nil {
		return codexPreparedTarget{}, err
	}
	if !configSnapshot.Exists && (!target.desired || !target.ref.CreateIfAbsent) {
		return missingCodexTarget(target, stateSnapshot.Exists, filepath.Base(statePath))
	}
	catalogSnapshot, err := platform.CaptureFileSnapshot(codexCatalogPath(target.ref.Path))
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
	base, state, err := codexUserConfigTransition(configSnapshot, stateSnapshot, previous, runtime.UpstreamModel, replaceRootSelections)
	if err != nil {
		return codexPreparedTarget{}, err
	}
	// The hash AIGW recorded writing is read before the state is updated: it is
	// the only proof of which catalog bytes are AIGW's own, and therefore the only
	// safe authorization to remove the file.
	ownedCatalogHash := state.CatalogHash
	provider := codexRuntimeProvider(runtime)
	catalogModel := runtime.UpstreamModel
	if provider != modelProviderAIGW {
		catalogModel = ""
	}
	catalog := codexCatalogProjection(target.ref, catalogModel, runtime.FamilyID, base, state, catalogSnapshot)
	projection, err := projectCodex(base, block, runtime.UpstreamModel, catalog.path, provider)
	if err != nil {
		return codexPreparedTarget{}, err
	}
	projected := []byte(projection)
	applyCodexCatalogState(&state, catalog)
	catalogDesired, err := codexCatalogDesiredSnapshot(catalog, catalogSnapshot, ownedCatalogHash)
	if err != nil {
		return codexPreparedTarget{}, err
	}
	state.ManagedBlockHash = platform.SHA256([]byte(block))
	state.ProjectedModel = runtime.UpstreamModel
	if provider == modelProviderAIGW {
		state.ProjectedProvider = ""
	} else {
		state.ProjectedProvider = provider
	}
	state.ProjectedSchedulerHash = codexSchedulerHash(string(projected))
	state.ProjectionMode = ProjectionFullSelection
	state.WriterID = ProjectionWriterID
	stateData, _ := adapter.EncodeJSON(state)
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
		stateData, _ = adapter.EncodeJSON(state)
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

// missingCodexTarget plans a configured Codex file that does not exist. A file
// that the configuration keeps is refused, because AIGW does not create a
// Codex home. A file that the change drops is absent, with nothing to
// withdraw, unless AIGW's record of it remains, which is inspected first.
func missingCodexTarget(target codexReconciliationTarget, recorded bool, record string) (codexPreparedTarget, error) {
	switch {
	case target.desired:
		return codexPreparedTarget{}, missingTargetError{}
	case recorded:
		return codexPreparedTarget{}, recordedMissingTargetError{record: record}
	}
	return codexPreparedTarget{plan: ProjectionPlan{Target: target.ref.Path, Action: ProjectionActionAbsent, SurfaceID: target.ref.SurfaceID}}, nil
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

func prepareCodexRestoreTransition(target TargetRef, configSnapshot, stateSnapshot, catalogSnapshot platform.FileSnapshot, previous domain.Runtime, replaceRootSelections bool) (codexPreparedTarget, error) {
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
				artifacts[index].desired = platform.FileSnapshot{}
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
func codexArtifactsForDesiredState(target TargetRef, configBefore platform.FileSnapshot, configData []byte, stateBefore platform.FileSnapshot, stateData []byte, catalogBefore, catalogDesired platform.FileSnapshot) []codexPreparedArtifact {
	artifacts := make([]codexPreparedArtifact, 0, 3)
	catalog := codexPreparedArtifact{path: codexCatalogPath(target.Path), before: catalogBefore, desired: catalogDesired, exactMode: true, name: target.reportName(codexCatalogPath(target.Path))}
	catalogChanged := !catalogBefore.Equal(catalogDesired)
	if catalogChanged && catalogDesired.Exists {
		artifacts = append(artifacts, catalog)
	}
	configMode := configBefore.Mode
	if !configBefore.Exists {
		configMode = 0o600
	}
	configDesired := platform.NewFileSnapshot(configData, configMode)
	if !configBefore.Equal(configDesired) {
		artifacts = append(artifacts, codexPreparedArtifact{path: target.Path, before: configBefore, desired: configDesired, name: target.reportName(target.Path)})
	}
	stateDesired := platform.FileSnapshot{}
	if stateData != nil {
		stateMode := os.FileMode(0o600)
		if stateBefore.Exists {
			stateMode = stateBefore.Mode
		}
		stateDesired = platform.NewFileSnapshot(stateData, stateMode)
	}
	if !stateBefore.Equal(stateDesired) {
		statePath := targetCodexStatePath(target)
		artifacts = append(artifacts, codexPreparedArtifact{path: statePath, before: stateBefore, desired: stateDesired, name: target.reportName(statePath)})
	}
	if catalogChanged && !catalogDesired.Exists {
		artifacts = append(artifacts, catalog)
	}
	return artifacts
}

func commitCodexArtifact(artifact codexPreparedArtifact) (platform.FileSnapshot, error) {
	if artifact.desired.Exists {
		if artifact.exactMode {
			return writeFileAtomicExactModeIfUnchanged(artifact.path, artifact.before, artifact.desired.Data, artifact.desired.Mode)
		}
		return writeFileAtomicIfUnchanged(artifact.path, artifact.before, artifact.desired.Data, artifact.desired.Mode)
	}
	return removeFileIfUnchanged(artifact.path, artifact.before)
}

// restoreCodexArtifacts restores the committed artifacts in reverse order and
// reports each.
func restoreCodexArtifacts(committed []committedCodexArtifact) platform.Restoration {
	restored := make(platform.Restoration, 0, len(committed))
	for _, artifact := range slices.Backward(committed) {
		restored = append(restored, platform.FileOutcome{
			Path: artifact.prepared.reported(),
			Err:  restoreFileAtomicIfPostimage(artifact.prepared.path, artifact.prepared.before, artifact.post),
		})
	}
	return restored
}

func codexStateForTarget(snapshot platform.FileSnapshot) (codexState, error) {
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
