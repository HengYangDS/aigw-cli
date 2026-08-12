package claude

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/transaction"
)

const (
	settingsStateSuffix = ".aigw-state.json"
	credentialHelper    = "aigw credential claude"
)

var (
	captureSnapshot = transaction.CaptureFileSnapshot
	writeGuarded    = transaction.WriteFileAtomicIfUnchanged
	removeGuarded   = transaction.RemoveFileIfUnchanged
	restoreGuarded  = transaction.RestoreFileAtomicIfPostimage
)

var managedEnvironmentKeys = []string{
	"ANTHROPIC_API_KEY",
	"ANTHROPIC_AUTH_TOKEN",
	"ANTHROPIC_BASE_URL",
	"ANTHROPIC_MODEL",
}

type optionalValue struct {
	Present bool            `json:"present"`
	Value   json.RawMessage `json:"value,omitempty"`
}

type originalSettings struct {
	FileExisted  bool                     `json:"file_existed"`
	APIKeyHelper optionalValue            `json:"api_key_helper"`
	Model        optionalValue            `json:"model"`
	Environment  map[string]optionalValue `json:"environment,omitempty"`
}

type settingsState struct {
	Version       int              `json:"version"`
	WriterID      string           `json:"writer_id"`
	Original      originalSettings `json:"original"`
	ManagedSHA256 string           `json:"managed_sha256"`
}

type SettingsReceipt struct {
	Action string `json:"action"`
	Target string `json:"target"`
}

type settingsDocument map[string]json.RawMessage

// ReconcileSettings atomically projects or removes AIGW-owned Claude Code
// user settings. It preserves every foreign setting, never writes a token, and
// fails closed when the owned projection has been changed externally.
func ReconcileSettings(path string, disabled bool, runtime configuration.Runtime) (SettingsReceipt, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return SettingsReceipt{}, errors.New("Claude settings path is empty")
	}
	statePath := path + settingsStateSuffix
	settingsBefore, err := captureSnapshot(path)
	if err != nil {
		return SettingsReceipt{}, fmt.Errorf("read Claude settings: %w", err)
	}
	stateBefore, err := captureSnapshot(statePath)
	if err != nil {
		return SettingsReceipt{}, fmt.Errorf("read Claude settings state: %w", err)
	}
	document, err := decodeSettings(settingsBefore)
	if err != nil {
		return SettingsReceipt{}, err
	}

	if disabled {
		return disableSettings(path, statePath, document, settingsBefore, stateBefore)
	}
	if runtime.Endpoint == "" {
		return SettingsReceipt{}, fmt.Errorf("profile %q has no Claude endpoint", runtime.ProfileID)
	}
	if runtime.AccountID == "" {
		return SettingsReceipt{}, fmt.Errorf("profile %q has no account", runtime.ProfileID)
	}

	state, err := prepareSettingsState(document, settingsBefore, stateBefore)
	if err != nil {
		return SettingsReceipt{}, err
	}
	projectSettings(document, runtime)
	settingsData := encodeSettings(document)
	state.ManagedSHA256 = managedSettingsHash(document)
	stateData := encodeSettingsState(state)
	if snapshotDataEqual(settingsBefore, settingsData) && snapshotDataEqual(stateBefore, stateData) {
		return SettingsReceipt{Action: "already-converged", Target: path}, nil
	}
	if err := commitSettings(path, statePath, settingsBefore, settingsData, stateBefore, stateData); err != nil {
		return SettingsReceipt{}, err
	}
	return SettingsReceipt{Action: "project", Target: path}, nil
}

func prepareSettingsState(document settingsDocument, settingsBefore, stateBefore transaction.FileSnapshot) (settingsState, error) {
	if !stateBefore.Exists {
		if hasCredentialOwnershipConflict(document) {
			return settingsState{}, errors.New("Claude credential ownership conflict: remove the plaintext credential or foreign apiKeyHelper before enabling AIGW")
		}
		return settingsState{
			Version:  1,
			WriterID: "aigw-cli",
			Original: captureOriginalSettings(document, settingsBefore.Exists),
		}, nil
	}
	state, err := decodeSettingsState(stateBefore.Data)
	if err != nil {
		return settingsState{}, err
	}
	if state.ManagedSHA256 != managedSettingsHash(document) {
		return settingsState{}, errors.New("managed Claude settings changed outside AIGW; refusing to overwrite user edits")
	}
	return state, nil
}

