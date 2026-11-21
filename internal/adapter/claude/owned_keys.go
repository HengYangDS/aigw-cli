package claude

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/platform"
)

// ownedKeysSuffix names the record of the settings keys that AIGW owns
// beside each client file that it owns. It is a record of its own because the
// first record beside a client file is a published contract with a strict
// reader, while this one keeps an entry that its own release does not know,
// so a later release can own one more settings key without breaking an
// earlier one.
const ownedKeysSuffix = ".aigw-owned-keys.json"

// ownedKeyModelOverrides is the settings key that this release newly owns.
const ownedKeyModelOverrides = "modelOverrides"

// ownedKeysRecord is the record of owned settings keys. Keys holds each
// entry as it was read, so an entry of a later release is written back as it
// was and its settings key is neither read nor written.
type ownedKeysRecord struct {
	Version  int                        `json:"version"`
	WriterID string                     `json:"writer_id"`
	Keys     map[string]json.RawMessage `json:"keys"`
}

// ownedKeyEntry is one entry of the record: what the settings key held
// before AIGW wrote it, and a digest of what AIGW wrote. The digest is taken
// over the value's canonical JSON, because a previous release re-indents the
// file without changing the value.
type ownedKeyEntry struct {
	Original      optionalValue `json:"original"`
	WrittenSHA256 string        `json:"written_sha256"`
}

// ownedKeyAction is what a transition does with one owned settings key.
type ownedKeyAction int

const (
	// ownedKeyLeave leaves a key that is not AIGW's as it is.
	ownedKeyLeave ownedKeyAction = iota
	// ownedKeyWrite writes the selection's value and records its digest.
	ownedKeyWrite
	// ownedKeyRestore returns the settings to the value that AIGW recorded.
	ownedKeyRestore
)

// ownedKeysPath returns the record of owned settings keys beside the client
// file at path.
func ownedKeysPath(path string) string { return path + ownedKeysSuffix }

// decideOwnedKey is the design's table, the one rule that both a projection
// and a withdrawal follow: it says what to do with current, the file's value
// of the key, when desired is the value the selection needs, and entry is
// what the record holds for the key. A current value that is neither what
// AIGW wrote, nor what it is about to write, nor the recorded original is an
// outside edit, which the transition refuses.
func decideOwnedKey(current, desired, entry json.RawMessage) (ownedKeyAction, error) {
	recorded, err := decodeOwnedKeyEntry(entry)
	if err != nil {
		return ownedKeyLeave, err
	}
	desiredPresent := len(desired) > 0
	switch {
	case len(entry) == 0 && !desiredPresent:
		return ownedKeyLeave, nil
	case len(entry) == 0:
		return ownedKeyWrite, nil
	}
	if !desiredPresent {
		// A previous release re-indents every value without changing it, and
		// a projection takes the map off only to write the same one again
		// while it records it, so the current value may be either what AIGW
		// recorded before or what it wrote last. A value that is neither is an
		// outside edit, which this refuses.
		if valueDigest(current) == recorded.WrittenSHA256 || sameJSON(entryValue(recorded.Original), current) {
			return ownedKeyRestore, nil
		}
		return ownedKeyLeave, outsideEdit("managed Claude settings changed outside AIGW; refusing to overwrite user edits")
	}
	if sameJSON(desired, current) || valueDigest(current) == recorded.WrittenSHA256 {
		return ownedKeyWrite, nil
	}
	return ownedKeyLeave, outsideEdit("managed Claude settings changed outside AIGW; refusing to overwrite user edits")
}

// decodeOwnedKeyEntry decodes one entry of the record. A record that holds an
// entry which is not an object of this shape is a record this program cannot
// read; a field that this release does not know is kept as the entry was
// read.
func decodeOwnedKeyEntry(entry json.RawMessage) (ownedKeyEntry, error) {
	var recorded ownedKeyEntry
	if len(entry) == 0 {
		return recorded, nil
	}
	if err := json.Unmarshal(entry, &recorded); err != nil {
		return ownedKeyEntry{}, fmt.Errorf("parse the entry of an owned settings key: %w", err)
	}
	return recorded, nil
}

