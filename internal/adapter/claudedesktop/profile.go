package claudedesktop

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// managedPolicy holds AIGW's fixed profile settings. A setting with
// desktopDefault may be omitted because Claude Desktop applies the same value
// to an omitted key, and its configuration editor omits it when it saves the
// profile: Allow Cowork and Allow Code default to true, and a gateway without
// the x-api-key scheme uses bearer (Claude Desktop 3.1623.0).
var managedPolicy = []struct {
	key, label     string
	value          json.RawMessage
	desktopDefault bool
}{
	{"chatTabEnabled", "Allow Chat", raw(true), false},
	{"coworkTabEnabled", "Allow Cowork", raw(true), true},
	{"inferenceCredentialKind", "credential kind", raw("helper-script"), false},
	{"inferenceGatewayAuthScheme", "gateway authentication scheme", raw("bearer"), true},
	{"inferenceProvider", "inference provider", raw("gateway"), false},
	{"isClaudeCodeForDesktopEnabled", "Allow Code", raw(true), true},
	{"modelDiscoveryEnabled", "model discovery", raw(false), false},
}

// requestKeys carry the projection request; every key outside requestKeys and
// managedPolicy belongs to Claude Desktop.
var requestKeys = []string{"inferenceCredentialHelper", "inferenceCredentialHelperArgs", "inferenceGatewayBaseUrl", "inferenceModels", "toolSearchEnabled"}

// readManagedProfile reads the projection request that a profile carries and
// names each managed setting that no longer holds AIGW's value.
func readManagedProfile(profile document) (Desired, []string) {
	var changed []string
	for _, setting := range managedPolicy {
		value, present := profile[setting.key]
		if !present && setting.desktopDefault {
			continue
		}
		if !present || !sameJSON(value, setting.value) {
			changed = append(changed, setting.label)
		}
	}
	var request Desired
	var models []struct {
		Name  string `json:"name"`
		Label string `json:"labelOverride"`
	}
	for _, field := range []struct {
		key, label string
		target     any
	}{
		{"inferenceGatewayBaseUrl", "gateway base URL", &request.BaseURL},
		{"inferenceCredentialHelper", "credential helper", &request.CredentialExecutable},
		{"inferenceCredentialHelperArgs", "credential helper arguments", &request.CredentialArguments},
		{"inferenceModels", "models", &models},
		{"toolSearchEnabled", "tool search", &request.ToolSearch},
	} {
		value, present := profile[field.key]
		if !present && field.key == "toolSearchEnabled" {
			continue
		}
		if !present || json.Unmarshal(value, field.target) != nil {
			changed = append(changed, field.label)
		}
	}
	for _, model := range models {
		request.Models = append(request.Models, Model{Name: model.Name, Label: model.Label})
	}
	return request, changed
}

// selectionChanges names each Claude Desktop selection that no longer applies
// the AIGW profile.
func selectionChanges(standard, thirdParty, metadata document) []string {
	var changed []string
	if !sameJSON(standard["deploymentMode"], raw("3p")) || !sameJSON(thirdParty["deploymentMode"], raw("3p")) {
		changed = append(changed, "Claude Desktop deployment mode")
	}
	if !sameJSON(metadata["appliedId"], raw(profileID)) {
		changed = append(changed, "selected Claude Desktop profile")
	}
	entry, _ := json.Marshal(map[string]string{"id": profileID, "name": profileName})
	entries, err := metadataEntries(metadata)
	if err != nil || !slices.ContainsFunc(entries, func(candidate json.RawMessage) bool { return sameJSON(candidate, entry) }) {
		changed = append(changed, "Claude Desktop profile entry")
	}
	return changed
}

