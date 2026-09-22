// Package desktop owns AIGW's local Claude Desktop third-party profile.
package desktop

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

	"aigw-cli/internal/transaction"
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
	Name  string
	Label string
}

// Desired is the complete AIGW-owned Claude Desktop profile.
type Desired struct {
	BaseURL              string
	CredentialExecutable string
	CredentialArguments  []string
	Models               []Model
}

// Plan is a prepared, side-effect-free Claude Desktop projection.
type Plan struct {
	Action  Action `json:"action"`
	changes []fileChange
}

// ChangesState reports whether applying the plan changes an owned file.
func (plan Plan) ChangesState() bool { return len(plan.changes) > 0 }

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

type ownershipState struct {
	Version       int           `json:"version"`
	WriterID      string        `json:"writer_id"`
	Original      originalState `json:"original"`
	ManagedSHA256 string        `json:"managed_sha256"`
}

type fileChange struct {
	path   string
	before transaction.FileSnapshot
	after  transaction.FileSnapshot
}

type appliedChange struct {
	path string
	pre  transaction.FileSnapshot
	post transaction.FileSnapshot
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
	profile, err := encodeProfile(*desired)
	if err != nil {
		return Plan{}, err
	}
	state.ManagedSHA256, err = managedHash(profileID, standard, thirdParty, profile, metadata)
	if err != nil {
		return Plan{}, err
	}
	stateBytes, err := encode(state)
	if err != nil {
		return Plan{}, err
	}
	return buildPlan(ActionProject, paths, before, projectedFiles{
		standard: standard, thirdParty: thirdParty, profile: profile, metadata: metadata, state: stateBytes, original: state.Original,
	})
}

func buildPlan(action Action, paths Paths, before snapshots, projected projectedFiles) (Plan, error) {
	standardBytes, err := encodeOptional(projected.standard, projected.original.StandardExists)
	if err != nil {
		return Plan{}, err
	}
	thirdPartyBytes, err := encodeOptional(projected.thirdParty, projected.original.ThirdPartyExists)
	if err != nil {
		return Plan{}, err
	}
	metadataBytes, err := encodeOptional(projected.metadata, projected.original.MetadataExists)
	if err != nil {
		return Plan{}, err
	}
	standardBytes = preserveEquivalentJSON(standardBytes, before.standard)
	thirdPartyBytes = preserveEquivalentJSON(thirdPartyBytes, before.thirdParty)
	metadataBytes = preserveEquivalentJSON(metadataBytes, before.metadata)
	after := []transaction.FileSnapshot{
		snapshot(standardBytes, before.standard),
		snapshot(thirdPartyBytes, before.thirdParty),
		snapshot(projected.profile, before.profile),
		snapshot(metadataBytes, before.metadata),
		snapshot(projected.state, before.state),
		snapshot(projected.legacyProfile, before.legacyProfile),
		snapshot(projected.legacyState, before.legacyState),
	}
	legacyProfile, legacyState := legacyPaths(paths)
	pathList := []string{paths.StandardConfig, paths.ThirdPartyConfig, paths.Profile, paths.Metadata, paths.State, legacyProfile, legacyState}
	beforeList := []transaction.FileSnapshot{before.standard, before.thirdParty, before.profile, before.metadata, before.state, before.legacyProfile, before.legacyState}
	changes := make([]fileChange, 0, len(pathList))
	for index, path := range pathList {
		if beforeList[index].Equal(after[index]) {
			continue
		}
		changes = append(changes, fileChange{path: path, before: beforeList[index], after: after[index]})
	}
	if len(changes) == 0 {
		action = ActionUnchanged
	}
	return Plan{Action: action, changes: changes}, nil
}

// Apply commits the prepared projection and returns a guarded rollback receipt.
func (plan Plan) Apply() (Receipt, error) {
	receipt := Receipt{}
	for _, change := range plan.changes {
		var post transaction.FileSnapshot
		var err error
		if change.after.Exists {
			post, err = transaction.WriteFileAtomicIfUnchanged(change.path, change.before, change.after.Data, change.after.Mode)
		} else {
			post, err = transaction.RemoveFileIfUnchanged(change.path, change.before)
		}
		if err != nil {
			return Receipt{}, errors.Join(err, receipt.Rollback())
		}
		receipt.changes = append(receipt.changes, appliedChange{path: change.path, pre: change.before, post: post})
	}
	return receipt, nil
}

// Rollback restores the exact preimages while every written postimage remains unchanged.
func (receipt Receipt) Rollback() error {
	var result error
	for _, change := range slices.Backward(receipt.changes) {
		result = errors.Join(result, transaction.RestoreFileAtomicIfPostimage(change.path, change.pre, change.post))
	}
	return result
}

type snapshots struct {
	standard, thirdParty       transaction.FileSnapshot
	profile, state             transaction.FileSnapshot
	legacyProfile, legacyState transaction.FileSnapshot
	metadata                   transaction.FileSnapshot
}

func capture(paths Paths) (snapshots, error) {
	values := []*transaction.FileSnapshot{}
	result := snapshots{}
	values = append(values, &result.standard, &result.thirdParty, &result.profile, &result.metadata, &result.state, &result.legacyProfile, &result.legacyState)
	legacyProfile, legacyState := legacyPaths(paths)
	pathList := []string{paths.StandardConfig, paths.ThirdPartyConfig, paths.Profile, paths.Metadata, paths.State, legacyProfile, legacyState}
	for index, path := range pathList {
		snapshot, err := transaction.CaptureFileSnapshot(path)
		if err != nil {
			return snapshots{}, err
		}
		*values[index] = snapshot
	}
	return result, nil
}

func decodeObject(path string, snapshot transaction.FileSnapshot) (document, error) {
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
	return encode(document{
		"chatTabEnabled":                raw(true),
		"coworkTabEnabled":              raw(true),
		"inferenceCredentialHelper":     raw(desired.CredentialExecutable),
		"inferenceCredentialHelperArgs": raw(desired.CredentialArguments),
		"inferenceCredentialKind":       raw("helper-script"),
		"inferenceGatewayAuthScheme":    raw("bearer"),
		"inferenceGatewayBaseUrl":       raw(strings.TrimRight(desired.BaseURL, "/")),
		"inferenceModels":               raw(models),
		"inferenceProvider":             raw("gateway"),
		"isClaudeCodeForDesktopEnabled": raw(true),
		"modelDiscoveryEnabled":         raw(false),
	})
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
	entries = append(removeOwnedEntries(entries, profileID, legacyProfileID), entry)
	metadata["entries"] = raw(entries)
	return nil
}

func removeProfileEntries(metadata document) error {
	entries, err := metadataEntries(metadata)
	if err != nil {
		return err
	}
	entries = removeOwnedEntries(entries, profileID, legacyProfileID)
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

func snapshot(data []byte, before transaction.FileSnapshot) transaction.FileSnapshot {
	if data == nil {
		return transaction.FileSnapshot{}
	}
	mode := os.FileMode(0o600)
	if before.Exists {
		mode = before.Mode
	}
	return transaction.NewFileSnapshot(data, mode)
}

func preserveEquivalentJSON(data []byte, before transaction.FileSnapshot) []byte {
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