// desiredOwnedKey returns the value that the selection needs for the map: nil,
// which writes and checks nothing, when the selection needs no map.
func desiredOwnedKey(wanted map[string]string) json.RawMessage {
	if len(wanted) == 0 {
		return nil
	}
	return adapter.RawJSON(wanted)
}

// ownedKeysForProjection follows the design's table for a projection: it
// decides what happens to key, records what AIGW wrote, and returns the map's
// preimage so that the caller may write it back when the map is not the
// selection's to own. The record keeps what the settings held before AIGW
// wrote the map, or what an entry AIGW wrote before recorded first, so a
// later projection does not take AIGW's own map for the user's.
func ownedKeysForProjection(document settingsDocument, key string, desired json.RawMessage, record ownedKeysRecord) (ownedKeysRecord, ownedKeyAction, error) {
	action, err := decideOwnedKey(document[key], desired, record.Keys[key])
	if err != nil {
		return record, ownedKeyLeave, err
	}
	if action != ownedKeyLeave {
		record.Keys = refreshedOwnedKeys(record, key, action, document[key], desired)
	}
	return record, action, nil
}

// projectedOwnedKey writes what the settings hold for key after action: the
// selection's value or the original that the entry recorded.
func projectedOwnedKey(key string, document settingsDocument, action ownedKeyAction, desired, entry json.RawMessage) {
	switch action {
	case ownedKeyWrite:
		document[key] = append(json.RawMessage(nil), desired...)
	case ownedKeyRestore:
		recorded, _ := decodeOwnedKeyEntry(entry)
		restoreOptional(document, key, recorded.Original)
	}
}

// refreshedOwnedKeys returns the record as this transition writes it: an
// entry for key that AIGW wrote, no entry once the key is back to the original
// that its entry recorded, an unchanged entry that this release does not know,
// and nil when no entry remains. An entry that AIGW wrote before keeps the
// original that it recorded first, so a later projection does not take AIGW's
// own map for the user's.
func refreshedOwnedKeys(record ownedKeysRecord, key string, action ownedKeyAction, original, written json.RawMessage) map[string]json.RawMessage {
	keys := make(map[string]json.RawMessage, len(record.Keys))
	for name, entry := range record.Keys {
		keys[name] = entry
	}
	switch action {
	case ownedKeyWrite:
		// The digest is taken over the value's canonical JSON, so that the same
		// map written again, or re-indented by a previous release, keeps its
		// entry.
		entry := ownedKeyEntry{Original: captureOptionalValue(original), WrittenSHA256: valueDigest(written)}
		if previous, kept := record.Keys[key]; kept {
			// The entry recorded what the settings held before AIGW first
			// wrote the key, which the settings may by now hold AIGW's own
			// value at. It is kept even when that was no value: then the key,
			// once no map covers it, goes rather than holding an empty map.
			if earlier, err := decodeOwnedKeyEntry(previous); err == nil {
				entry.Original = earlier.Original
			}
		}
		data, _ := json.Marshal(entry)
		keys[key] = data
	case ownedKeyRestore:
		delete(keys, key)
	}
	return keys
}

// normalizedRecord drops an entry that a transition leaves empty, so that the
// record of no entry is the record that does not exist.
func normalizedRecord(keys map[string]json.RawMessage) map[string]json.RawMessage {
	if len(keys) == 0 {
		return nil
	}
	return keys
}

// ownedKeysDisposition is what a transition does with the record of owned
// settings keys.
type ownedKeysDisposition int

const (
	// keysUnchanged leaves the record as it is.
	keysUnchanged ownedKeysDisposition = iota
	// keysWritten writes the record.
	keysWritten
	// keysRemoved removes the record.
	keysRemoved
)