func disableSettings(path, statePath string, document settingsDocument, settingsBefore, stateBefore transaction.FileSnapshot) (SettingsReceipt, error) {
	if !stateBefore.Exists {
		return SettingsReceipt{Action: "already-restored", Target: path}, nil
	}
	state, err := decodeSettingsState(stateBefore.Data)
	if err != nil {
		return SettingsReceipt{}, err
	}
	if state.ManagedSHA256 != managedSettingsHash(document) {
		return SettingsReceipt{}, errors.New("managed Claude settings changed outside AIGW; refusing to remove user edits")
	}
	if !state.Original.FileExisted {
		if _, err := removeGuarded(path, settingsBefore); err != nil {
			return SettingsReceipt{}, fmt.Errorf("restore absent Claude settings: %w", err)
		}
		if _, err := removeGuarded(statePath, stateBefore); err != nil {
			if rollbackErr := transaction.WriteFileAtomic(path, settingsBefore.Data, 0o600); rollbackErr != nil {
				return SettingsReceipt{}, fmt.Errorf("remove Claude settings state: %w; settings rollback failed: %v", err, rollbackErr)
			}
			return SettingsReceipt{}, fmt.Errorf("remove Claude settings state: %w", err)
		}
		return SettingsReceipt{Action: "restore", Target: path}, nil
	}
	restoreOriginalSettings(document, state.Original)
	settingsData := encodeSettings(document)
	settingsAfter, err := writeGuarded(path, settingsBefore, settingsData, 0o600)
	if err != nil {
		return SettingsReceipt{}, fmt.Errorf("restore Claude settings: %w", err)
	}
	if _, err := removeGuarded(statePath, stateBefore); err != nil {
		rollbackErr := restoreGuarded(path, settingsBefore, settingsAfter)
		if rollbackErr != nil {
			return SettingsReceipt{}, fmt.Errorf("remove Claude settings state: %w; settings rollback failed: %v", err, rollbackErr)
		}
		return SettingsReceipt{}, fmt.Errorf("remove Claude settings state: %w", err)
	}
	return SettingsReceipt{Action: "restore", Target: path}, nil
}

func commitSettings(path, statePath string, settingsBefore transaction.FileSnapshot, settingsData []byte, stateBefore transaction.FileSnapshot, stateData []byte) error {
	settingsAfter, err := writeGuarded(path, settingsBefore, settingsData, 0o600)
	if err != nil {
		return fmt.Errorf("write Claude settings: %w", err)
	}
	if _, err := writeGuarded(statePath, stateBefore, stateData, 0o600); err != nil {
		rollbackErr := restoreGuarded(path, settingsBefore, settingsAfter)
		if rollbackErr != nil {
			return fmt.Errorf("write Claude settings state: %w; settings rollback failed: %v", err, rollbackErr)
		}
		return fmt.Errorf("write Claude settings state: %w", err)
	}
	return nil
}

func decodeSettings(snapshot transaction.FileSnapshot) (settingsDocument, error) {
	if !snapshot.Exists || len(bytes.TrimSpace(snapshot.Data)) == 0 {
		return settingsDocument{}, nil
	}
	var document settingsDocument
	decoder := json.NewDecoder(bytes.NewReader(snapshot.Data))
	decoder.UseNumber()
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("parse Claude settings: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("parse Claude settings: trailing JSON value")
	}
	if document == nil {
		return settingsDocument{}, nil
	}
	return document, nil
}

func encodeSettings(document settingsDocument) []byte {
	data, _ := json.MarshalIndent(document, "", "  ")
	return append(data, '\n')
}

func decodeEnvironment(document settingsDocument) (map[string]json.RawMessage, error) {
	raw, ok := document["env"]
	if !ok {
		return map[string]json.RawMessage{}, nil
	}
	var environment map[string]json.RawMessage
	if err := json.Unmarshal(raw, &environment); err != nil {
		return nil, fmt.Errorf("parse Claude settings env: %w", err)
	}
	if environment == nil {
		environment = map[string]json.RawMessage{}
	}
	return environment, nil
}

func encodeRaw(value string) json.RawMessage {
	data, _ := json.Marshal(value)
	return data
}

