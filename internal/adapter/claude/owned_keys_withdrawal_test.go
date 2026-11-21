package claude

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
)

// A withdrawal restores what the user's settings held before AIGW wrote the
// map and then removes the record.
func TestOwnedSettingsKeysWithdrawToTheUsersValue(t *testing.T) {
	userMap := `{"theme":"dark","modelOverrides":{"claude-opus-4-8":"my-opus"}}`
	for _, test := range []struct {
		name, before, want string
		projected          func() domain.Runtime
	}{
		{name: "a projection with a map over the user's", before: userMap, want: `{"modelOverrides":{"claude-opus-4-8":"my-opus"},"theme":"dark"}`, projected: overridesRuntime},
		{name: "a projection without a map leaves the user's map", before: userMap, want: `{"modelOverrides":{"claude-opus-4-8":"my-opus"},"theme":"dark"}`, projected: withoutMapRuntime},
		{name: "a projection with a map over settings without one", before: `{"theme":"dark"}`, want: `{"theme":"dark"}`, projected: overridesRuntime},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			if err := os.WriteFile(path, []byte(test.before), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := ReconcileSettings(path, false, test.projected(), testExecutable(), ""); err != nil {
				t.Fatal(err)
			}
			if _, err := ReconcileSettings(path, true, domain.Runtime{}, "", ""); err != nil {
				t.Fatal(err)
			}
			var want, got any
			if err := json.Unmarshal([]byte(test.want), &want); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(readFileBytes(t, path), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("the withdrawal left %s, want %s", readFileBytes(t, path), test.want)
			}
			if _, err := os.Stat(ownedKeysPath(path)); !os.IsNotExist(err) {
				t.Fatalf("the record remains after a withdrawal that gave the key back: %v", err)
			}
		})
	}
}

// The previous release withdraws the client after a rollback, which removes
// the first record and leaves the map and the record of owned keys. The
// entry alone says what to restore; when the settings then hold nothing else
// they keep an empty object, since whether the file existed went with the
// first record.
func TestOwnedSettingsKeysWithdrawWithoutTheFirstRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"modelOverrides":{"claude-opus-4-8":"my-opus"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileSettings(path, false, overridesRuntime(), testExecutable(), ""); err != nil {
		t.Fatal(err)
	}
	// What 0.8.3 leaves: no first record, and the map as it wrote it.
	if err := os.Remove(path + adapter.StateSuffix); err != nil {
		t.Fatal(err)
	}
	document := readSettingsFile(t, path)
	delete(document, "apiKeyHelper")
	delete(document, "model")
	delete(document, "env")
	if err := os.WriteFile(path, settingsJSON(t, document), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileSettings(path, true, domain.Runtime{}, "", ""); err != nil {
		t.Fatal(err)
	}
	wantDocument := settingsDocument{"modelOverrides": adapter.RawJSON(map[string]string{"claude-opus-4-8": "my-opus"})}
	if got := readSettingsFile(t, path); !sameDocument(got, wantDocument) {
		t.Fatalf("the withdrawal without the first record left %s, want the user's map", settingsJSON(t, got))
	}
	if _, err := os.Stat(ownedKeysPath(path)); !os.IsNotExist(err) {
		t.Fatalf("the record remains after the withdrawal: %v", err)
	}

	// A map that AIGW wrote into a file it created, where the user held none:
	// the restored settings keep an empty object.
	created := filepath.Join(t.TempDir(), "settings.json")
	if _, err := ReconcileSettings(created, false, overridesRuntime(), testExecutable(), ""); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(created + adapter.StateSuffix); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(created, []byte(`{"modelOverrides":`+string(adapter.RawJSON(modelOverrides(overridesRuntime())))+`}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileSettings(created, true, domain.Runtime{}, "", ""); err != nil {
		t.Fatal(err)
	}
	if got := readSettingsFile(t, created); len(got) != 0 {
		t.Fatalf("the withdrawal of a map the user never held left %s, want an empty object", settingsJSON(t, got))
	}
}

// Settings that AIGW created itself, with a map in them, leave nothing behind
// at a withdrawal: the first record owns whether the file existed, and an
// entry exists exactly while AIGW's map is in the file. The settings go with
// both records, and the receipt's rollback brings all three back as they
// were.
func TestOwnedSettingsKeysWithdrawRemovesTheSettingsThatAIGWCreated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if _, err := ReconcileSettings(path, false, overridesRuntime(), testExecutable(), ""); err != nil {
		t.Fatal(err)
	}
	if _, present := writtenOverrides(t, path); !present {
		t.Fatal("the fixture wrote no map, so the withdrawal under test is not the one this test is about")
	}
	files := []string{path, path + adapter.StateSuffix, ownedKeysPath(path)}
	projected := make([][]byte, len(files))
	for index, file := range files {
		if projected[index] = readFileBytes(t, file); projected[index] == nil {
			t.Fatalf("the projection wrote no %s, so the withdrawal has nothing of it to remove", filepath.Base(file))
		}
	}
	receipt, err := ReconcileSettings(path, true, domain.Runtime{}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			t.Fatalf("the withdrawal left %s holding %q (%v), want it removed", filepath.Base(file), readFileBytes(t, file), err)
		}
		if !slices.Contains(receipt.Effects(), file) {
			t.Fatalf("the withdrawal's effects %v do not name %s, which it removed", receipt.Effects(), filepath.Base(file))
		}
	}
	if err := receipt.Rollback(); err != nil {
		t.Fatal(err)
	}
	for index, file := range files {
		if got := readFileBytes(t, file); !bytes.Equal(got, projected[index]) {
			t.Fatalf("the rollback left %s holding %q, want the projection's %q", filepath.Base(file), got, projected[index])
		}
	}
}
