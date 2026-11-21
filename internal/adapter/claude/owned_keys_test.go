package claude

import (
	"bytes"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
)

// readFileBytes reads a file, or returns nil when it is absent.
func readFileBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// readOwnedKeysFile reads the record of settings keys that AIGW newly owns, or
// nil when the file is absent.
func readOwnedKeysFile(t *testing.T, path string) map[string]json.RawMessage {
	t.Helper()
	data := readFileBytes(t, ownedKeysPath(path))
	if data == nil {
		return nil
	}
	var record struct {
		Version  int                        `json:"version"`
		WriterID string                     `json:"writer_id"`
		Keys     map[string]json.RawMessage `json:"keys"`
	}
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("the record of owned keys is not JSON: %s", data)
	}
	if record.Version != 1 || record.WriterID != "aigw-cli" {
		t.Fatalf("the record of owned keys is %s", data)
	}
	return record.Keys
}

// sameDocument reports whether two settings documents hold the same values,
// whatever their formatting or key order.
func sameDocument(left, right settingsDocument) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if !sameJSON(value, right[key]) {
			return false
		}
	}
	return true
}

// canonicalDigest is the digest of a JSON value's canonical encoding, which is
// what the record of owned keys holds: the predecessor re-indents the file
// without changing the value.
func canonicalDigest(t *testing.T, raw []byte) string {
	t.Helper()
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("digest of %s: %v", raw, err)
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return platform.SHA256(canonical)
}

// requireOwnedMapDigest requires the record's entry for modelOverrides to
// digest the map that the settings file holds.
func requireOwnedMapDigest(t *testing.T, path string) {
	t.Helper()
	entry, present := readOwnedKeysFile(t, path)["modelOverrides"]
	if !present {
		t.Fatalf("no entry for modelOverrides after a projection with a map")
	}
	var observed struct {
		WrittenSHA256 string `json:"written_sha256"`
	}
	if err := json.Unmarshal(entry, &observed); err != nil {
		t.Fatal(err)
	}
	if want := canonicalDigest(t, readSettingsFile(t, path)["modelOverrides"]); observed.WrittenSHA256 != want {
		t.Fatalf("written_sha256 = %q, want the digest of the written map %q", observed.WrittenSHA256, want)
	}
}

