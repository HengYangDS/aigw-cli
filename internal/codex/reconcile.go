package codex

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/surface"
	"aigw-cli/internal/transaction"
)

const (
	ProjectionFullSelection = "full-selection"
	ProjectionWriterID      = "aigw-cli"
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
	statePath      string
}

// ReconciliationReceipt describes a completed in-process projection
// transaction without exposing configuration bodies or credentials.
type ReconciliationReceipt struct {
	TransactionID string           `json:"transaction_id"`
	Plans         []ProjectionPlan `json:"plans"`
}

// ProjectionIdentity is the bounded sidecar identity used by surface
// routing. It intentionally excludes configuration bodies, paths, endpoints,
// and credential material.
type ProjectionIdentity struct {
	Present          bool
	ProjectionMode   string
	AttributionState string
}

type codexReconciliationTarget struct {
	ref     TargetRef
	desired bool
}

type codexPreparedArtifact struct {
	path    string
	before  transaction.FileSnapshot
	desired transaction.FileSnapshot
	// exactMode writes the desired mode instead of inheriting the one already on
	// disk. It is set for artifacts whose permissions AIGW owns.
	exactMode bool
}

type codexPreparedTarget struct {
	plan      ProjectionPlan
	artifacts []codexPreparedArtifact
}

type committedCodexArtifact struct {
	prepared codexPreparedArtifact
	post     transaction.FileSnapshot
}

// These seams let the reconciliation tests inject deterministic write failure
// and concurrent-edit scenarios. Production calls the transaction package.
var writeFileAtomicIfUnchanged = transaction.WriteFileAtomicIfUnchanged
var writeFileAtomicExactModeIfUnchanged = transaction.WriteFileAtomicExactModeIfUnchanged
var removeFileIfUnchanged = transaction.RemoveFileIfUnchanged
var restoreFileAtomicIfPostimage = transaction.RestoreFileAtomicIfPostimage

// ReadProjectionIdentity reads only the sidecar attribution required to
// select a safe surface mode. It never changes configuration or sessions.
func ReadProjectionIdentity(path string) (ProjectionIdentity, error) {
	data, err := os.ReadFile(codexStatePath(path))
	if os.IsNotExist(err) {
		return ProjectionIdentity{}, nil
	}
	if err != nil {
		return ProjectionIdentity{}, fmt.Errorf("read Codex adapter state: %w", err)
	}
	var state codexState
	if err := json.Unmarshal(data, &state); err != nil {
		return ProjectionIdentity{}, fmt.Errorf("parse Codex adapter state: %w", err)
	}
	if err := validateCodexStateAttribution(state); err != nil {
		return ProjectionIdentity{}, err
	}
	return ProjectionIdentity{
		Present:          true,
		ProjectionMode:   state.ProjectionMode,
		AttributionState: "recognized",
	}, nil
}

// PlanReconciliation prepares a before-to-after target transition
// without writing configuration, sidecars, credentials, or sessions.
func PlanReconciliation(before, after []TargetRef, runtime configuration.Runtime) ([]ProjectionPlan, error) {
	prepared, _, err := prepareCodexReconciliation(before, after, runtime)
	if err != nil {
		return nil, err
	}
	plans := make([]ProjectionPlan, 0, len(prepared))
	for _, target := range prepared {
		plans = append(plans, target.plan)
	}
	return plans, nil
}

// ReconcileConfigs applies a prepared before-to-after target transition.
// It guards every write against its captured preimage and compensates prior
// writes in reverse order only while their postimages remain unchanged.
func ReconcileConfigs(before, after []TargetRef, runtime configuration.Runtime) (ReconciliationReceipt, error) {
	prepared, transactionID, err := prepareCodexReconciliation(before, after, runtime)
	if err != nil {
		return ReconciliationReceipt{}, err
	}
	receipt := ReconciliationReceipt{TransactionID: transactionID, Plans: make([]ProjectionPlan, 0, len(prepared))}
	for _, target := range prepared {
		receipt.Plans = append(receipt.Plans, target.plan)
	}
	committed := make([]committedCodexArtifact, 0)
	for _, target := range prepared {
		for _, artifact := range target.artifacts {
			post, commitErr := commitCodexArtifact(artifact)
			if commitErr != nil {
				rollbackErr := rollbackCodexArtifacts(committed)
				if rollbackErr != nil {
					return ReconciliationReceipt{}, fmt.Errorf("commit Codex reconciliation %s: %w; rollback also failed: %v", artifact.path, commitErr, rollbackErr)
				}
				return ReconciliationReceipt{}, fmt.Errorf("commit Codex reconciliation %s: %w; all artifacts rolled back", artifact.path, commitErr)
			}
			committed = append(committed, committedCodexArtifact{prepared: artifact, post: post})
		}
	}
	return receipt, nil
}

