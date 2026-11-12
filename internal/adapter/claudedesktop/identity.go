package claudedesktop

import (
	"encoding/json"
	"errors"
	"path/filepath"

	"aigw-cli/internal/adapter"
)

const (
	profileID   = "6500fbf3-029c-5c0d-842a-48ee47e228c5"
	profileName = "AIGW"
)

// prepareState reads AIGW's record of the profile, or starts one. withdrawing
// says whether the plan restores the user's settings.
func prepareState(paths Paths, before snapshots, standard, thirdParty, metadata document, withdrawing bool) (ownershipState, error) {
	if before.state.Exists {
		if !before.profile.Exists {
			return recordedWithoutProfile(paths, before.state.Data, standard, thirdParty, metadata, withdrawing)
		}
		return readOwnedState(paths.State, before.state.Data, standard, thirdParty, before.profile.Data, metadata)
	}
	if before.profile.Exists || hasProfileEntry(metadata, profileID) {
		return ownershipState{}, adapter.NewConflictError(
			"Claude Desktop AIGW profile already exists without AIGW ownership",
			adapter.ThenRun("Restore "+filepath.Base(paths.State)+" in "+filepath.Base(paths.Profile)+"'s folder from a copy of it, or delete "+filepath.Base(paths.Profile)+" there and remove AIGW's entry and appliedId from _meta.json"),
			nil)
	}
	return ownershipState{
		Version:  1,
		WriterID: "aigw-cli",
		Original: originalState{
			StandardExists:   before.standard.Exists,
			ThirdPartyExists: before.thirdParty.Exists,
			MetadataExists:   before.metadata.Exists,
			StandardMode:     captureValue(standard, "deploymentMode"),
			ThirdPartyMode:   captureValue(thirdParty, "deploymentMode"),
			AppliedID:        captureValue(metadata, "appliedId"),
		},
	}, nil
}

// recordedWithoutProfile reads a record whose profile file is gone. A
// withdrawal restores from the record alone, because the record holds every
// value that withdrawal puts back and the deleted profile holds nothing of the
// user's, provided the shared settings still hold AIGW's values; the
// file list keeps the user's entries, and AIGW removes only its own. A
// projection refuses, because it would write AIGW's profile again over the
// user's choice to delete it. While aigw.toml exists, disabling Claude Desktop
// withdraws; before it does, `aigw client disable` finds Claude Desktop
// disabled and changes nothing, so the profile restored is the one fix that
// keeps the settings that the record holds.
func recordedWithoutProfile(paths Paths, data []byte, standard, thirdParty, metadata document, withdrawing bool) (ownershipState, error) {
	state, err := decodeOwnedState(paths.State, data)
	if err != nil {
		return ownershipState{}, err
	}
	if withdrawing {
		if changed := selectionChanges(standard, thirdParty, metadata); len(changed) > 0 {
			return ownershipState{}, changedOutsideAIGW(changed)
		}
		return state, nil
	}
	profile, record := filepath.Base(paths.Profile), filepath.Base(paths.State)
	return ownershipState{}, adapter.NewConflictError(
		adapter.MissingWithRecord("Claude Desktop", paths.Profile, paths.State, "").Error(),
		adapter.Fix{
			Configured:   "Restore " + profile + ", or run `aigw client disable claude-desktop`, which restores the settings that AIGW changed from " + record + ", if " + profile + " is gone for good",
			Unconfigured: "Restore " + profile + " in its folder from a copy of it, then run the same command again.",
		},
		nil)
}

func decodeOwnedState(record string, data []byte) (ownershipState, error) {
	var state ownershipState
	if err := json.Unmarshal(data, &state); err != nil || state.Version != 1 || state.WriterID != "aigw-cli" {
		return ownershipState{}, adapter.UnreadableRecord(record, errors.New("Claude Desktop ownership state is unavailable or invalid"))
	}
	return state, nil
}

func readOwnedState(record string, data []byte, standard, thirdParty document, profile []byte, metadata document) (ownershipState, error) {
	state, err := decodeOwnedState(record, data)
	if err != nil {
		return ownershipState{}, err
	}
	currentHash, err := managedHash(profileID, standard, thirdParty, profile, metadata)
	if err != nil {
		return ownershipState{}, err
	}
	if currentHash == state.ManagedSHA256 {
		return state, nil
	}
	// Claude Desktop may save the profile again in its own encoding.
	equivalent, changed := equivalentProfile(state, standard, thirdParty, metadata, profile)
	if !equivalent {
		return ownershipState{}, changedOutsideAIGW(changed)
	}
	return state, nil
}
