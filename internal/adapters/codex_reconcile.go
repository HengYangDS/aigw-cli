package adapters

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/transaction"
)

const (
	CodexProjectionFullSelection                 = "full-selection"
	CodexProjectionNamespacedFallback            = "namespaced-fallback"
	CodexProjectionStaleAirFullSelectionRecovery = "recover-stale-full-selection"
	CodexProjectionWriterID                      = "aigw-cli"

	codexFallbackBegin = "# >>> AIGW fallback provider >>>"
	codexFallbackEnd   = "# <<< AIGW fallback provider <<<"
)

// CodexTargetRef identifies a persistent configuration file together with the
// host surface and ownership mode that authorizes AIGW to change it.
type CodexTargetRef struct {
	SurfaceID      string
	Authority      string
	ProjectionMode string
	Path           string
	statePath      string
}

// CodexReconciliationReceipt describes a completed in-process projection
// transaction without exposing configuration bodies or credentials.
type CodexReconciliationReceipt struct {
	TransactionID string                `json:"transaction_id"`
	Plans         []CodexProjectionPlan `json:"plans"`
}

// CodexProjectionIdentity is the bounded sidecar identity used by surface
// routing. It intentionally excludes configuration bodies, paths, endpoints,
// and credential material.
type CodexProjectionIdentity struct {
	Present          bool
	ProjectionMode   string
	AttributionState string
}

type codexReconciliationTarget struct {
	ref     CodexTargetRef
	desired bool
}

type codexPreparedArtifact struct {
	path    string
	before  transaction.FileSnapshot
	desired transaction.FileSnapshot
}

type codexPreparedTarget struct {
	plan      CodexProjectionPlan
	artifacts []codexPreparedArtifact
}

type committedCodexArtifact struct {
	prepared codexPreparedArtifact
	post     transaction.FileSnapshot
}

// These seams let the reconciliation tests inject deterministic write failure
// and concurrent-edit scenarios. Production calls the transaction package.
var writeFileAtomicIfUnchanged = transaction.WriteFileAtomicIfUnchanged
var removeFileIfUnchanged = transaction.RemoveFileIfUnchanged
var restoreFileAtomicIfPostimage = transaction.RestoreFileAtomicIfPostimage

// ReadCodexProjectionIdentity reads only the sidecar attribution required to
// select a safe surface mode. It never changes configuration or sessions.
func ReadCodexProjectionIdentity(path string) (CodexProjectionIdentity, error) {
	data, err := os.ReadFile(codexStatePath(path))
	if os.IsNotExist(err) {
		return CodexProjectionIdentity{}, nil
	}
	if err != nil {
		return CodexProjectionIdentity{}, fmt.Errorf("read Codex adapter state: %w", err)
	}
	var state codexState
	if err := json.Unmarshal(data, &state); err != nil {
		return CodexProjectionIdentity{}, fmt.Errorf("parse Codex adapter state: %w", err)
	}
	legacy, err := validateCodexStateAttribution(state, "")
	if err != nil {
		return CodexProjectionIdentity{}, err
	}
	if legacy {
		return CodexProjectionIdentity{
			Present:          true,
			ProjectionMode:   CodexProjectionFullSelection,
			AttributionState: "legacy",
		}, nil
	}
	return CodexProjectionIdentity{
		Present:          true,
		ProjectionMode:   state.ProjectionMode,
		AttributionState: "recognized",
	}, nil
}

// PlanCodexReconciliation prepares a before-to-after target transition
// without writing configuration, sidecars, credentials, or sessions.
func PlanCodexReconciliation(before, after []CodexTargetRef, runtime domain.Runtime) ([]CodexProjectionPlan, error) {
	prepared, _, err := prepareCodexReconciliation(before, after, runtime)
	if err != nil {
		return nil, err
	}
	plans := make([]CodexProjectionPlan, 0, len(prepared))
	for _, target := range prepared {
		plans = append(plans, target.plan)
	}
	return plans, nil
}