func prepareCodexReconciliation(before, after []TargetRef, runtime configuration.Runtime) ([]codexPreparedTarget, string, error) {
	targets, err := codexTargetUnion(before, after)
	if err != nil {
		return nil, "", err
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
			return nil, "", err
		}
	}
	prepared := make([]codexPreparedTarget, 0, len(targets))
	for _, target := range targets {
		candidate, err := prepareCodexReconciliationTarget(target, runtime, endpoint, transactionID)
		if err != nil {
			return nil, "", fmt.Errorf("prepare Codex target %s: %w", target.ref.Path, err)
		}
		prepared = append(prepared, candidate)
	}
	return prepared, transactionID, nil
}

func prepareCodexReconciliationTarget(target codexReconciliationTarget, runtime configuration.Runtime, endpoint, transactionID string) (codexPreparedTarget, error) {
	configSnapshot, err := transaction.CaptureFileSnapshot(target.ref.Path)
	if err != nil {
		return codexPreparedTarget{}, err
	}
	if !configSnapshot.Exists {
		return codexPreparedTarget{}, fmt.Errorf("Codex config does not exist")
	}
	statePath := targetCodexStatePath(target.ref)
	stateSnapshot, err := transaction.CaptureFileSnapshot(statePath)
	if err != nil {
		return codexPreparedTarget{}, err
	}
	catalogSnapshot, err := transaction.CaptureFileSnapshot(targetCodexCatalogPath(target.ref))
	if err != nil {
		return codexPreparedTarget{}, err
	}
	if !target.desired {
		return prepareCodexRestore(target.ref, configSnapshot, stateSnapshot, catalogSnapshot)
	}
	block := codexManagedBlock(runtime, endpoint)
	return prepareCodexFullSelection(target.ref, runtime, block, configSnapshot, stateSnapshot, catalogSnapshot, transactionID)
}

func prepareCodexFullSelection(target TargetRef, runtime configuration.Runtime, block string, configSnapshot, stateSnapshot, catalogSnapshot transaction.FileSnapshot, transactionID string) (codexPreparedTarget, error) {
	state, err := codexStateForTarget(stateSnapshot)
	if err != nil {
		return codexPreparedTarget{}, err
	}
	base, projectedState, err := codexUserConfig(configSnapshot, stateSnapshot, runtime, block)
	if err != nil {
		return codexPreparedTarget{}, err
	}
	state = projectedState
	// The hash AIGW recorded writing is read before the state is updated: it is
	// the only proof of which catalog bytes are AIGW's own, and therefore the only
	// safe authorization to remove the file.
	ownedCatalogHash := state.CatalogHash
	provider := codexRuntimeProvider(runtime)
	catalogModel := runtime.Model
	if provider != configuration.ModelProviderAIGW {
		catalogModel = ""
	}
	catalog := codexCatalogProjection(target, catalogModel, base, state, catalogSnapshot)
	projection, err := projectCodexForProvider(base, block, runtime.Model, catalog.path, provider)
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
	if provider == configuration.ModelProviderAIGW {
		state.ProjectedProvider = ""
	} else {
		state.ProjectedProvider = provider
	}
	state.ProjectedSchedulerHash = codexSchedulerHash(string(projected))
	state.ProjectionMode = ProjectionFullSelection
	state.WriterID = ProjectionWriterID
	stateData := encodeCodexState(state)
	catalogConverged := sameCodexSnapshot(catalogSnapshot, catalogDesired)
	if !bytes.Equal(configSnapshot.Data, projected) || !bytes.Equal(stateSnapshot.Data, stateData) || !stateSnapshot.Exists || !catalogConverged {
		state.TransactionID = transactionID
		stateData = encodeCodexState(state)
	}
	action := "update"
	if bytes.Equal(configSnapshot.Data, projected) && stateSnapshot.Exists && bytes.Equal(stateSnapshot.Data, stateData) && catalogConverged {
		action = "already-converged"
	} else if !stateSnapshot.Exists {
		action = "initial-project"
	} else if isExactTruncatedCodexProjection(string(configSnapshot.Data), stateSnapshot.Data, runtime, block) {
		action = "repair-truncated"
	}
	return codexPreparedTarget{
		plan:      ProjectionPlan{Target: target.Path, Action: action},
		artifacts: codexArtifactsForDesiredState(target, configSnapshot, projected, stateSnapshot, stateData, catalogSnapshot, catalogDesired),
	}, nil
}

