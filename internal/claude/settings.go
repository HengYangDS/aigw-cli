package claude

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/transaction"
)

const (
	settingsStateSuffix = ".aigw-state.json"
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

// SettingsPlan describes the non-secret action and target for a Claude settings projection.
type SettingsPlan struct {
	Action string `json:"action"`
	Target string `json:"target"`
}

// SettingsReceipt records the settings projection that was actually applied.
type SettingsReceipt struct {
	SettingsPlan
	rollback func() error
}

// Rollback restores the captured preimages only while this projection's
// postimages remain unchanged. A receipt for a no-op never changes files.
func (receipt SettingsReceipt) Rollback() error {
	if receipt.rollback == nil {
		return nil
	}
	return receipt.rollback()
}

type settingsDocument map[string]json.RawMessage

type settingsChange struct {
	plan           SettingsPlan
	path           string
	statePath      string
	settingsBefore transaction.FileSnapshot
	stateBefore    transaction.FileSnapshot
	settingsData   []byte
	stateData      []byte
	removeSettings bool
}

// PlanSettings returns the exact non-secret Claude Code settings change that
// ReconcileSettings would apply without writing either the settings file or
// its ownership state. previousModel is the selection before this transition,
// not a replacement inferred from the user's settings.
func PlanSettings(path string, disabled bool, runtime configuration.Runtime, executable, previousModel string) (SettingsPlan, error) {
	change, err := prepareSettingsChange(path, disabled, runtime, executable, previousModel)
	if err != nil {
		return SettingsPlan{}, err
	}
	return change.plan, nil
}

// ValidateSettings checks that the owned Claude settings match the selected
// runtime and credential helper without writing files or reading credentials.
func ValidateSettings(path string, runtime configuration.Runtime, executable string) error {
	settings, err := PlanSettings(path, false, runtime, executable, runtime.Model)
	if err != nil {
		return fmt.Errorf("Claude settings are not synchronized: %w; run `aigw sync`", err)
	}
	if settings.Action != "already-converged" {
		return fmt.Errorf("Claude settings are not synchronized; run `aigw sync`")
	}
	return nil
}

// ReconcileSettings atomically projects or removes AIGW-owned Claude Code
// user settings. It preserves every foreign setting, never writes a token, and
// accepts model preferences only when previousModel proves every other owned
// field unchanged. Connection and credential conflicts remain protected.
func ReconcileSettings(path string, disabled bool, runtime configuration.Runtime, executable, previousModel string) (SettingsReceipt, error) {
	change, err := prepareSettingsChange(path, disabled, runtime, executable, previousModel)
	if err != nil {
		return SettingsReceipt{}, err
	}
	switch change.plan.Action {
	case "already-converged", "already-restored":
		return SettingsReceipt{SettingsPlan: change.plan}, nil
	case "project", "restore":
		return change.apply()
	default:
		return SettingsReceipt{}, fmt.Errorf("unsupported Claude settings action %q", change.plan.Action)
	}
}

func prepareSettingsChange(path string, disabled bool, runtime configuration.Runtime, executable, previousModel string) (settingsChange, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return settingsChange{}, errors.New("Claude settings path is empty")
	}
	statePath := path + settingsStateSuffix
	settingsBefore, err := captureSnapshot(path)
	if err != nil {
		return settingsChange{}, fmt.Errorf("read Claude settings: %w", err)
	}
	stateBefore, err := captureSnapshot(statePath)
	if err != nil {
		return settingsChange{}, fmt.Errorf("read Claude settings state: %w", err)
	}
	document, err := decodeSettings(settingsBefore)
	if err != nil {
		return settingsChange{}, err
	}
	change := settingsChange{
		path:           path,
		statePath:      statePath,
		settingsBefore: settingsBefore,
		stateBefore:    stateBefore,
	}

	if disabled && !stateBefore.Exists {
		change.plan = SettingsPlan{Action: "already-restored", Target: path}
		return change, nil
	}
	if disabled {
		state, err := decodeSettingsState(stateBefore.Data)
		if err != nil {
			return settingsChange{}, err
		}
		if !state.acceptModelPreference(document, previousModel) {
			return settingsChange{}, errors.New("managed Claude settings changed outside AIGW; refusing to remove user edits")
		}
		change.plan = SettingsPlan{Action: "restore", Target: path}
		restoreOriginalSettings(document, state.Original)
		if !state.Original.FileExisted && len(document) == 0 {
			change.removeSettings = true
			return change, nil
		}
		change.settingsData = encodeSettings(document)
		return change, nil
	}
	if runtime.Endpoint == "" {
		return settingsChange{}, fmt.Errorf("profile %q has no Claude endpoint", runtime.ProfileID)
	}
	if runtime.AccountID == "" {
		return settingsChange{}, fmt.Errorf("profile %q has no account", runtime.ProfileID)
	}
	executable, err = validateExecutable(executable)
	if err != nil {
		return settingsChange{}, err
	}

	state, err := prepareSettingsState(document, settingsBefore, stateBefore, previousModel)
	if err != nil {
		return settingsChange{}, err
	}
	projectSettings(document, runtime, executable)
	change.settingsData = encodeSettings(document)
	state.ManagedSHA256 = managedSettingsHash(document)
	change.stateData = encodeSettingsState(state)
	if snapshotDataEqual(settingsBefore, change.settingsData) && snapshotDataEqual(stateBefore, change.stateData) {
		change.plan = SettingsPlan{Action: "already-converged", Target: path}
		return change, nil
	}
	change.plan = SettingsPlan{Action: "project", Target: path}
	return change, nil
}

