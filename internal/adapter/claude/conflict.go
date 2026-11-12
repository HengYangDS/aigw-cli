package claude

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/platform"
)

// outsideEdit refuses settings that AIGW manages and that were changed
// outside it. Undoing the edit is the only way out: AIGW neither overwrites it
// nor adopts it.
func outsideEdit(message string) error {
	return adapter.NewConflictError(message, adapter.UndoOutsideEdit("Claude Code"), nil)
}

// refuseMissingSettings refuses settings that were deleted while AIGW's record
// of them remains, which an undone edit could not resolve without a copy.
func refuseMissingSettings(path string, settings, state platform.FileSnapshot) error {
	if state.Exists && !settings.Exists {
		return adapter.MissingWithRecord("Claude Code", path, path+adapter.StateSuffix, "")
	}
	return nil
}

// readSettingsState decodes AIGW's record of the settings at path.
func readSettingsState(path string, data []byte) (settingsState, error) {
	state, err := decodeSettingsState(data)
	if err != nil {
		return settingsState{}, adapter.UnreadableRecord(path+adapter.StateSuffix, err)
	}
	return state, nil
}

// refuseUnrecordedWithdrawal refuses to withdraw from settings that still hold
// AIGW's credential helper while AIGW's record of them is gone. Only the
// record holds the values that AIGW replaced, so without it a withdrawal would
// leave AIGW's helper, endpoint and model in place and claim that nothing
// remains; AIGW cannot tell its other values from the user's either.
func refuseUnrecordedWithdrawal(path string, document settingsDocument) error {
	var helper string
	if json.Unmarshal(document["apiKeyHelper"], &helper) != nil || !strings.Contains(helper, " credential "+ID+" ") {
		return nil
	}
	record := filepath.Base(path + adapter.StateSuffix)
	return adapter.NewConflictError(
		"AIGW's record "+record+" is missing, so AIGW cannot tell its settings in "+filepath.Base(path)+" from yours; its apiKeyHelper stays, with ANTHROPIC_BASE_URL and model",
		adapter.Rerun("Restore "+record+" in its folder from a copy of it, then run the same command again; or remove apiKeyHelper, and ANTHROPIC_BASE_URL and model if AIGW set them, from "+filepath.Base(path)+" yourself"),
		nil,
	)
}

// foreignCredential refuses settings that hold a credential before AIGW owns
// them.
func foreignCredential() error {
	return adapter.NewConflictError(
		"Claude credential ownership conflict: remove the plaintext credential or foreign apiKeyHelper before enabling AIGW",
		adapter.Rerun("Remove the plaintext credential or foreign apiKeyHelper from Claude Code's settings, then run the same command again."),
		nil,
	)
}

// unreadableSettings refuses a settings file that does not parse, because AIGW
// could not preserve what the file holds.
func unreadableSettings(err error) error {
	return adapter.NewConflictError(err.Error(), adapter.Rerun("Correct the JSON in Claude Code's settings file, then run the same command again."), err)
}
