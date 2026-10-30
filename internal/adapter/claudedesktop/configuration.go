// Package claudedesktop owns AIGW's local Claude Desktop third-party profile.
package claudedesktop

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"aigw-cli/internal/platform"
)

// Action describes the observable projection transition.
type Action string

const (
	// ActionProject writes or updates the AIGW-owned profile.
	ActionProject Action = "project"
	// ActionRestore withdraws the AIGW-owned profile and restores original values.
	ActionRestore Action = "restore"
	// ActionUnchanged reports that the requested state already exists.
	ActionUnchanged Action = "unchanged"
)

// Paths identifies the complete local Claude Desktop configuration boundary.
type Paths struct {
	StandardConfig   string
	ThirdPartyConfig string
	Profile          string
	Metadata         string
	State            string
}

// PathsForLibrary derives all owned targets from Claude Desktop's configLibrary.
func PathsForLibrary(library string) Paths {
	thirdPartyRoot := filepath.Dir(library)
	standardRoot := filepath.Join(filepath.Dir(thirdPartyRoot), "Claude")
	profile := filepath.Join(library, profileID+".json")
	return Paths{
		StandardConfig:   filepath.Join(standardRoot, "claude_desktop_config.json"),
		ThirdPartyConfig: filepath.Join(thirdPartyRoot, "claude_desktop_config.json"),
		Profile:          profile,
		Metadata:         filepath.Join(library, "_meta.json"),
		State:            profile + stateSuffix,
	}
}

// Model is one explicit Claude Desktop model-picker entry.
type Model struct {
	Name  string `json:"name"`
	Label string `json:"label,omitempty"`
}

// Desired is the complete AIGW-owned Claude Desktop profile.
type Desired struct {
	BaseURL              string   `json:"base_url"`
	CredentialExecutable string   `json:"credential_executable"`
	CredentialArguments  []string `json:"credential_arguments"`
	Models               []Model  `json:"models"`
	// ToolSearch enables Desktop's host tool search for every listed model.
	ToolSearch bool `json:"tool_search,omitempty"`
}

// Plan is a prepared, side-effect-free Claude Desktop projection.
type Plan struct {
	Action                       Action `json:"action"`
	changes                      []fileChange
	observedCredentialExecutable string
}

// ChangesState reports whether applying the plan changes an owned file.
func (plan Plan) ChangesState() bool { return len(plan.changes) > 0 }

// Effects returns every file that applying the plan writes or removes.
func (plan Plan) Effects() []string {
	effects := make([]string, 0, len(plan.changes))
	for _, change := range plan.changes {
		effects = append(effects, change.path)
	}
	return effects
}

// ObservedCredentialExecutable returns the sidecar-validated managed helper.
func (plan Plan) ObservedCredentialExecutable() string {
	return plan.observedCredentialExecutable
}

// Receipt compensates one applied projection while its postimages remain unchanged.
type Receipt struct{ changes []appliedChange }

type document map[string]json.RawMessage

type optionalValue struct {
	Present bool            `json:"present"`
	Value   json.RawMessage `json:"value,omitempty"`
}

type originalState struct {
	StandardExists   bool          `json:"standard_exists"`
	ThirdPartyExists bool          `json:"third_party_exists"`
	MetadataExists   bool          `json:"metadata_exists"`
	StandardMode     optionalValue `json:"standard_mode"`
	ThirdPartyMode   optionalValue `json:"third_party_mode"`
	AppliedID        optionalValue `json:"applied_id"`
}

// ownershipState keeps version 1 and the raw-byte digest that earlier releases
// read; ManagedRequest records the projection request so that a later
// Claude Desktop save can be compared by meaning.
type ownershipState struct {
	Version        int           `json:"version"`
	WriterID       string        `json:"writer_id"`
	Original       originalState `json:"original"`
	ManagedSHA256  string        `json:"managed_sha256"`
	ManagedRequest *Desired      `json:"managed_request,omitempty"`
}

type fileChange struct {
	path   string
	before platform.FileSnapshot
	after  platform.FileSnapshot
}

type appliedChange struct {
	path string
	pre  platform.FileSnapshot
	post platform.FileSnapshot
}

type projectedFiles struct {
	standard, thirdParty document
	profile, state       []byte
	metadata             document
	legacyProfile        []byte
	legacyState          []byte
	original             originalState
}