func prepareSettingsState(document settingsDocument, settingsBefore, stateBefore transaction.FileSnapshot, previousModel string) (settingsState, error) {
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
	if !state.acceptModelPreference(document, previousModel) {
		return settingsState{}, errors.New("managed Claude settings changed outside AIGW; refusing to overwrite user edits")
	}
	return state, nil
}

func (state *settingsState) acceptModelPreference(document settingsDocument, previousModel string) bool {
	if state.ManagedSHA256 == managedSettingsHash(document) {
		return true
	}
	previous := maps.Clone(document)
	delete(previous, "model")
	if previousModel != "" {
		previous["model"] = encodeRaw(previousModel)
	}
	if state.ManagedSHA256 != managedSettingsHash(previous) {
		return false
	}
	state.Original.Model = captureOptional(document, "model")
	return true
}

func (change settingsChange) apply() (SettingsReceipt, error) {
	var settingsAfter transaction.FileSnapshot
	var err error
	settingsAction, stateAction := "write", "write"
	if change.plan.Action == "restore" {
		settingsAction, stateAction = "restore", "remove"
	}
	if change.removeSettings {
		settingsAfter, err = removeGuarded(change.path, change.settingsBefore)
	} else {
		settingsAfter, err = writeGuarded(change.path, change.settingsBefore, change.settingsData, 0o600)
	}
	if err != nil {
		return SettingsReceipt{}, fmt.Errorf("%s Claude settings: %w", settingsAction, err)
	}
	var stateAfter transaction.FileSnapshot
	if change.plan.Action == "restore" {
		stateAfter, err = removeGuarded(change.statePath, change.stateBefore)
	} else {
		stateAfter, err = writeGuarded(change.statePath, change.stateBefore, change.stateData, 0o600)
	}
	if err != nil {
		rollbackErr := restoreGuarded(change.path, change.settingsBefore, settingsAfter)
		if rollbackErr != nil {
			return SettingsReceipt{}, fmt.Errorf("%s Claude settings state: %w; settings rollback failed: %w", stateAction, err, rollbackErr)
		}
		return SettingsReceipt{}, fmt.Errorf("%s Claude settings state: %w", stateAction, err)
	}
	return SettingsReceipt{SettingsPlan: change.plan, rollback: func() error {
		return errors.Join(
			restoreGuarded(change.path, change.settingsBefore, settingsAfter),
			restoreGuarded(change.statePath, change.stateBefore, stateAfter),
		)
	}}, nil
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
	if _, err := decodeEnvironment(document); err != nil {
		return nil, err
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

func projectSettings(document settingsDocument, runtime configuration.Runtime, executable string) {
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
	document["apiKeyHelper"] = encodeRaw(credentialHelper(executable, runtime.CredentialProjectionFingerprint(configuration.ClientClaude)))
}

func validateExecutable(executable string) (string, error) {
	if strings.IndexFunc(executable, unicode.IsControl) >= 0 {
		return "", errors.New("AIGW executable path contains control characters")
	}
	executable = strings.TrimSpace(executable)
	if executable == "" || !filepath.IsAbs(executable) {
		return "", errors.New("AIGW executable path must be absolute")
	}
	return executable, nil
}

func credentialHelper(executable, scope string) string {
	if runtime.GOOS == "windows" {
		return `"` + strings.ReplaceAll(executable, `"`, `\"`) + `" credential claude ` + scope
	}
	return "'" + strings.ReplaceAll(executable, "'", "'\\''") + "' credential claude " + scope
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
	for key, raw := range managed {
		var value string
		if bytes.HasPrefix(bytes.TrimSpace(raw), []byte(`"`)) && json.Unmarshal(raw, &value) == nil {
			managed[key] = encodeRaw(value)
		}
	}
	data, _ := json.Marshal(managed)
	return hashBytes(data)
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
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum)
}
