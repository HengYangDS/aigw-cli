package claudedesktop

import (
	"encoding/json"
	"errors"
)

const (
	profileID   = "6500fbf3-029c-5c0d-842a-48ee47e228c5"
	profileName = "AIGW"
)

func prepareState(before snapshots, standard, thirdParty, metadata document) (ownershipState, error) {
	if before.state.Exists {
		return readOwnedState(before.state.Data, standard, thirdParty, before.profile.Data, metadata)
	}
	if before.profile.Exists || hasProfileEntry(metadata, profileID) {
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

func readOwnedState(data []byte, standard, thirdParty document, profile []byte, metadata document) (ownershipState, error) {
	var state ownershipState
	if err := json.Unmarshal(data, &state); err != nil || state.Version != 1 || state.WriterID != "aigw-cli" {
		return ownershipState{}, errors.New("Claude Desktop ownership state is unavailable or invalid")
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
