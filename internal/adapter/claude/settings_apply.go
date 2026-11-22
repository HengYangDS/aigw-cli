package claude

import (
	"fmt"
	"slices"

	"aigw-cli/internal/platform"
)

// SettingsReceipt records the settings projection that was actually applied.
type SettingsReceipt struct {
	SettingsPlan
	restore func() platform.Restoration
}

// restorationFile is one file that a transition wrote, and its preimage and
// postimage.
type restorationFile struct {
	path          string
	before, after platform.FileSnapshot
}

// undoWritten restores each written file in the reverse order of writing,
// while it still holds the transition's own postimage.
func undoWritten(written []restorationFile) platform.Restoration {
	restored := make(platform.Restoration, 0, len(written))
	for _, file := range slices.Backward(written) {
		restored = append(restored, platform.FileOutcome{Path: file.path, Err: restoreGuarded(file.path, file.before, file.after)})
	}
	return restored
}

// Rollback restores the captured preimages only while this projection's
// postimages remain unchanged. A receipt for a no-op never changes files.
func (receipt SettingsReceipt) Rollback() error {
	return receipt.Restore().Err()
}

// Restore rolls the projection back as Rollback does and reports each file
// that it wrote.
func (receipt SettingsReceipt) Restore() platform.Restoration {
	if receipt.restore == nil {
		return nil
	}
	return receipt.restore()
}

// apply writes the transition's files in the order that makes a failure
// recoverable: the settings first, then the record of the keys that AIGW
// newly owns, and the first record last, so that a failure at any step gives
// back what the steps before it wrote. A refusal has planned none of them.
func (change settingsChange) apply() (SettingsReceipt, error) {
	var written []restorationFile
	var settingsAfter platform.FileSnapshot
	var err error
	settingsAction := "write"
	if change.plan.Action == SettingsActionRestore {
		settingsAction = "restore"
	}
	if change.removeSettings {
		settingsAfter, err = removeGuarded(change.path, change.settingsBefore)
	} else {
		settingsAfter, err = writeGuarded(change.path, change.settingsBefore, change.settingsData, 0o600)
	}
	if err != nil {
		return SettingsReceipt{}, fmt.Errorf("%s Claude settings: %w", settingsAction, err)
	}
	written = append(written, restorationFile{path: change.path, before: change.settingsBefore, after: settingsAfter})
	keysAction := "write"
	var keysAfter platform.FileSnapshot
	switch change.planKeys {
	case keysUnchanged:
		// The record of owned keys stays as it is.
	case keysWritten:
		keysAfter, err = writeGuarded(change.keysPath, change.keysBefore, change.keysData, 0o600)
	case keysRemoved:
		keysAction = "remove"
		keysAfter, err = removeGuarded(change.keysPath, change.keysBefore)
	}
	if err != nil {
		return SettingsReceipt{}, platform.FailedWrite(fmt.Errorf("%s Claude settings keys: %w", keysAction, err), undoWritten(written))
	}
	if change.planKeys != keysUnchanged {
		written = append(written, restorationFile{path: change.keysPath, before: change.keysBefore, after: keysAfter})
	}
	stateAction := "write"
	if change.plan.Action == SettingsActionRestore {
		stateAction = "remove"
	}
	var stateAfter platform.FileSnapshot
	if change.plan.Action == SettingsActionRestore {
		stateAfter, err = removeGuarded(change.statePath, change.stateBefore)
	} else {
		stateAfter, err = writeGuarded(change.statePath, change.stateBefore, change.stateData, 0o600)
	}
	if err != nil {
		return SettingsReceipt{}, platform.FailedWrite(fmt.Errorf("%s Claude settings state: %w", stateAction, err), undoWritten(written))
	}
	written = append(written, restorationFile{path: change.statePath, before: change.stateBefore, after: stateAfter})
	return SettingsReceipt{SettingsPlan: change.plan, restore: func() platform.Restoration { return undoWritten(written) }}, nil
}