// The design's table decides every transition of the map and its record:
// each case names the row that it holds.
func TestOwnedSettingsKeysFollowTheDesignTable(t *testing.T) {
	userMap := `{"theme":"dark","modelOverrides":{"claude-opus-4-8":"my-opus"}}`
	for _, test := range []struct {
		name    string
		row     string
		before  string
		current string
		entry   string
		runtime func() domain.Runtime
		refuse  bool
	}{
		{name: "no entry, no map, any value", row: "one", before: userMap, runtime: withoutMapRuntime},
		{name: "no entry, some, absent", row: "two", before: `{"theme":"dark"}`, runtime: overridesRuntime},
		{name: "no entry, some, the file holds a map", row: "two", before: userMap, runtime: overridesRuntime},
		{name: "entry, some, what AIGW wrote", row: "three", before: userMap, current: `{"claude-opus-4-8":"aigw-opus"}`, entry: `{"original":{"present":true,"value":{"claude-opus-4-8":"my-opus"}},"written_sha256":"%s"}`, runtime: overridesRuntime},
		{name: "entry, none, what AIGW wrote", row: "four", before: userMap, current: `{"claude-opus-4-8":"aigw-opus"}`, entry: `{"original":{"present":true,"value":{"claude-opus-4-8":"my-opus"}},"written_sha256":"%s"}`, runtime: withoutMapRuntime},
		{name: "entry, some, changed outside AIGW", row: "five", before: userMap, entry: `{"original":{"present":true,"value":{"claude-opus-4-8":"my-opus"}},"written_sha256":"%s"}`, runtime: overridesRuntime, refuse: true},
		{name: "entry, none, a value changed outside AIGW", row: "five", before: userMap, entry: `{"original":{"present":true,"value":{"claude-opus-4-8":"some-other-opus"}},"written_sha256":"%s"}`, runtime: withoutMapRuntime, refuse: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			if err := os.WriteFile(path, []byte(test.before), 0o600); err != nil {
				t.Fatal(err)
			}
			if test.current != "" {
				document := readSettingsFile(t, path)
				document["modelOverrides"] = json.RawMessage(test.current)
				if err := os.WriteFile(path, settingsJSON(t, document), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if test.entry != "" {
				// The entry records a map that AIGW wrote, which is neither
				// the user's map in the file nor the selection's.
				written := adapter.RawJSON(map[string]string{"claude-opus-4-8": "aigw-opus"})
				record := `{"version":1,"writer_id":"aigw-cli","keys":{"modelOverrides":` +
					strings.Replace(test.entry, "%s", canonicalDigest(t, written), 1) + `}}`
				if err := os.WriteFile(ownedKeysPath(path), []byte(record), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			settingsBefore, keysBefore := readFileBytes(t, path), readFileBytes(t, ownedKeysPath(path))
			_, err := ReconcileSettings(path, false, test.runtime(), testExecutable(), test.runtime().UpstreamModel)
			if test.refuse {
				if err == nil || !strings.Contains(err.Error(), "managed Claude settings changed") {
					t.Fatalf("row five: error = %v, want the managed-drift refusal", err)
				}
				if !bytes.Equal(readFileBytes(t, path), settingsBefore) || !bytes.Equal(readFileBytes(t, ownedKeysPath(path)), keysBefore) {
					t.Fatal("row five: the refusal wrote a file")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			overrides, present := writtenOverrides(t, path)
			want := modelOverrides(test.runtime())
			if test.row == "one" || test.row == "four" {
				want = map[string]string{"claude-opus-4-8": "my-opus"}
			}
			if present != (len(want) > 0 || test.row == "two") || !maps.Equal(overrides, want) {
				t.Fatalf("row %s: modelOverrides = %v (present %t), want %v", test.row, overrides, present, want)
			}
			if test.row != "one" && test.row != "four" {
				requireOwnedMapDigest(t, path)
			}
			// The same projection again changes nothing.
			settings, keys := readFileBytes(t, path), readFileBytes(t, ownedKeysPath(path))
			plan, err := PlanSettings(path, false, test.runtime(), testExecutable(), test.runtime().UpstreamModel)
			if err != nil || plan.Action != SettingsActionAlreadyConverged {
				t.Fatalf("row %s: the projection again plans %q, %v; want already converged", test.row, plan.Action, err)
			}
			if !bytes.Equal(readFileBytes(t, path), settings) || !bytes.Equal(readFileBytes(t, ownedKeysPath(path)), keys) {
				t.Fatal("the projection again rewrote a file")
			}
		})
	}

	// Row three with the entry that a projection wrote itself: a selection
	// whose map changed replaces the map that AIGW wrote, and the entry keeps
	// the original that the first projection recorded, not AIGW's earlier map.
	t.Run("entry, some, the map of an earlier selection", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "settings.json")
		if err := os.WriteFile(path, []byte(userMap), 0o600); err != nil {
			t.Fatal(err)
		}
		usersMap := readSettingsFile(t, path)["modelOverrides"]
		earlier, changed := overridesRuntime(), anotherChannelRuntime()
		if maps.Equal(modelOverrides(earlier), modelOverrides(changed)) {
			t.Fatal("the two selections need the same map, so the projection has nothing to replace")
		}
		if _, err := ReconcileSettings(path, false, earlier, testExecutable(), ""); err != nil {
			t.Fatal(err)
		}
		if original := recordedOriginal(t, path); !original.Present || !sameJSON(original.Value, usersMap) {
			t.Fatalf("the first projection recorded the original %s, want the user's map %s", original.Value, usersMap)
		}
		if _, err := ReconcileSettings(path, false, changed, testExecutable(), earlier.UpstreamModel); err != nil {
			t.Fatal(err)
		}
		if got, present := writtenOverrides(t, path); !present || !maps.Equal(got, modelOverrides(changed)) {
			t.Fatalf("row three: modelOverrides = %v (present %t), want the changed selection's %v", got, present, modelOverrides(changed))
		}
		requireOwnedMapDigest(t, path)
		if original := recordedOriginal(t, path); !original.Present || !sameJSON(original.Value, usersMap) {
			t.Fatalf("row three: the entry's original became %s, want the user's map %s unchanged", original.Value, usersMap)
		}
	})
}

// recordedOriginal reads what the record's entry for modelOverrides says the
// settings held before AIGW wrote the map.
func recordedOriginal(t *testing.T, path string) optionalValue {
	t.Helper()
	entry, present := readOwnedKeysFile(t, path)["modelOverrides"]
	if !present {
		t.Fatal("no entry for modelOverrides after a projection with a map")
	}
	recorded, err := decodeOwnedKeyEntry(entry)
	if err != nil {
		t.Fatal(err)
	}
	return recorded.Original
}

// The verification copy carries the record of owned keys, so that the copied
// projection knows what AIGW replaced in the map as well as in the settings.
func TestCopyProjectionCarriesTheRecordOfOwnedKeys(t *testing.T) {
	source := filepath.Join(t.TempDir(), "settings.json")
	if _, err := ReconcileSettings(source, false, overridesRuntime(), testExecutable(), ""); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "settings.json")
	if err := CopyProjection(source, target); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"", adapter.StateSuffix, ownedKeysSuffix} {
		want, err := os.ReadFile(source + suffix)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := os.ReadFile(target + suffix); err != nil || !bytes.Equal(got, want) {
			t.Fatalf("the copy of %s = %s, %v; want %s", filepath.Base(target+suffix), got, err, want)
		}
	}
}

// An entry that this program does not know is kept when the record is
// written, its settings key is neither read nor written, and the record is
// not removed at a withdrawal while the entry remains.
func TestOwnedSettingsKeysKeepAnUnknownEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	stranger := `{"original":{"present":false},"written_sha256":"feed","later":{"field":1}}`
	record := "{\n  \"version\": 1,\n  \"writer_id\": \"aigw-cli\",\n  \"keys\": {\n    \"modelContextLimits\": " + stranger + "\n  }\n}\n"
	if err := os.WriteFile(ownedKeysPath(path), []byte(record), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"theme":"dark","modelContextLimits":{"some":"value"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, runtime := range []domain.Runtime{overridesRuntime(), withoutMapRuntime()} {
		if _, err := ReconcileSettings(path, false, runtime, testExecutable(), ""); err != nil {
			t.Fatal(err)
		}
		if got := readSettingsFile(t, path)["modelContextLimits"]; !sameJSON(got, json.RawMessage(`{"some":"value"}`)) {
			t.Fatalf("an entry this program does not know changed its key: %s", got)
		}
		var want, got any
		if err := json.Unmarshal([]byte(stranger), &want); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(readOwnedKeysFile(t, path)["modelContextLimits"], &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("the record rewrote an entry that this program does not know: %s", readFileBytes(t, ownedKeysPath(path)))
		}
	}
	if _, err := ReconcileSettings(path, true, domain.Runtime{}, "", ""); err != nil {
		t.Fatal(err)
	}
	if got := readSettingsFile(t, path)["modelContextLimits"]; !sameJSON(got, json.RawMessage(`{"some":"value"}`)) {
		t.Fatalf("the withdrawal changed the key of an entry it does not know: %s", got)
	}
	if _, kept := readOwnedKeysFile(t, path)["modelContextLimits"]; !kept {
		t.Fatal("the withdrawal removed the record while an entry it does not know remains")
	}
}

// 0.8.3 re-indents the settings on every rewrite. The map stays AIGW's: its
// digest is taken over the value, not the bytes, and the keys' order does not
// matter.
func TestOwnedSettingsKeysSurviveAReindentedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	runtime := overridesRuntime()
	if _, err := ReconcileSettings(path, false, runtime, testExecutable(), ""); err != nil {
		t.Fatal(err)
	}
	reindented := strings.ReplaceAll(string(readFileBytes(t, path)), "\n  ", "\n\t")
	reindented = strings.Replace(reindented,
		"\"claude-fable-5-2\": \"claude-code-fable-5-2\",\n\t\t\"claude-opus-5-5\": \"claude-code-opus-5-5\"",
		"\"claude-opus-5-5\" : \"claude-code-opus-5-5\" , \"claude-fable-5-2\": \"claude-code-fable-5-2\"", 1)
	if err := os.WriteFile(path, []byte(reindented), 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := PlanSettings(path, false, runtime, testExecutable(), runtime.UpstreamModel)
	if err != nil || plan.Action != SettingsActionAlreadyConverged {
		t.Fatalf("a re-indented map with reordered keys plans %q, %v; want already converged", plan.Action, err)
	}
	if inspection, err := InspectSettings(path, runtime, testExecutable()); err != nil || inspection.NativeModelOverride {
		t.Fatalf("inspection of a re-indented map = %#v, %v", inspection, err)
	}
	if _, err := ReconcileSettings(path, true, domain.Runtime{}, "", ""); err != nil {
		t.Fatal(err)
	}
	if data := readFileBytes(t, path); data != nil {
		if _, present := readSettingsFile(t, path)["modelOverrides"]; present {
			t.Fatal("the withdrawal of a re-indented map kept it")
		}
	}
}

// A failed write of the record gives back what the transition wrote before
// it, in the reverse order, and leaves the first record untouched.
func TestOwnedSettingsKeysRecordWriteFailureRestoresTheOthers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"theme":"dark"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	settingsBefore := readFileBytes(t, path)
	writes := 0
	withSettingsTransaction(t, captureSnapshot,
		func(path string, before platform.FileSnapshot, data []byte, mode os.FileMode) (platform.FileSnapshot, error) {
			writes++
			if strings.HasSuffix(path, ownedKeysSuffix) {
				return platform.FileSnapshot{}, os.ErrPermission
			}
			return platform.WriteFileAtomicIfUnchanged(path, before, data, mode)
		},
		removeGuarded, restoreGuarded,
	)
	_, err := ReconcileSettings(path, false, overridesRuntime(), testExecutable(), "")
	if err == nil || !strings.Contains(err.Error(), "write Claude settings keys") {
		t.Fatalf("error = %v, want the record's write failure", err)
	}
	if writes != 2 {
		t.Fatalf("writes before the failure = %d, want the settings and the record before the first record", writes)
	}
	if got := readFileBytes(t, path); !bytes.Equal(got, settingsBefore) {
		t.Fatalf("the failed record write left the settings as %s", got)
	}
	for _, left := range []string{path + adapter.StateSuffix, ownedKeysPath(path)} {
		if _, err := os.Stat(left); !os.IsNotExist(err) {
			t.Fatalf("the failed record write left %s: %v", filepath.Base(left), err)
		}
	}
}

// A record of the second kind whose version or writer this program does not
// know is refused as unreadable, with the record's name and the fix to
// restore it.
func TestOwnedSettingsKeysRefuseAnUnknownRecord(t *testing.T) {
	for name, record := range map[string]string{
		"another version":   `{"version":2,"writer_id":"aigw-cli","keys":{}}`,
		"another writer":    `{"version":1,"writer_id":"other","keys":{}}`,
		"no version":        `{"keys":{}}`,
		"malformed":         `{`,
		"another entry key": `{"version":1,"writer_id":"aigw-cli","keys":{"modelOverrides":7}}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			if err := os.WriteFile(path, []byte(`{"theme":"dark"}`), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(ownedKeysPath(path), []byte(record), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := ReconcileSettings(path, false, overridesRuntime(), testExecutable(), "")
			if err == nil || !strings.Contains(err.Error(), "settings.json.aigw-owned-keys.json cannot be read") {
				t.Fatalf("error = %v, want the record's unreadable refusal", err)
			}
			var conflict *adapter.ConflictError
			if !errors.As(err, &conflict) || !strings.HasPrefix(conflict.NextAction(), "Restore settings.json.aigw-owned-keys.json") {
				t.Fatalf("error = %v, want the fix that restores the record", err)
			}
			if got := readFileBytes(t, path); !bytes.Equal(got, []byte(`{"theme":"dark"}`)) {
				t.Fatalf("the refusal wrote the settings: %s", got)
			}
		})
	}
}

// The inspection reads a map that differs from the selection's, or that is
// missing while the selection needs one, as convergence that `aigw sync`
// repairs; a map that the record does not own reads as the managed-drift
// refusal. A map that AIGW gave back and the user then removed is not drift:
// the entry still says what to restore.
func TestInspectionReportsTheMapAndItsRecord(t *testing.T) {
	for _, test := range []struct {
		name    string
		before  func() domain.Runtime
		now     func() domain.Runtime
		edit    func(t *testing.T, path string)
		issue   string
		repair  string
		refusal bool
		ready   bool
	}{
		{
			name:   "a map that differs from the selection's",
			before: overridesRuntime, now: opusOnlyRuntime,
			issue: "not synchronized", repair: "aigw sync",
		},
		{
			name:   "a map the selection does not need",
			before: overridesRuntime, now: withoutMapRuntime,
			edit: func(t *testing.T, path string) {
				document := readSettingsFile(t, path)
				delete(document, "modelOverrides")
				if err := os.WriteFile(path, settingsJSON(t, document), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			ready: true,
		},
		{
			name:   "a map the record does not own",
			before: opusOnlyRuntime, now: overridesRuntime,
			edit: func(t *testing.T, path string) {
				document := readSettingsFile(t, path)
				document["modelOverrides"] = adapter.RawJSON(map[string]string{"claude-opus-5-5": "someone-elses"})
				if err := os.WriteFile(path, settingsJSON(t, document), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			issue: "changed outside AIGW", repair: adapter.UndoOutsideEdit("Claude Code").Configured, refusal: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			if _, err := ReconcileSettings(path, false, test.before(), testExecutable(), ""); err != nil {
				t.Fatal(err)
			}
			if test.edit != nil {
				test.edit(t, path)
			}
			before := readFileBytes(t, path)
			_, err := InspectSettings(path, test.now(), testExecutable())
			if test.ready {
				if err != nil {
					t.Fatalf("inspection = %v, want a synchronized projection", err)
				}
				if got := readFileBytes(t, path); !bytes.Equal(got, before) {
					t.Fatal("inspection wrote the settings")
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.issue) {
				t.Fatalf("inspection = %v, want an error with %q", err, test.issue)
			}
			var conflict *adapter.ConflictError
			if errors.As(err, &conflict) != test.refusal {
				t.Fatalf("inspection = %v, want a conflict: %t", err, test.refusal)
			}
			if conflict != nil && conflict.NextAction() != test.repair {
				t.Fatalf("inspection repair = %q, want %q", conflict.NextAction(), test.repair)
			}
			if got := readFileBytes(t, path); !bytes.Equal(got, before) {
				t.Fatal("inspection wrote the settings")
			}
		})
	}
}