// Prepare builds a guarded projection or withdrawal plan.
func Prepare(paths Paths, desired *Desired) (Plan, error) {
	before, err := capture(paths)
	if err != nil {
		return Plan{}, err
	}
	standard, err := decodeObject(paths.StandardConfig, before.standard)
	if err != nil {
		return Plan{}, err
	}
	thirdParty, err := decodeObject(paths.ThirdPartyConfig, before.thirdParty)
	if err != nil {
		return Plan{}, err
	}
	metadata, err := decodeObject(paths.Metadata, before.metadata)
	if err != nil {
		return Plan{}, err
	}
	if desired == nil && !before.state.Exists && !before.legacyState.Exists {
		return Plan{Action: ActionUnchanged}, nil
	}
	state, err := prepareState(before, standard, thirdParty, metadata)
	if err != nil {
		return Plan{}, err
	}

	if desired == nil {
		restoreValue(standard, "deploymentMode", state.Original.StandardMode)
		restoreValue(thirdParty, "deploymentMode", state.Original.ThirdPartyMode)
		if err := removeProfileEntries(metadata); err != nil {
			return Plan{}, err
		}
		restoreValue(metadata, "appliedId", state.Original.AppliedID)
		return buildPlan(ActionRestore, paths, before, projectedFiles{
			standard: standard, thirdParty: thirdParty, metadata: metadata, original: state.Original,
		})
	}

	standard["deploymentMode"] = raw("3p")
	thirdParty["deploymentMode"] = raw("3p")
	metadata["appliedId"] = raw(profileID)
	if err := addProfileEntry(metadata); err != nil {
		return Plan{}, err
	}
	profile, err := projectedProfile(paths.Profile, before, *desired)
	if err != nil {
		return Plan{}, err
	}
	projected := projectedFiles{standard: standard, thirdParty: thirdParty, profile: profile, metadata: metadata, original: state.Original}
	if projected.state, err = recordOwnership(state, projected, before, *desired); err != nil {
		return Plan{}, err
	}
	plan, err := buildPlan(ActionProject, paths, before, projected)
	if err != nil {
		return Plan{}, err
	}
	if before.state.Exists || before.legacyState.Exists {
		ownedProfile := before.profile
		ownedPath := paths.Profile
		if before.legacyState.Exists {
			ownedProfile = before.legacyProfile
			ownedPath, _ = legacyPaths(paths)
		}
		observed, err := decodeObject(ownedPath, ownedProfile)
		if err != nil {
			return Plan{}, err
		}
		if err := json.Unmarshal(observed["inferenceCredentialHelper"], &plan.observedCredentialExecutable); err != nil {
			return Plan{}, fmt.Errorf("decode managed Claude Desktop credential helper: %w", err)
		}
	}
	return plan, nil
}

func buildPlan(action Action, paths Paths, before snapshots, projected projectedFiles) (Plan, error) {
	encoded, err := encodeDocuments(projected, before)
	if err != nil {
		return Plan{}, err
	}
	after := []platform.FileSnapshot{
		snapshot(encoded[0], before.standard),
		snapshot(encoded[1], before.thirdParty),
		snapshot(projected.profile, before.profile),
		snapshot(encoded[2], before.metadata),
		snapshot(projected.state, before.state),
		snapshot(projected.legacyProfile, before.legacyProfile),
		snapshot(projected.legacyState, before.legacyState),
	}
	legacyProfile, legacyState := legacyPaths(paths)
	pathList := []string{paths.StandardConfig, paths.ThirdPartyConfig, paths.Profile, paths.Metadata, paths.State, legacyProfile, legacyState}
	beforeList := []platform.FileSnapshot{before.standard, before.thirdParty, before.profile, before.metadata, before.state, before.legacyProfile, before.legacyState}
	changes := make([]fileChange, 0, len(pathList))
	for index, path := range pathList {
		if beforeList[index].Equal(after[index]) {
			continue
		}
		changes = append(changes, fileChange{path: path, before: beforeList[index], after: after[index]})
	}
	// Refreshing only the ownership record leaves the projection unchanged.
	if !slices.ContainsFunc(changes, func(change fileChange) bool { return change.path != paths.State }) {
		action = ActionUnchanged
	}
	return Plan{Action: action, changes: changes}, nil
}

