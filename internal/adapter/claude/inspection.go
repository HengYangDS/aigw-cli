package claude

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"runtime"
	"strings"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
)

// SettingsInspection identifies a native model preference while retaining
// AIGW's sidecar-proven endpoint and credential-helper ownership.
type SettingsInspection struct {
	NativeModelOverride bool
}

// ObservedCredentialExecutable reads the exact AIGW helper grammar without
// trusting the settings. InspectSettings must still verify its ownership state.
func ObservedCredentialExecutable(path string, selected domain.Runtime) (string, error) {
	snapshot, err := captureSnapshot(path)
	if err != nil {
		return "", fmt.Errorf("read Claude settings: %w", err)
	}
	document, err := decodeSettings(snapshot)
	if err != nil {
		return "", err
	}
	var command string
	if err := json.Unmarshal(document["apiKeyHelper"], &command); err != nil {
		return "", fmt.Errorf("decode Claude credential helper: %w", err)
	}
	return secret.ExecutableFromCommand(command, ID, selected.CredentialProjectionFingerprint(ID), runtime.GOOS)
}

// InspectSettings checks Claude's owned connection without writing settings,
// sidecar state, or credentials. A model-only native preference is not a Route
// wire-model observation.
func InspectSettings(path string, runtime domain.Runtime, executable string) (SettingsInspection, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return SettingsInspection{}, fmt.Errorf("Claude settings are not synchronized: settings path is empty")
	}
	settingsBefore, err := captureSnapshot(path)
	if err != nil {
		return SettingsInspection{}, fmt.Errorf("Claude settings are not synchronized: read settings: %w", err)
	}
	stateBefore, err := captureSnapshot(path + adapter.StateSuffix)
	if err != nil {
		return SettingsInspection{}, fmt.Errorf("Claude settings are not synchronized: read ownership state: %w", err)
	}
	if !stateBefore.Exists {
		return SettingsInspection{}, fmt.Errorf("Claude settings are not synchronized; run `aigw sync`")
	}
	if err := refuseMissingSettings(path, settingsBefore, stateBefore); err != nil {
		return SettingsInspection{}, err
	}
	document, err := decodeSettings(settingsBefore)
	if err != nil {
		// Synchronization refuses this file too, so the error's own next
		// action names what resolves it.
		return SettingsInspection{}, fmt.Errorf("Claude settings are not synchronized: %w", err)
	}
	state, err := readSettingsState(path, stateBefore.Data)
	if err != nil {
		// Synchronization refuses this record too.
		return SettingsInspection{}, fmt.Errorf("Claude settings are not synchronized: %w", err)
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
	if runtime.UpstreamModel == "" {
		delete(previous, "model")
	} else {
		previous["model"] = adapter.RawJSON(runtime.UpstreamModel)
	}
	if managedSettingsHash(previous) == expectedHash {
		return SettingsInspection{NativeModelOverride: true}, nil
	}
	return SettingsInspection{}, outsideEdit("Claude settings are not synchronized: managed connection changed outside AIGW")
}

// unsynchronized is the status of settings that do not match the selection:
// the refusal, when synchronization would refuse them, or else the issue,
// which synchronization repairs.
func unsynchronized(issue, path, home string, selected domain.Runtime) adapter.Status {
	refusal := synchronizationRefusal(path, selected.UpstreamModel)
	if status, refused := adapter.ConflictStatus(refusal); refused {
		return status
	}
	if failed, atTarget := errors.AsType[*adapter.TargetError](refusal); atTarget {
		if status, unreadable := adapter.UnreadableStatus(refusal, failed.Path, home); unreadable {
			return status
		}
	}
	return adapter.Status{Issue: issue, RepairAction: "aigw sync"}
}

// synchronizationRefusal runs synchronization's own guards on the settings at
// path, and writes nothing. A selection or credential reader that changed is
// drift in a file that AIGW wrote, and passes them; an outside edit does not.
func synchronizationRefusal(path, model string) error {
	settingsBefore, err := captureSnapshot(path)
	if err != nil {
		return &adapter.TargetError{Path: path, Desired: true, Err: err}
	}
	stateBefore, err := captureSnapshot(path + adapter.StateSuffix)
	if err != nil {
		return &adapter.TargetError{Path: path + adapter.StateSuffix, Desired: true, Err: err}
	}
	if err := refuseMissingSettings(path, settingsBefore, stateBefore); err != nil {
		return err
	}
	document, err := decodeSettings(settingsBefore)
	if err != nil {
		return err
	}
	_, err = prepareSettingsState(path, document, settingsBefore, stateBefore, model)
	return err
}

// ValidateSettings accepts a sidecar-proven native model preference while
// rejecting endpoint, helper, and managed-credential changes.
func ValidateSettings(path string, runtime domain.Runtime, executable string) error {
	_, err := InspectSettings(path, runtime, executable)
	return err
}