// equivalentProfile reports whether a profile that differs from the recorded
// bytes still carries AIGW's managed meaning. A record from an earlier release
// holds only the digest of AIGW's own encoding, so the profile's request is
// encoded again and compared by digest.
func equivalentProfile(state ownershipState, standard, thirdParty, metadata document, profile []byte) (bool, []string) {
	changed := selectionChanges(standard, thirdParty, metadata)
	var decoded document
	if json.Unmarshal(profile, &decoded) != nil || decoded == nil {
		return false, nil
	}
	request, policy := readManagedProfile(decoded)
	if changed = append(changed, policy...); len(changed) != 0 {
		return false, changed
	}
	if state.ManagedRequest != nil {
		changed = requestChanges(*state.ManagedRequest, request)
		return len(changed) == 0, changed
	}
	encoded, err := encodeProfile(request)
	if err != nil {
		return false, nil
	}
	entry, _ := json.Marshal(map[string]string{"id": profileID, "name": profileName})
	modes := document{"deploymentMode": raw("3p")}
	recorded, err := managedHash(profileID, modes, modes, encoded, document{"appliedId": raw(profileID), "entries": raw([]json.RawMessage{entry})})
	return err == nil && recorded == state.ManagedSHA256, nil
}

func requestChanges(recorded, current Desired) []string {
	var changed []string
	for _, field := range []struct {
		label string
		same  bool
	}{
		{"gateway base URL", strings.TrimRight(recorded.BaseURL, "/") == strings.TrimRight(current.BaseURL, "/")},
		{"credential helper", recorded.CredentialExecutable == current.CredentialExecutable},
		{"credential helper arguments", slices.Equal(recorded.CredentialArguments, current.CredentialArguments)},
		{"models", slices.Equal(recorded.Models, current.Models)},
		{"tool search", recorded.ToolSearch == current.ToolSearch},
	} {
		if !field.same {
			changed = append(changed, field.label)
		}
	}
	return changed
}

// projectedProfile keeps a current profile that already carries the request,
// merges a changed request into Claude Desktop's own settings, or encodes a
// new profile.
func projectedProfile(path string, before snapshots, desired Desired) ([]byte, error) {
	owned, err := profileDocument(desired)
	if err != nil {
		return nil, err
	}
	if !before.state.Exists || !before.profile.Exists {
		return encode(owned)
	}
	merged, err := decodeObject(path, before.profile)
	if err != nil {
		return nil, err
	}
	current, changed := readManagedProfile(merged)
	if len(changed) == 0 && len(requestChanges(desired, current)) == 0 {
		return slices.Clone(before.profile.Data), nil
	}
	for _, setting := range managedPolicy {
		delete(merged, setting.key)
	}
	for _, key := range requestKeys {
		delete(merged, key)
	}
	maps.Copy(merged, owned)
	return encode(merged)
}

// recordOwnership returns the ownership record for a projection. Its digest
// covers the files as they will be on disk, which earlier releases compare,
// and it records the request for later comparisons by meaning.
func recordOwnership(state ownershipState, projected projectedFiles, before snapshots, desired Desired) ([]byte, error) {
	onDisk, err := encodeDocuments(projected, before)
	if err != nil {
		return nil, err
	}
	decoded := make([]document, len(onDisk))
	for index, data := range onDisk {
		if err := json.Unmarshal(data, &decoded[index]); data != nil && err != nil {
			return nil, err
		}
	}
	state.ManagedSHA256, err = managedHash(profileID, decoded[0], decoded[1], projected.profile, decoded[2])
	if err != nil {
		return nil, err
	}
	desired.BaseURL = strings.TrimRight(desired.BaseURL, "/")
	state.ManagedRequest = &desired
	return encode(state)
}

func changedOutsideAIGW(settings []string) error {
	if len(settings) == 0 {
		return errors.New("managed Claude Desktop configuration changed outside AIGW; refusing to overwrite user edits; restore the AIGW profile in Claude Desktop, then run `aigw sync`")
	}
	return fmt.Errorf("managed Claude Desktop configuration changed outside AIGW (%s); refusing to overwrite user edits; restore those settings in Claude Desktop, then run `aigw sync`", strings.Join(settings, ", "))
}

func sameJSON(left, right json.RawMessage) bool {
	leftCanonical, leftOK := canonicalJSON(left)
	rightCanonical, rightOK := canonicalJSON(right)
	return leftOK && rightOK && bytes.Equal(leftCanonical, rightCanonical)
}