// encodeDocuments returns the standard, third-party and metadata bytes that a
// plan writes, keeping a current file whose JSON is already equivalent.
func encodeDocuments(projected projectedFiles, before snapshots) ([3][]byte, error) {
	var result [3][]byte
	sources := [3]document{projected.standard, projected.thirdParty, projected.metadata}
	existed := [3]bool{projected.original.StandardExists, projected.original.ThirdPartyExists, projected.original.MetadataExists}
	previous := [3]platform.FileSnapshot{before.standard, before.thirdParty, before.metadata}
	for index := range sources {
		data, err := encodeOptional(sources[index], existed[index])
		if err != nil {
			return result, err
		}
		result[index] = preserveEquivalentJSON(data, previous[index])
	}
	return result, nil
}

// Apply commits the prepared projection and returns a guarded rollback receipt.
func (plan Plan) Apply() (Receipt, error) {
	receipt := Receipt{}
	for _, change := range plan.changes {
		var post platform.FileSnapshot
		var err error
		if change.after.Exists {
			post, err = platform.WriteFileAtomicIfUnchanged(change.path, change.before, change.after.Data, change.after.Mode)
		} else {
			post, err = platform.RemoveFileIfUnchanged(change.path, change.before)
		}
		if err != nil {
			return Receipt{}, platform.CompensationFailed(err, receipt.Rollback())
		}
		receipt.changes = append(receipt.changes, appliedChange{path: change.path, pre: change.before, post: post})
	}
	return receipt, nil
}

// Rollback restores the exact preimages while every written postimage remains unchanged.
func (receipt Receipt) Rollback() error {
	var result error
	for _, change := range slices.Backward(receipt.changes) {
		result = errors.Join(result, platform.RestoreFileAtomicIfPostimage(change.path, change.pre, change.post))
	}
	return result
}

type snapshots struct {
	standard, thirdParty       platform.FileSnapshot
	profile, state             platform.FileSnapshot
	legacyProfile, legacyState platform.FileSnapshot
	metadata                   platform.FileSnapshot
}

func capture(paths Paths) (snapshots, error) {
	values := []*platform.FileSnapshot{}
	result := snapshots{}
	values = append(values, &result.standard, &result.thirdParty, &result.profile, &result.metadata, &result.state, &result.legacyProfile, &result.legacyState)
	legacyProfile, legacyState := legacyPaths(paths)
	pathList := []string{paths.StandardConfig, paths.ThirdPartyConfig, paths.Profile, paths.Metadata, paths.State, legacyProfile, legacyState}
	for index, path := range pathList {
		snapshot, err := platform.CaptureFileSnapshot(path)
		if err != nil {
			return snapshots{}, err
		}
		*values[index] = snapshot
	}
	return result, nil
}

func decodeObject(path string, snapshot platform.FileSnapshot) (document, error) {
	if !snapshot.Exists {
		return document{}, nil
	}
	var value document
	if err := json.Unmarshal(snapshot.Data, &value); err != nil || value == nil {
		return nil, fmt.Errorf("decode %s as a JSON object", path)
	}
	return value, nil
}

func encodeProfile(desired Desired) ([]byte, error) {
	profile, err := profileDocument(desired)
	if err != nil {
		return nil, err
	}
	return encode(profile)
}

func profileDocument(desired Desired) (document, error) {
	if err := desired.validate(); err != nil {
		return nil, err
	}
	models := make([]map[string]string, 0, len(desired.Models))
	for _, model := range desired.Models {
		entry := map[string]string{"name": model.Name}
		if model.Label != "" {
			entry["labelOverride"] = model.Label
		}
		models = append(models, entry)
	}
	profile := document{
		"inferenceCredentialHelper":     raw(desired.CredentialExecutable),
		"inferenceCredentialHelperArgs": raw(desired.CredentialArguments),
		"inferenceGatewayBaseUrl":       raw(strings.TrimRight(desired.BaseURL, "/")),
		"inferenceModels":               raw(models),
	}
	for _, setting := range managedPolicy {
		profile[setting.key] = slices.Clone(setting.value)
	}
	if desired.ToolSearch {
		profile["toolSearchEnabled"] = raw(true)
	}
	return profile, nil
}