func prepareCodexRestore(target TargetRef, configSnapshot, stateSnapshot, catalogSnapshot transaction.FileSnapshot) (codexPreparedTarget, error) {
	if !stateSnapshot.Exists {
		return codexPreparedTarget{plan: ProjectionPlan{Target: target.Path, Action: "already-restored"}}, nil
	}
	state, err := codexStateForTarget(stateSnapshot)
	if err != nil {
		return codexPreparedTarget{}, err
	}
	restored, err := removeCodexProjection(string(configSnapshot.Data), state)
	if err != nil {
		return codexPreparedTarget{}, err
	}
	catalogDesired, err := codexCatalogDesiredSnapshot(codexCatalogPlan{}, catalogSnapshot, state.CatalogHash)
	if err != nil {
		return codexPreparedTarget{}, err
	}
	return codexPreparedTarget{
		plan:      ProjectionPlan{Target: target.Path, Action: "restore-external"},
		artifacts: codexArtifactsForDesiredState(target, configSnapshot, []byte(restored), stateSnapshot, nil, catalogSnapshot, catalogDesired),
	}, nil
}

// codexArtifactsForDesiredState orders one target's writes along their
// dependency direction. A configuration that names a catalog file must never be
// readable before that file exists, because the client refuses to start when the
// reference cannot be resolved; a withdrawal therefore runs the other way and
// deletes the file only after nothing refers to it.
func codexArtifactsForDesiredState(target TargetRef, configBefore transaction.FileSnapshot, configData []byte, stateBefore transaction.FileSnapshot, stateData []byte, catalogBefore, catalogDesired transaction.FileSnapshot) []codexPreparedArtifact {
	artifacts := make([]codexPreparedArtifact, 0, 3)
	catalog := codexPreparedArtifact{path: targetCodexCatalogPath(target), before: catalogBefore, desired: catalogDesired, exactMode: true}
	catalogChanged := !sameCodexSnapshot(catalogBefore, catalogDesired)
	if catalogChanged && catalogDesired.Exists {
		artifacts = append(artifacts, catalog)
	}
	configDesired := desiredCodexSnapshot(configData, configBefore.Mode)
	if !sameCodexSnapshot(configBefore, configDesired) {
		artifacts = append(artifacts, codexPreparedArtifact{path: target.Path, before: configBefore, desired: configDesired})
	}
	stateDesired := transaction.FileSnapshot{}
	if stateData != nil {
		stateMode := os.FileMode(0o600)
		if stateBefore.Exists {
			stateMode = stateBefore.Mode
		}
		stateDesired = desiredCodexSnapshot(stateData, stateMode)
	}
	if !sameCodexSnapshot(stateBefore, stateDesired) {
		artifacts = append(artifacts, codexPreparedArtifact{path: targetCodexStatePath(target), before: stateBefore, desired: stateDesired})
	}
	if catalogChanged && !catalogDesired.Exists {
		artifacts = append(artifacts, catalog)
	}
	return artifacts
}

func commitCodexArtifact(artifact codexPreparedArtifact) (transaction.FileSnapshot, error) {
	if artifact.desired.Exists {
		if artifact.exactMode {
			return writeFileAtomicExactModeIfUnchanged(artifact.path, artifact.before, artifact.desired.Data, artifact.desired.Mode)
		}
		return writeFileAtomicIfUnchanged(artifact.path, artifact.before, artifact.desired.Data, artifact.desired.Mode)
	}
	return removeFileIfUnchanged(artifact.path, artifact.before)
}

