package codex

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Inspection is a secret-free classification of the local ownership
// state. It never contains a path, endpoint, provider block, model, token, or
// sidecar transaction identifier.
type Inspection struct {
	State              string `json:"state"`
	DiskSelection      string `json:"disk_selection"`
	ProjectionMode     string `json:"projection_mode"`
	AttributionState   string `json:"attribution_state"`
	AIGWManaged        bool   `json:"aigw_managed"`
	SidecarPresent     bool   `json:"sidecar_present"`
	SidecarHashMatches bool   `json:"sidecar_hash_matches"`
}

// InspectConfig reads configuration and sidecar state without changing
// either. It converts all content into bounded ownership classifications.
func InspectConfig(path string) (Inspection, error) {
	configData, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Inspection{State: "missing", DiskSelection: "not-present", AttributionState: "none"}, nil
	}
	if err != nil {
		return Inspection{}, fmt.Errorf("read Codex config: %w", err)
	}
	text := string(configData)
	inspection := Inspection{DiskSelection: classifyCodexDiskSelection(text), AttributionState: "none"}
	stateData, err := os.ReadFile(codexStatePath(path))
	if os.IsNotExist(err) {
		inspection.State = "external"
		if strings.Contains(text, codexBegin) || inspection.DiskSelection == "aigw-managed" {
			inspection.State = "orphaned-aigw-marker"
		}
		return inspection, nil
	}
	if err != nil {
		return Inspection{}, fmt.Errorf("read Codex sidecar: %w", err)
	}
	inspection.SidecarPresent = true
	var state codexState
	if err := json.Unmarshal(stateData, &state); err != nil {
		inspection.State = "invalid-sidecar"
		inspection.AttributionState = "invalid"
		return inspection, nil
	}
	if attributionErr := validateCodexStateAttribution(state); attributionErr != nil {
		inspection.State = "ownership-conflict"
		inspection.AttributionState = "foreign-or-incomplete"
		inspection.ProjectionMode = state.ProjectionMode
		return inspection, nil
	}
	inspection.AttributionState = "recognized"
	inspection.ProjectionMode = state.ProjectionMode
	provider := codexStateProvider(state)
	inspection.DiskSelection = classifyCodexDiskSelectionForProvider(text, provider)
	block, err := codexManagedBlockForProviderIn(text, provider)
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
	if inspection.DiskSelection != "aigw-managed" {
		inspection.State = "aigw-drift"
	} else {
		inspection.State = "aigw-managed"
	}
	return inspection, nil
}

func classifyCodexDiskSelection(text string) string {
	return classifyCodexDiskSelectionForProvider(text, "aigw")
}

func classifyCodexDiskSelectionForProvider(text, provider string) string {
	line := modelProviderLine.FindString(text)
	if line == "" {
		return "unset"
	}
	if isManagedSelection(line, "model_provider", provider) {
		return "aigw-managed"
	}
	_, value, _ := strings.Cut(line, "=")
	value, _, _ = strings.Cut(value, "#")
	value = strings.Trim(strings.TrimSpace(value), "\"")
	if value == "aigw" || value == "aigw_fallback" {
		return "aigw-user-selected"
	}
	return "external-or-host-owned"
}
