package claude

import (
	"encoding/json"

	"aigw-cli/internal/adapter"
)

// withdrawal plans the removal of AIGW's settings from document, restoring
// the values that the first record holds and the map that the record of owned
// keys holds. Without the first record nothing is AIGW's to remove, unless
// AIGW's credential helper is still there, which it refuses; the record of
// owned keys still says what the map held before AIGW wrote it, including
// when a previous release withdrew the client after a rollback.
func (change settingsChange) withdrawal(document settingsDocument, previousModel string) (settingsChange, error) {
	var original originalSettings
	if change.stateBefore.Exists {
		state, err := readSettingsState(change.path, change.stateBefore.Data)
		if err != nil {
			return settingsChange{}, err
		}
		if !state.acceptModelPreference(document, previousModel) {
			return settingsChange{}, outsideEdit("managed Claude settings changed outside AIGW; refusing to remove user edits")
		}
		original = state.Original
	} else if err := refuseUnrecordedWithdrawal(change.path, document); err != nil {
		return settingsChange{}, err
	}
	// The first record's values go back first, and the record of owned keys
	// then decides the map against the settings as the user has them.
	restoreOriginalSettings(document, original)
	change, err := change.ownedKeysForWithdrawal(document)
	if err != nil {
		return settingsChange{}, err
	}
	change.plan = SettingsPlan{Action: SettingsActionRestore, Target: change.path, ownedKeys: change.planKeys != keysUnchanged}
	if !change.stateBefore.Exists {
		if change.planKeys == keysUnchanged {
			change.plan = SettingsPlan{Action: SettingsActionAlreadyRestored, Target: change.path}
			return change, nil
		}
		// Only the map that a previous release left behind is AIGW's to take
		// back, and its entry alone says what to restore.
		change.settingsData, _ = adapter.EncodeJSON(document)
		return change, nil
	}
	// The first record owns whether the file existed before AIGW. Settings
	// that AIGW created and that hold nothing once its values are back are
	// removed. The record of owned keys is as ownedKeysForWithdrawal planned
	// it: removed with the map's entry, or kept with an entry that this
	// release does not know, which is never dropped.
	if !original.FileExisted && len(document) == 0 {
		change.removeSettings = true
		return change, nil
	}
	change.settingsData, _ = adapter.EncodeJSON(document)
	return change, nil
}

// ownedKeysForWithdrawal follows the design's table with no map to write: the
// key returns to what the entry recorded and the entry goes; an entry that
// this program does not know stays, so the record is removed only when no
// entry remains. The map is decided again after the other restored values,
// because a recorded map that equals one of them, such as an absent value, is
// AIGW's own map as much as one that carries its digest.
func (change settingsChange) ownedKeysForWithdrawal(document settingsDocument) (settingsChange, error) {
	record, err := readOwnedKeys(change.path, change.keysBefore)
	if err != nil {
		return settingsChange{}, err
	}
	action, err := decideOwnedKey(document[ownedKeyModelOverrides], nil, record.Keys[ownedKeyModelOverrides])
	if err != nil {
		return settingsChange{}, err
	}
	projectedOwnedKey(ownedKeyModelOverrides, document, action, nil, record.Keys[ownedKeyModelOverrides])
	delete(record.Keys, ownedKeyModelOverrides)
	change.planKeys = ownedKeysForWrite(change.keysBefore, normalizedRecord(record.Keys))
	if change.planKeys == keysWritten {
		change.keysData = ownedKeysData(normalizedRecord(record.Keys))
	}
	return change, nil
}

// restoreOriginalSettings restores the values that the first record holds.
// The map is the record of owned keys' to restore, not the first record's.
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