func rollbackCodexArtifacts(committed []committedCodexArtifact) error {
	failures := make([]string, 0)
	for index := len(committed) - 1; index >= 0; index-- {
		artifact := committed[index]
		if err := restoreFileAtomicIfPostimage(artifact.prepared.path, artifact.prepared.before, artifact.post); err != nil {
			failures = append(failures, fmt.Sprintf("restore %s: %v", artifact.prepared.path, err))
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("%s", strings.Join(failures, "; "))
	}
	return nil
}

func desiredCodexSnapshot(data []byte, mode os.FileMode) transaction.FileSnapshot {
	return transaction.FileSnapshot{Exists: true, Data: append([]byte(nil), data...), SHA256: hashBytes(data), Mode: mode}
}

func sameCodexSnapshot(left, right transaction.FileSnapshot) bool {
	return left.Exists == right.Exists && left.SHA256 == right.SHA256 && left.Mode == right.Mode && bytes.Equal(left.Data, right.Data)
}

func hashBytes(data []byte) string {
	return hashText(string(data))
}

func codexStateForTarget(snapshot transaction.FileSnapshot) (codexState, error) {
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

func codexTargetUnion(before, after []TargetRef) ([]codexReconciliationTarget, error) {
	normalizedBefore, err := normalizeCodexTargets(before)
	if err != nil {
		return nil, err
	}
	normalizedAfter, err := normalizeCodexTargets(after)
	if err != nil {
		return nil, err
	}
	byPath := make(map[string]codexReconciliationTarget, len(normalizedBefore)+len(normalizedAfter))
	for _, target := range normalizedBefore {
		byPath[target.Path] = codexReconciliationTarget{ref: target}
	}
	for _, target := range normalizedAfter {
		if err := validateDesiredCodexTarget(target); err != nil {
			return nil, err
		}
		byPath[target.Path] = codexReconciliationTarget{ref: target, desired: true}
	}
	union := make([]codexReconciliationTarget, 0, len(byPath))
	for _, target := range byPath {
		union = append(union, target)
	}
	sort.Slice(union, func(left, right int) bool { return union[left].ref.Path < union[right].ref.Path })
	return union, nil
}

func normalizeCodexTargets(values []TargetRef) ([]TargetRef, error) {
	normalized := make([]TargetRef, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, target := range values {
		if target.Path == "" || target.SurfaceID == "" || target.Authority == "" || target.ProjectionMode == "" {
			return nil, fmt.Errorf("Codex target requires surface_id, authority, projection_mode, and path")
		}
		sourcePath, err := absoluteCodexTargetPath(target.Path)
		if err != nil {
			return nil, err
		}
		path, err := canonicalCodexTargetPath(sourcePath)
		if err != nil {
			return nil, err
		}
		if _, duplicate := seen[path]; duplicate {
			return nil, fmt.Errorf("Codex config target %s is duplicated", path)
		}
		seen[path] = struct{}{}
		target.Path = path
		target.statePath = preferredCodexStatePath(sourcePath, path)
		normalized = append(normalized, target)
	}
	sort.Slice(normalized, func(left, right int) bool { return normalized[left].Path < normalized[right].Path })
	return normalized, nil
}

func canonicalCodexTargetPath(path string) (string, error) {
	absolute := filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(absolute)
	if err == nil {
		return filepath.Clean(resolved), nil
	}
	if os.IsNotExist(err) {
		return absolute, nil
	}
	return "", fmt.Errorf("resolve Codex target symlinks %s: %w", path, err)
}

func absoluteCodexTargetPath(path string) (string, error) {
	absolute, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("resolve Codex target %s: %w", path, err)
	}
	return absolute, nil
}

func preferredCodexStatePath(sourcePath, canonicalPath string) string {
	canonicalStatePath := codexStatePath(canonicalPath)
	if sourcePath == canonicalPath {
		return canonicalStatePath
	}
	if info, err := os.Lstat(canonicalStatePath); err == nil && !info.IsDir() {
		return canonicalStatePath
	}
	sourceStatePath := codexStatePath(sourcePath)
	if info, err := os.Lstat(sourceStatePath); err == nil && !info.IsDir() {
		return sourceStatePath
	}
	return canonicalStatePath
}

func targetCodexStatePath(target TargetRef) string {
	if target.statePath != "" {
		return target.statePath
	}
	return codexStatePath(target.Path)
}

func validateDesiredCodexTarget(target TargetRef) error {
	surfaceID := surface.ID(target.SurfaceID)
	authority := surface.Authority(target.Authority)
	switch {
	case surfaceID.IsCodexHome() && surfaceID.HasAuthority(authority) && target.ProjectionMode == ProjectionFullSelection:
		return nil
	default:
		return fmt.Errorf("Codex target %s cannot use authority %s with projection mode %s", target.SurfaceID, target.Authority, target.ProjectionMode)
	}
}

func codexHomeTargets(paths []string) []TargetRef {
	targets := make([]TargetRef, 0, len(paths))
	for _, path := range paths {
		targets = append(targets, TargetRef{
			SurfaceID:      string(surface.CodexHomeDefault),
			Authority:      string(surface.AuthorityAIGW),
			ProjectionMode: ProjectionFullSelection,
			Path:           path,
		})
	}
	return targets
}

func newCodexTransactionID() string {
	bytes := make([]byte, 16)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