func projectSettings(document settingsDocument, runtime configuration.Runtime) {
	environment, _ := decodeEnvironment(document)
	for _, key := range managedEnvironmentKeys {
		delete(environment, key)
	}
	environment["ANTHROPIC_BASE_URL"] = encodeRaw(runtime.Endpoint)
	document["env"], _ = json.Marshal(environment)
	if runtime.Model == "" {
		delete(document, "model")
	} else {
		document["model"] = encodeRaw(runtime.Model)
	}
	document["apiKeyHelper"] = encodeRaw(credentialHelper)
}

func captureOriginalSettings(document settingsDocument, fileExisted bool) originalSettings {
	environment, _ := decodeEnvironment(document)
	original := originalSettings{
		FileExisted:  fileExisted,
		APIKeyHelper: captureOptional(document, "apiKeyHelper"),
		Model:        captureOptional(document, "model"),
		Environment:  map[string]optionalValue{},
	}
	for _, key := range managedEnvironmentKeys {
		if value, ok := environment[key]; ok {
			original.Environment[key] = optionalValue{Present: true, Value: append(json.RawMessage(nil), value...)}
		} else {
			original.Environment[key] = optionalValue{}
		}
	}
	return original
}

func restoreOriginalSettings(document settingsDocument, original originalSettings) {
	restoreOptional(document, "apiKeyHelper", original.APIKeyHelper)
	restoreOptional(document, "model", original.Model)
	environment, _ := decodeEnvironment(document)
	for _, key := range managedEnvironmentKeys {
		value := original.Environment[key]
		if value.Present {
			environment[key] = append(json.RawMessage(nil), value.Value...)
		} else {
			delete(environment, key)
		}
	}
	if len(environment) == 0 {
		delete(document, "env")
	} else {
		document["env"], _ = json.Marshal(environment)
	}
}

func captureOptional(document settingsDocument, key string) optionalValue {
	value, ok := document[key]
	if !ok {
		return optionalValue{}
	}
	return optionalValue{Present: true, Value: append(json.RawMessage(nil), value...)}
}

func restoreOptional(document settingsDocument, key string, value optionalValue) {
	if value.Present {
		document[key] = append(json.RawMessage(nil), value.Value...)
	} else {
		delete(document, key)
	}
}

func hasCredentialOwnershipConflict(document settingsDocument) bool {
	if _, ok := document["apiKeyHelper"]; ok {
		return true
	}
	environment, err := decodeEnvironment(document)
	if err != nil {
		return true
	}
	_, authToken := environment["ANTHROPIC_AUTH_TOKEN"]
	_, apiKey := environment["ANTHROPIC_API_KEY"]
	return authToken || apiKey
}

func managedSettingsHash(document settingsDocument) string {
	environment, _ := decodeEnvironment(document)
	managed := settingsDocument{}
	for _, key := range managedEnvironmentKeys {
		if value, ok := environment[key]; ok {
			managed[key] = value
		}
	}
	if value, ok := document["apiKeyHelper"]; ok {
		managed["apiKeyHelper"] = value
	}
	if value, ok := document["model"]; ok {
		managed["model"] = value
	}
	data, _ := json.Marshal(managed)
	return hashBytes(data)
}

func loadSettingsState(path string) (settingsState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return settingsState{}, err
	}
	return decodeSettingsState(data)
}

func decodeSettingsState(data []byte) (settingsState, error) {
	var state settingsState
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return settingsState{}, fmt.Errorf("parse Claude settings state: %w", err)
	}
	if state.Version != 1 || state.WriterID != "aigw-cli" || state.ManagedSHA256 == "" {
		return settingsState{}, errors.New("Claude settings state is incomplete or not owned by AIGW")
	}
	return state, nil
}

func encodeSettingsState(state settingsState) []byte {
	data, _ := json.MarshalIndent(state, "", "  ")
	return append(data, '\n')
}

func snapshotDataEqual(snapshot transaction.FileSnapshot, data []byte) bool {
	return snapshot.Exists && bytes.Equal(snapshot.Data, data)
}
func hashBytes(data []byte) string {
	sum := sha256Sum(data)
	return fmt.Sprintf("%x", sum)
}

func sha256Sum(data []byte) [32]byte {
	return sha256.Sum256(data)
}
