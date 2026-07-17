package adapters

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// CodexInspection is a secret-free classification of the local ownership
// state. It never contains a path, endpoint, provider block, model, token, or
// sidecar transaction identifier.
type CodexInspection struct {
	State              string `json:"state"`
	DiskSelection      string `json:"disk_selection"`
	ProjectionMode     string `json:"projection_mode"`
	AttributionState   string `json:"attribution_state"`
	AIGWManaged        bool   `json:"aigw_managed"`
	SidecarPresent     bool   `json:"sidecar_present"`
	SidecarHashMatches bool   `json:"sidecar_hash_matches"`
}

// InspectCodexConfig reads configuration and sidecar state without changing
// either. It converts all content into bounded ownership classifications.
func InspectCodexConfig(path string) (CodexInspection, error) {
	configData, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return CodexInspection{State: "missing", DiskSelection: "not-present", AttributionState: "none"}, nil
	}
	if err != nil {
		return CodexInspection{}, fmt.Errorf("read Codex config: %w", err)
	}
	text := string(configData)
	inspection := CodexInspection{DiskSelection: classifyCodexDiskSelection(text), AttributionState: "none"}
	stateData, err := os.ReadFile(codexStatePath(path))
	if os.IsNotExist(err) {
		inspection.State = "external"
		if strings.Contains(text, codexBegin) || strings.Contains(text, codexFallbackBegin) || inspection.DiskSelection == "aigw-managed" {
			inspection.State = "orphaned-aigw-marker"
		}
		return inspection, nil
	}
	if err != nil {
		return CodexInspection{}, fmt.Errorf("read Codex sidecar: %w", err)
	}
	inspection.SidecarPresent = true
	var state codexState
	if err := json.Unmarshal(stateData, &state); err != nil {
		inspection.State = "invalid-sidecar"
		inspection.AttributionState = "invalid"
		return inspection, nil
	}
	legacy, attributionErr := validateCodexStateAttribution(state, "")
	if attributionErr != nil {
		inspection.State = "ownership-conflict"
		inspection.AttributionState = "foreign-or-incomplete"
		inspection.ProjectionMode = state.ProjectionMode
		return inspection, nil
	}
	if legacy {
		inspection.AttributionState = "legacy"
		inspection.ProjectionMode = CodexProjectionFullSelection
	} else {
		inspection.AttributionState = "recognized"
		inspection.ProjectionMode = state.ProjectionMode
	}

	var block string
	switch inspection.ProjectionMode {
	case CodexProjectionFullSelection:
		block, err = codexManagedBlockIn(text)
	case CodexProjectionNamespacedFallback:
		block, err = codexFallbackBlockIn(text)
	default:
		inspection.State = "ownership-conflict"
		return inspection, nil
	}
	if err != nil {
		inspection.State = "stale-sidecar"
		return inspection, nil
	}
	inspection.SidecarHashMatches = managedBlockHashMatches(state.ManagedBlockHash, block)
	if !inspection.SidecarHashMatches {
		inspection.State = "aigw-drift"
		return inspection, nil
	}
	inspection.AIGWManaged = true
	switch inspection.ProjectionMode {
	case CodexProjectionFullSelection:
		if inspection.DiskSelection != "aigw-managed" {
			inspection.State = "aigw-drift"
		} else if inspection.AttributionState == "legacy" {
			inspection.State = "legacy-full-selection"
		} else {
			inspection.State = "aigw-managed"
		}
	case CodexProjectionNamespacedFallback:
		if inspection.DiskSelection == "aigw-managed" || inspection.DiskSelection == "aigw-user-selected" {
			inspection.State = "fallback-selected-conflict"
		} else {
			inspection.State = "fallback-staged"
		}
	}
	return inspection, nil
}

func classifyCodexDiskSelection(text string) string {
	line := modelProviderLine.FindString(text)
	if line == "" {
		return "unset"
	}
	if isManagedSelection(line, "model_provider", "aigw") {
		return "aigw-managed"
	}
	_, value, ok := strings.Cut(line, "=")
	if !ok {
		return "unrecognized"
	}
	value, _, _ = strings.Cut(value, "#")
	value = strings.Trim(strings.TrimSpace(value), "\"")
	if value == "aigw" || value == "aigw_fallback" {
		return "aigw-user-selected"
	}
	return "external-or-host-owned"
}

func codexFallbackBlockIn(current string) (string, error) {
	start := strings.Index(current, codexFallbackBegin)
	if start < 0 {
		return "", fmt.Errorf("Codex fallback block is missing")
	}
	endMarker := strings.Index(current[start:], codexFallbackEnd)
	if endMarker < 0 {
		return "", fmt.Errorf("Codex fallback block is incomplete")
	}
	end := start + endMarker + len(codexFallbackEnd)
	if end < len(current) && current[end] == '\r' {
		end++
	}
	if end < len(current) && current[end] == '\n' {
		end++
	}
	return current[start:end], nil
}
