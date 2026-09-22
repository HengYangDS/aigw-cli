package desktop

import (
	"encoding/json"
	"errors"
	"path/filepath"
)

const (
	profileID       = "6500fbf3-029c-5c0d-842a-48ee47e228c5"
	legacyProfileID = "aigw"
	profileName     = "AIGW"
	stateSuffix     = ".aigw-state.json"
)

func prepareState(before snapshots, standard, thirdParty, metadata document) (ownershipState, error) {
	if before.state.Exists && before.legacyState.Exists {
		return ownershipState{}, errors.New("multiple Claude Desktop ownership states are present")
	}
	if before.state.Exists {
		if before.legacyProfile.Exists {
			return ownershipState{}, errors.New("legacy Claude Desktop profile exists beside the current ownership state")
		}
		return readOwnedState(before.state.Data, profileID, standard, thirdParty, before.profile.Data, metadata)
	}
	if before.legacyState.Exists {
		if before.profile.Exists {
			return ownershipState{}, errors.New("current Claude Desktop profile exists beside the legacy ownership state")
		}
		return readOwnedState(before.legacyState.Data, legacyProfileID, standard, thirdParty, before.legacyProfile.Data, metadata)
	}
	if before.profile.Exists || before.legacyProfile.Exists || hasProfileEntry(metadata, profileID, legacyProfileID) {
		return ownershipState{}, errors.New("Claude Desktop AIGW profile already exists without AIGW ownership")
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

func readOwnedState(data []byte, id string, standard, thirdParty document, profile []byte, metadata document) (ownershipState, error) {
	var state ownershipState
	if err := json.Unmarshal(data, &state); err != nil || state.Version != 1 || state.WriterID != "aigw-cli" {
		return ownershipState{}, errors.New("Claude Desktop ownership state is unavailable or invalid")
	}
	currentHash, err := managedHash(id, standard, thirdParty, profile, metadata)
	if err != nil {
		return ownershipState{}, err
	}
	if currentHash != state.ManagedSHA256 {
		return ownershipState{}, errors.New("managed Claude Desktop configuration changed outside AIGW; refusing to overwrite user edits")
	}
	return state, nil
}

func legacyPaths(paths Paths) (string, string) {
	directory := filepath.Dir(paths.Profile)
	profile := filepath.Join(directory, legacyProfileID+".json")
	return profile, profile + stateSuffix
}
