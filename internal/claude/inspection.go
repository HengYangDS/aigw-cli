package claude

import (
	"fmt"
	"maps"
	"strings"

	configuration "aigw-cli/internal/configuration"
)

// SettingsInspection identifies a native model preference while retaining
// AIGW's sidecar-proven endpoint and credential-helper ownership.
type SettingsInspection struct {
	NativeModelOverride bool
}

// InspectSettings checks Claude's owned connection without writing settings,
// sidecar state, or credentials. A model-only native preference is not a Route
// wire-model observation.
func InspectSettings(path string, runtime configuration.Runtime, executable string) (SettingsInspection, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return SettingsInspection{}, fmt.Errorf("Claude settings are not synchronized: settings path is empty")
	}
	settingsBefore, err := captureSnapshot(path)
	if err != nil {
		return SettingsInspection{}, fmt.Errorf("Claude settings are not synchronized: read settings: %w", err)
	}
	stateBefore, err := captureSnapshot(path + settingsStateSuffix)
	if err != nil {
		return SettingsInspection{}, fmt.Errorf("Claude settings are not synchronized: read ownership state: %w", err)
	}
	if !stateBefore.Exists {
		return SettingsInspection{}, fmt.Errorf("Claude settings are not synchronized; run `aigw sync`")
	}
	document, err := decodeSettings(settingsBefore)
	if err != nil {
		return SettingsInspection{}, fmt.Errorf("Claude settings are not synchronized: %w; run `aigw sync`", err)
	}
	state, err := decodeSettingsState(stateBefore.Data)
	if err != nil {
		return SettingsInspection{}, fmt.Errorf("Claude settings are not synchronized: %w; run `aigw sync`", err)
	}
	executable, err = validateExecutable(executable)
	if err != nil {
		return SettingsInspection{}, fmt.Errorf("Claude settings are not synchronized: %w; run `aigw sync`", err)
	}
	expected := maps.Clone(document)
	projectSettings(expected, runtime, executable)
	expectedHash := managedSettingsHash(expected)
	if state.ManagedSHA256 != expectedHash {
		return SettingsInspection{}, fmt.Errorf("Claude settings are not synchronized; run `aigw sync`")
	}
	if managedSettingsHash(document) == expectedHash {
		return SettingsInspection{}, nil
	}
	previous := maps.Clone(document)
	if runtime.Model == "" {
		delete(previous, "model")
	} else {
		previous["model"] = encodeRaw(runtime.Model)
	}
	if managedSettingsHash(previous) == expectedHash {
		return SettingsInspection{NativeModelOverride: true}, nil
	}
	return SettingsInspection{}, fmt.Errorf("Claude settings are not synchronized: managed connection changed outside AIGW")
}

// ValidateSettings accepts a sidecar-proven native model preference while
// rejecting endpoint, helper, and managed-credential changes.
func ValidateSettings(path string, runtime configuration.Runtime, executable string) error {
	_, err := InspectSettings(path, runtime, executable)
	return err
}