// ownedKeysForWrite plans the record for keys: it is written when an entry
// changed, removed when no entry remains and the record exists, and left
// otherwise.
func ownedKeysForWrite(before platform.FileSnapshot, keys map[string]json.RawMessage) ownedKeysDisposition {
	if len(keys) == 0 {
		if before.Exists {
			return keysRemoved
		}
		return keysUnchanged
	}
	record := ownedKeysRecord{Version: 1, WriterID: "aigw-cli", Keys: keys}
	data, _ := adapter.EncodeJSON(record)
	if before.Exists && sameJSON(before.Data, data) {
		return keysUnchanged
	}
	return keysWritten
}

// ownedKeysData encodes the record for keys.
func ownedKeysData(keys map[string]json.RawMessage) []byte {
	record := ownedKeysRecord{Version: 1, WriterID: "aigw-cli", Keys: keys}
	data, _ := adapter.EncodeJSON(record)
	return data
}

// captureOptionalValue reports raw as an optional value, as the settings
// record does.
func captureOptionalValue(raw json.RawMessage) optionalValue {
	if len(raw) == 0 {
		return optionalValue{}
	}
	return optionalValue{Present: true, Value: append(json.RawMessage(nil), raw...)}
}

// entryValue is the raw value that an optional value holds.
func entryValue(value optionalValue) json.RawMessage {
	if !value.Present {
		return nil
	}
	return value.Value
}

// sameJSON reports whether two raw JSON values are equal regardless of their
// formatting or the order of their object keys.
func sameJSON(left, right json.RawMessage) bool {
	if len(left) == 0 || len(right) == 0 {
		return len(left) == 0 && len(right) == 0
	}
	var leftValue, rightValue any
	if json.Unmarshal(left, &leftValue) != nil || json.Unmarshal(right, &rightValue) != nil {
		return bytes.Equal(bytes.TrimSpace(left), bytes.TrimSpace(right))
	}
	leftCanonical, _ := json.Marshal(leftValue)
	rightCanonical, _ := json.Marshal(rightValue)
	return bytes.Equal(leftCanonical, rightCanonical)
}

// valueDigest is the digest that the record keeps of a written value: its
// canonical JSON, so that a rewrite that only re-indents the file does not
// change it.
func valueDigest(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return platform.SHA256(raw)
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return platform.SHA256(raw)
	}
	return platform.SHA256(canonical)
}

// readOwnedKeys decodes the record beside the client file at path. A record
// without its version or writer, or with one this program does not know, is
// refused as unreadable, like the first record of the same file, because only
// the record holds what AIGW replaced. An entry's own shape is fixed too; one
// that this program cannot read is unreadable, never silently dropped.
func readOwnedKeys(path string, before platform.FileSnapshot) (ownedKeysRecord, error) {
	record := ownedKeysRecord{Version: 1, WriterID: "aigw-cli"}
	if !before.Exists {
		return record, nil
	}
	unreadable := func(cause error) error {
		return adapter.UnreadableRecord(ownedKeysPath(path), cause)
	}
	var header struct {
		Version  *int                       `json:"version"`
		WriterID *string                    `json:"writer_id"`
		Keys     map[string]json.RawMessage `json:"keys"`
	}
	if err := json.Unmarshal(before.Data, &header); err != nil {
		return ownedKeysRecord{}, unreadable(fmt.Errorf("parse Claude owned settings keys: %w", err))
	}
	if header.Version == nil || *header.Version != 1 || header.WriterID == nil || *header.WriterID != "aigw-cli" {
		return ownedKeysRecord{}, unreadable(errors.New("the record of owned settings keys is incomplete or not owned by AIGW"))
	}
	record.Keys = header.Keys
	for _, entry := range record.Keys {
		if _, err := decodeOwnedKeyEntry(entry); err != nil {
			return ownedKeysRecord{}, unreadable(err)
		}
	}
	return record, nil
}