// ReconcileCodexConfigs applies a prepared before-to-after target transition.
// It guards every write against its captured preimage and compensates prior
// writes in reverse order only while their postimages remain unchanged.
func ReconcileCodexConfigs(before, after []CodexTargetRef, runtime domain.Runtime) (CodexReconciliationReceipt, error) {
	prepared, transactionID, err := prepareCodexReconciliation(before, after, runtime)
	if err != nil {
		return CodexReconciliationReceipt{}, err
	}
	receipt := CodexReconciliationReceipt{TransactionID: transactionID, Plans: make([]CodexProjectionPlan, 0, len(prepared))}
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
					return CodexReconciliationReceipt{}, fmt.Errorf("commit Codex reconciliation %s: %w; rollback also failed: %v", artifact.path, commitErr, rollbackErr)
				}
				return CodexReconciliationReceipt{}, fmt.Errorf("commit Codex reconciliation %s: %w; all artifacts rolled back", artifact.path, commitErr)
			}
			committed = append(committed, committedCodexArtifact{prepared: artifact, post: post})
		}
	}
	return receipt, nil
}

func prepareCodexReconciliation(before, after []CodexTargetRef, runtime domain.Runtime) ([]codexPreparedTarget, string, error) {
	targets, err := codexTargetUnion(before, after)
	if err != nil {
		return nil, "", err
	}
	transactionID, err := newCodexTransactionID()
	if err != nil {
		return nil, "", err
	}
	endpoint := ""
	needsEndpoint := false
	for _, target := range targets {
		if !target.desired {
			continue
		}
		if target.ref.ProjectionMode == CodexProjectionFullSelection || target.ref.ProjectionMode == CodexProjectionNamespacedFallback {
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

func prepareCodexReconciliationTarget(target codexReconciliationTarget, runtime domain.Runtime, endpoint, transactionID string) (codexPreparedTarget, error) {
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
	if !target.desired {
		return prepareCodexRestore(target.ref, configSnapshot, stateSnapshot)
	}
	switch target.ref.ProjectionMode {
	case CodexProjectionFullSelection:
		block := codexManagedBlock(runtime.ProfileLabel, endpoint)
		return prepareCodexFullSelection(target.ref, runtime, block, configSnapshot, stateSnapshot, transactionID)
	case CodexProjectionNamespacedFallback:
		block := codexFallbackBlock(runtime.ProfileLabel, endpoint)
		return prepareCodexFallback(target.ref, block, configSnapshot, stateSnapshot, transactionID)
	case CodexProjectionStaleAirFullSelectionRecovery:
		return prepareStaleAirFullSelectionRecovery(target.ref, configSnapshot, stateSnapshot)
	default:
		return codexPreparedTarget{}, fmt.Errorf("unsupported Codex projection mode %q", target.ref.ProjectionMode)
	}
}

func prepareCodexFullSelection(target CodexTargetRef, runtime domain.Runtime, block string, configSnapshot, stateSnapshot transaction.FileSnapshot, transactionID string) (codexPreparedTarget, error) {
	state, legacy, err := codexStateForTarget(stateSnapshot, CodexProjectionFullSelection)
	if err != nil {
		return codexPreparedTarget{}, err
	}
	base, projectedState, err := codexUserConfigAt(target.Path, targetCodexStatePath(target), runtime, block)
	if err != nil {
		return codexPreparedTarget{}, err
	}
	state = projectedState
	projected := []byte(projectCodex(base, block, runtime.Model))
	state.ManagedBlockHash = hashText(block)
	state.ProjectionMode = CodexProjectionFullSelection
	state.WriterID = CodexProjectionWriterID
	stateData, err := encodeCodexState(state)
	if err != nil {
		return codexPreparedTarget{}, err
	}
	if !bytes.Equal(configSnapshot.Data, projected) || !bytes.Equal(stateSnapshot.Data, stateData) || !stateSnapshot.Exists || legacy {
		state.TransactionID = transactionID
		stateData, err = encodeCodexState(state)
		if err != nil {
			return codexPreparedTarget{}, err
		}
	}
	action := "update"
	if bytes.Equal(configSnapshot.Data, projected) && stateSnapshot.Exists && bytes.Equal(stateSnapshot.Data, stateData) {
		action = "already-converged"
	} else if !stateSnapshot.Exists {
		action = "initial-project"
	} else if legacy {
		action = "adopt-legacy-sidecar"
	} else if isExactTruncatedCodexProjection(string(configSnapshot.Data), stateSnapshot.Data, runtime, block) {
		action = "repair-truncated"
	}
	return codexPreparedTarget{
		plan:      CodexProjectionPlan{Target: target.Path, Action: action},
		artifacts: codexArtifactsForDesiredState(target.Path, targetCodexStatePath(target), configSnapshot, projected, stateSnapshot, stateData),
	}, nil
}

func prepareCodexFallback(target CodexTargetRef, block string, configSnapshot, stateSnapshot transaction.FileSnapshot, transactionID string) (codexPreparedTarget, error) {
	if airTopLevelSelectsAIGW(string(configSnapshot.Data)) {
		return codexPreparedTarget{}, fmt.Errorf("Air currently selects AIGW at top level; return Air to JetBrains AI before staging or changing fallback")
	}
	if !stateSnapshot.Exists && (strings.Contains(string(configSnapshot.Data), codexFallbackBegin) || strings.Contains(string(configSnapshot.Data), "[model_providers.aigw_fallback]")) {
		return codexPreparedTarget{}, fmt.Errorf("Codex config contains an unowned AIGW fallback block; refusing to add a duplicate")
	}
	state, _, err := codexStateForTarget(stateSnapshot, CodexProjectionNamespacedFallback)
	if err != nil {
		return codexPreparedTarget{}, err
	}
	base := string(configSnapshot.Data)
	if stateSnapshot.Exists {
		base, err = removeCodexFallbackProjection(base, state)
		if err != nil {
			return codexPreparedTarget{}, err
		}
	}
	projected, prefix := projectCodexFallback(base, block)
	state.ManagedBlockHash = hashText(block)
	state.ProjectionMode = CodexProjectionNamespacedFallback
	state.WriterID = CodexProjectionWriterID
	state.FallbackPrefix = prefix
	stateData, err := encodeCodexState(state)
	if err != nil {
		return codexPreparedTarget{}, err
	}
	if !bytes.Equal(configSnapshot.Data, []byte(projected)) || !bytes.Equal(stateSnapshot.Data, stateData) || !stateSnapshot.Exists {
		state.TransactionID = transactionID
		stateData, err = encodeCodexState(state)
		if err != nil {
			return codexPreparedTarget{}, err
		}
	}
	action := "update"
	if bytes.Equal(configSnapshot.Data, []byte(projected)) && stateSnapshot.Exists && bytes.Equal(stateSnapshot.Data, stateData) {
		action = "already-converged"
	} else if !stateSnapshot.Exists {
		action = "stage-aigw-fallback"
	}
	return codexPreparedTarget{
		plan:      CodexProjectionPlan{Target: target.Path, Action: action},
		artifacts: codexArtifactsForDesiredState(target.Path, targetCodexStatePath(target), configSnapshot, []byte(projected), stateSnapshot, stateData),
	}, nil
}

func prepareCodexRestore(target CodexTargetRef, configSnapshot, stateSnapshot transaction.FileSnapshot) (codexPreparedTarget, error) {
	if !stateSnapshot.Exists {
		return codexPreparedTarget{plan: CodexProjectionPlan{Target: target.Path, Action: "already-restored"}}, nil
	}
	state, _, err := codexStateForTarget(stateSnapshot, target.ProjectionMode)
	if err != nil {
		return codexPreparedTarget{}, err
	}
	var restored string
	switch state.ProjectionMode {
	case "", CodexProjectionFullSelection:
		restored, err = removeCodexProjection(string(configSnapshot.Data), state)
	case CodexProjectionNamespacedFallback:
		restored, err = removeCodexFallbackProjection(string(configSnapshot.Data), state)
	default:
		err = fmt.Errorf("unsupported Codex sidecar projection mode %q", state.ProjectionMode)
	}
	if err != nil {
		return codexPreparedTarget{}, err
	}
	return codexPreparedTarget{
		plan:      CodexProjectionPlan{Target: target.Path, Action: "restore-external"},
		artifacts: codexArtifactsForDesiredState(target.Path, targetCodexStatePath(target), configSnapshot, []byte(restored), stateSnapshot, nil),
	}, nil
}

func prepareStaleAirFullSelectionRecovery(target CodexTargetRef, configSnapshot, stateSnapshot transaction.FileSnapshot) (codexPreparedTarget, error) {
	if !stateSnapshot.Exists {
		text := string(configSnapshot.Data)
		if airTopLevelSelectsAIGW(text) ||
			strings.Contains(text, "managed by AIGW") ||
			strings.Contains(text, codexBegin) ||
			strings.Contains(text, codexEnd) ||
			strings.Contains(text, codexFallbackBegin) ||
			strings.Contains(text, codexFallbackEnd) ||
			strings.Contains(text, "[model_providers.aigw]") ||
			strings.Contains(text, "[model_providers.aigw_fallback]") {
			return codexPreparedTarget{}, errors.New("Air contains AIGW projection residue without an attributable sidecar")
		}
		return codexPreparedTarget{
			plan: CodexProjectionPlan{Target: target.Path, Action: "already-external"},
		}, nil
	}
	state, legacy, err := codexStateForTarget(stateSnapshot, CodexProjectionNamespacedFallback)
	if err != nil {
		return codexPreparedTarget{}, err
	}
	if legacy {
		return codexPreparedTarget{}, errors.New("legacy AIGW sidecar cannot authorize stale Air full-selection recovery")
	}
	recovered, err := removeStaleAirFullSelection(string(configSnapshot.Data), state)
	if err != nil {
		return codexPreparedTarget{}, err
	}
	return codexPreparedTarget{
		plan:      CodexProjectionPlan{Target: target.Path, Action: "recover-stale-full-selection"},
		artifacts: codexArtifactsForDesiredState(target.Path, targetCodexStatePath(target), configSnapshot, []byte(recovered), stateSnapshot, nil),
	}, nil
}

func codexArtifactsForDesiredState(configPath, statePath string, configBefore transaction.FileSnapshot, configData []byte, stateBefore transaction.FileSnapshot, stateData []byte) []codexPreparedArtifact {
	artifacts := make([]codexPreparedArtifact, 0, 2)
	configDesired := desiredCodexSnapshot(configData, configBefore.Mode)
	if !sameCodexSnapshot(configBefore, configDesired) {
		artifacts = append(artifacts, codexPreparedArtifact{path: configPath, before: configBefore, desired: configDesired})
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
		artifacts = append(artifacts, codexPreparedArtifact{path: statePath, before: stateBefore, desired: stateDesired})
	}
	return artifacts
}

func commitCodexArtifact(artifact codexPreparedArtifact) (transaction.FileSnapshot, error) {
	if artifact.desired.Exists {
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

func codexStateForTarget(snapshot transaction.FileSnapshot, expectedMode string) (codexState, bool, error) {
	if !snapshot.Exists {
		return codexState{}, false, nil
	}
	var state codexState
	if err := json.Unmarshal(snapshot.Data, &state); err != nil {
		return codexState{}, false, fmt.Errorf("parse Codex adapter state: %w", err)
	}
	legacy, err := validateCodexStateAttribution(state, expectedMode)
	if err != nil {
		return codexState{}, false, err
	}
	return state, legacy, nil
}

func encodeCodexState(state codexState) ([]byte, error) {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode Codex adapter state: %w", err)
	}
	return append(data, '\n'), nil
}

func validateCodexStateAttribution(state codexState, expectedMode string) (bool, error) {
	if state.ProjectionMode == "" && state.WriterID == "" && state.TransactionID == "" {
		if expectedMode == CodexProjectionNamespacedFallback {
			return false, fmt.Errorf("legacy Codex sidecar cannot authorize an Air fallback")
		}
		return true, nil
	}
	if state.ProjectionMode == "" || state.WriterID == "" || state.TransactionID == "" {
		return false, fmt.Errorf("Codex sidecar attribution is incomplete")
	}
	if state.ProjectionMode != CodexProjectionFullSelection && state.ProjectionMode != CodexProjectionNamespacedFallback {
		return false, fmt.Errorf("Codex sidecar has unsupported projection mode %q", state.ProjectionMode)
	}
	if state.WriterID != CodexProjectionWriterID {
		return false, fmt.Errorf("Codex sidecar is owned by foreign writer %q", state.WriterID)
	}
	if expectedMode != "" && state.ProjectionMode != expectedMode {
		return false, fmt.Errorf("Codex sidecar projection mode is %q, want %q", state.ProjectionMode, expectedMode)
	}
	return false, nil
}

func codexTargetUnion(before, after []CodexTargetRef) ([]codexReconciliationTarget, error) {
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

func normalizeCodexTargets(values []CodexTargetRef) ([]CodexTargetRef, error) {
	normalized := make([]CodexTargetRef, 0, len(values))
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
	absolute, err := absoluteCodexTargetPath(path)
	if err != nil {
		return "", err
	}
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
	legacyStatePath := codexStatePath(sourcePath)
	if info, err := os.Lstat(legacyStatePath); err == nil && !info.IsDir() {
		return legacyStatePath
	}
	return canonicalStatePath
}

func targetCodexStatePath(target CodexTargetRef) string {
	if target.statePath != "" {
		return target.statePath
	}
	return codexStatePath(target.Path)
}

func validateDesiredCodexTarget(target CodexTargetRef) error {
	switch {
	case (target.SurfaceID == "codex-cli-standalone" || strings.HasPrefix(target.SurfaceID, "codex-cli-explicit-")) && target.Authority == "aigw" && target.ProjectionMode == CodexProjectionFullSelection:
		return nil
	case target.SurfaceID == "jetbrains-air-codex" && target.Authority == "jetbrains-ai" && (target.ProjectionMode == CodexProjectionNamespacedFallback || target.ProjectionMode == CodexProjectionStaleAirFullSelectionRecovery):
		return nil
	default:
		return fmt.Errorf("Codex target %s cannot use authority %s with projection mode %s", target.SurfaceID, target.Authority, target.ProjectionMode)
	}
}

func standaloneCodexTargets(paths []string) []CodexTargetRef {
	targets := make([]CodexTargetRef, 0, len(paths))
	for _, path := range paths {
		targets = append(targets, CodexTargetRef{
			SurfaceID:      "codex-cli-standalone",
			Authority:      "aigw",
			ProjectionMode: CodexProjectionFullSelection,
			Path:           path,
		})
	}
	return targets
}

func newCodexTransactionID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate Codex transaction identifier: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

func projectCodexFallback(original, block string) (string, string) {
	prefix := ""
	if original != "" && !strings.HasSuffix(original, "\n") {
		prefix = "\n"
	}
	return original + prefix + block, prefix
}

func codexFallbackBlock(label, endpoint string) string {
	label = strings.ReplaceAll(label, "\"", "'")
	return codexFallbackBegin + "\n" +
		"[model_providers.aigw_fallback]\n" +
		fmt.Sprintf("name = \"AIGW fallback: %s\"\n", label) +
		fmt.Sprintf("base_url = \"%s\"\n", endpoint) +
		"wire_api = \"responses\"\n" +
		"requires_openai_auth = true\n" +
		codexFallbackEnd + "\n"
}

func removeCodexFallbackProjection(current string, state codexState) (string, error) {
	if airTopLevelSelectsAIGW(current) {
		return "", fmt.Errorf("Air currently selects AIGW at top level; return Air to JetBrains AI before removing fallback")
	}
	start := strings.Index(current, codexFallbackBegin)
	if start < 0 {
		return "", fmt.Errorf("Codex config conflict: AIGW fallback block is missing")
	}
	endMarker := strings.Index(current[start:], codexFallbackEnd)
	if endMarker < 0 {
		return "", fmt.Errorf("Codex config conflict: AIGW fallback block is incomplete")
	}
	end := start + endMarker + len(codexFallbackEnd)
	if end < len(current) && current[end] == '\r' {
		end++
	}
	if end < len(current) && current[end] == '\n' {
		end++
	}
	block := current[start:end]
	if !managedBlockHashMatches(state.ManagedBlockHash, block) {
		return "", fmt.Errorf("Codex config conflict: AIGW fallback block changed; refusing to overwrite user edits")
	}
	if state.FallbackPrefix != "" {
		if start < len(state.FallbackPrefix) || current[start-len(state.FallbackPrefix):start] != state.FallbackPrefix {
			return "", fmt.Errorf("Codex config conflict: AIGW fallback separator changed; refusing to overwrite user edits")
		}
		start -= len(state.FallbackPrefix)
	}
	return current[:start] + current[end:], nil
}

func airTopLevelSelectsAIGW(text string) bool {
	line := modelProviderLine.FindString(text)
	return strings.Contains(line, "\"aigw\"") || strings.Contains(line, "\"aigw_fallback\"")
}