func (desired Desired) validate() error {
	parsed, err := url.Parse(desired.BaseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return errors.New("Claude Desktop gateway base URL must be an absolute HTTP(S) URL")
	}
	if !filepath.IsAbs(desired.CredentialExecutable) {
		return errors.New("Claude Desktop credential helper must be an absolute path")
	}
	if len(desired.Models) == 0 || desired.Models[0].Name == "" {
		return errors.New("Claude Desktop requires at least one named model")
	}
	seen := map[string]bool{}
	for _, model := range desired.Models {
		if model.Name == "" || seen[model.Name] {
			return errors.New("Claude Desktop model names must be non-empty and unique")
		}
		seen[model.Name] = true
	}
	return nil
}

func addProfileEntry(metadata document) error {
	entries, err := metadataEntries(metadata)
	if err != nil {
		return err
	}
	entry, _ := json.Marshal(map[string]string{"id": profileID, "name": profileName})
	entries = append(removeOwnedEntries(entries, profileID, legacyRouteID), entry)
	metadata["entries"] = raw(entries)
	return nil
}

func removeProfileEntries(metadata document) error {
	entries, err := metadataEntries(metadata)
	if err != nil {
		return err
	}
	entries = removeOwnedEntries(entries, profileID, legacyRouteID)
	if len(entries) == 0 {
		delete(metadata, "entries")
	} else {
		metadata["entries"] = raw(entries)
	}
	return nil
}

func metadataEntries(metadata document) ([]json.RawMessage, error) {
	value, ok := metadata["entries"]
	if !ok {
		return nil, nil
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(value, &entries); err != nil {
		return nil, errors.New("Claude Desktop configuration metadata entries are invalid")
	}
	return entries, nil
}

func removeOwnedEntries(entries []json.RawMessage, ids ...string) []json.RawMessage {
	return slices.DeleteFunc(entries, func(entry json.RawMessage) bool {
		var value struct {
			ID string `json:"id"`
		}
		return json.Unmarshal(entry, &value) == nil && slices.Contains(ids, value.ID)
	})
}

func hasProfileEntry(metadata document, ids ...string) bool {
	entries, err := metadataEntries(metadata)
	return err == nil && len(removeOwnedEntries(slices.Clone(entries), ids...)) != len(entries)
}

func captureValue(value document, key string) optionalValue {
	rawValue, present := value[key]
	return optionalValue{Present: present, Value: slices.Clone(rawValue)}
}

func restoreValue(value document, key string, original optionalValue) {
	if original.Present {
		value[key] = slices.Clone(original.Value)
		return
	}
	delete(value, key)
}

func managedHash(id string, standard, thirdParty document, profile []byte, metadata document) (string, error) {
	entry := optionalValue{}
	entries, err := metadataEntries(metadata)
	if err != nil {
		return "", err
	}
	for _, candidate := range entries {
		var value struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(candidate, &value) == nil && value.ID == id {
			entry = optionalValue{Present: true, Value: slices.Clone(candidate)}
			break
		}
	}
	payload := struct {
		StandardMode   optionalValue `json:"standard_mode"`
		ThirdPartyMode optionalValue `json:"third_party_mode"`
		Profile        []byte        `json:"profile,omitempty"`
		AppliedID      optionalValue `json:"applied_id"`
		Entry          optionalValue `json:"entry"`
	}{captureValue(standard, "deploymentMode"), captureValue(thirdParty, "deploymentMode"), profile, captureValue(metadata, "appliedId"), entry}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func raw(value any) json.RawMessage {
	data, _ := json.Marshal(value)
	return data
}

func encode(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func encodeOptional(value document, existed bool) ([]byte, error) {
	if len(value) == 0 && !existed {
		return nil, nil
	}
	return encode(value)
}

func snapshot(data []byte, before platform.FileSnapshot) platform.FileSnapshot {
	if data == nil {
		return platform.FileSnapshot{}
	}
	mode := os.FileMode(0o600)
	if before.Exists {
		mode = before.Mode
	}
	return platform.NewFileSnapshot(data, mode)
}

func preserveEquivalentJSON(data []byte, before platform.FileSnapshot) []byte {
	if !before.Exists || data == nil {
		return data
	}
	current, currentOK := canonicalJSON(before.Data)
	desired, desiredOK := canonicalJSON(data)
	if currentOK && desiredOK && bytes.Equal(current, desired) {
		return slices.Clone(before.Data)
	}
	return data
}

func canonicalJSON(data []byte) ([]byte, bool) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, false
	}
	normalized, err := json.Marshal(value)
	return normalized, err == nil
}
