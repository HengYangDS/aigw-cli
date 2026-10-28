package claudedesktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// desktopSavedProfile reproduces how Claude Desktop 3.1623.0's configuration
// editor re-saves AIGW's profile: it keeps every AIGW value, omits settings that
// equal its defaults, writes model entries name first and adds its own settings.
func desktopSavedProfile(desired Desired) []byte {
	models := make([]string, 0, len(desired.Models))
	for _, model := range desired.Models {
		models = append(models, `    {
      "name": `+quoted(model.Name)+`,
      "labelOverride": `+quoted(model.Label)+`
    }`)
	}
	arguments := make([]string, 0, len(desired.CredentialArguments))
	for _, argument := range desired.CredentialArguments {
		arguments = append(arguments, "    "+quoted(argument))
	}
	toolSearch := ""
	if desired.ToolSearch {
		toolSearch = "\n  \"toolSearchEnabled\": true,"
	}
	return []byte(`{
  "inferenceGatewayBaseUrl": ` + quoted(desired.BaseURL) + `,
  "inferenceCredentialHelper": ` + quoted(desired.CredentialExecutable) + `,
  "inferenceCredentialHelperArgs": [
` + strings.Join(arguments, ",\n") + `
  ],
  "chatTabEnabled": true,
  "desktopHome": "standard",
  "modelDiscoveryEnabled": false,
  "inferenceModels": [
` + strings.Join(models, ",\n") + `
  ],` + toolSearch + `
  "builtinBrowserEnabled": true,
  "banner": {
    "backgroundColor": "#F5F5F5",
    "textColor": "#000000",
    "enabled": false
  },
  "inferenceProvider": "gateway",
  "inferenceCredentialKind": "helper-script"
}`)
}

func quoted(text string) string {
	data, _ := json.Marshal(text)
	return string(data)
}

func desktopFixture(t *testing.T) (Paths, Desired) {
	t.Helper()
	root := t.TempDir()
	paths := PathsForLibrary(filepath.Join(root, "Claude-3p", "configLibrary"))
	desired := Desired{
		BaseURL:              "https://gateway.example.test",
		CredentialExecutable: filepath.Join(root, "aigw"),
		CredentialArguments:  []string{"credential", "claude-desktop", "fingerprint"},
		Models:               []Model{{Name: "claude-opus-5-5", Label: "Gateway · Opus 5.5"}, {Name: "claude-sonnet-5-5", Label: "Gateway · Sonnet 5.5"}},
		ToolSearch:           true,
	}
	applyProjection(t, paths, desired)
	return paths, desired
}

func readFileBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// An earlier release accepts the profile when the record's digest covers the
// files on disk.
func requireEarlierReleaseAccepts(t *testing.T, paths Paths) {
	t.Helper()
	var state ownershipState
	if err := json.Unmarshal(readFileBytes(t, paths.State), &state); err != nil {
		t.Fatal(err)
	}
	documents := make([]document, 3)
	for index, path := range []string{paths.StandardConfig, paths.ThirdPartyConfig, paths.Metadata} {
		if err := json.Unmarshal(readFileBytes(t, path), &documents[index]); err != nil {
			t.Fatal(err)
		}
	}
	hash, err := managedHash(profileID, documents[0], documents[1], readFileBytes(t, paths.Profile), documents[2])
	if err != nil || state.Version != 1 || hash != state.ManagedSHA256 {
		t.Fatalf("earlier release record = version %d %s, files %s, %v", state.Version, state.ManagedSHA256, hash, err)
	}
}

// AIGW 0.4.8 and earlier record only the digest of their own encoding.
func writeEarlierReleaseRecord(t *testing.T, paths Paths) {
	t.Helper()
	var state ownershipState
	if err := json.Unmarshal(readFileBytes(t, paths.State), &state); err != nil {
		t.Fatal(err)
	}
	state.ManagedRequest = nil
	earlier, err := encode(state)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, paths.State, earlier)
}

func editJSON(t *testing.T, path string, edit func(map[string]any)) {
	t.Helper()
	value := readJSON(t, path)
	edit(value)
	writeJSON(t, path, value)
}

func TestDesktopResaveKeepsTheProfileAndRefreshesTheRecord(t *testing.T) {
	paths, desired := desktopFixture(t)
	saved := desktopSavedProfile(desired)
	writeFile(t, paths.Profile, saved)

	plan, err := Prepare(paths, &desired)
	if err != nil {
		t.Fatalf("a profile re-saved by Claude Desktop was refused: %v", err)
	}
	if plan.Action != ActionUnchanged {
		t.Fatalf("re-saved profile action = %q, want unchanged", plan.Action)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	if got := readFileBytes(t, paths.Profile); string(got) != string(saved) {
		t.Fatalf("Claude Desktop's saved profile was rewritten:\n%s", got)
	}
	requireEarlierReleaseAccepts(t, paths)
	again, err := Prepare(paths, &desired)
	if err != nil || again.Action != ActionUnchanged || again.ChangesState() {
		t.Fatalf("second plan = %q changes %t, %v", again.Action, again.ChangesState(), err)
	}
}

func TestDesktopResaveIsAcceptedUnderAnEarlierReleaseRecord(t *testing.T) {
	paths, desired := desktopFixture(t)
	writeEarlierReleaseRecord(t, paths)
	saved := desktopSavedProfile(desired)
	writeFile(t, paths.Profile, saved)

	plan, err := Prepare(paths, &desired)
	if err != nil || plan.Action != ActionUnchanged || !plan.ChangesState() {
		t.Fatalf("earlier-release record = %q changes %t, %v", plan.Action, plan.ChangesState(), err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	if got := readFileBytes(t, paths.Profile); string(got) != string(saved) {
		t.Fatal("Claude Desktop's saved profile was rewritten")
	}
	requireEarlierReleaseAccepts(t, paths)

	// A later Desktop-owned change stays accepted once the request is recorded.
	profile := readJSON(t, paths.Profile)
	profile["builtinBrowserEnabled"] = false
	writeJSON(t, paths.Profile, profile)
	if plan, err := Prepare(paths, &desired); err != nil || plan.Action != ActionUnchanged {
		t.Fatalf("a Desktop-owned change = %q, %v", plan.Action, err)
	}
}

func TestProjectionKeepsClaudeDesktopSettings(t *testing.T) {
	paths, desired := desktopFixture(t)
	writeFile(t, paths.Profile, desktopSavedProfile(desired))
	desired.CredentialExecutable = filepath.Join(filepath.Dir(desired.CredentialExecutable), "successor", "aigw")
	desired.Models = desired.Models[:1]

	applyProjection(t, paths, desired)
	requireJSONValues(t, paths.Profile, map[string]any{
		"inferenceCredentialHelper": desired.CredentialExecutable,
		"desktopHome":               "standard",
		"builtinBrowserEnabled":     true,
	})
	requireJSONKeys(t, paths.Profile, "banner")
	requireModelNames(t, paths.Profile, []string{"claude-opus-5-5"})
	requireEarlierReleaseAccepts(t, paths)
}

func TestChangedManagedSettingsAreRefusedByName(t *testing.T) {
	tests := []struct {
		name  string
		edit  func(*testing.T, Paths)
		names []string
	}{
		{
			name: "surfaces and discovery",
			edit: func(t *testing.T, paths Paths) {
				editJSON(t, paths.Profile, func(profile map[string]any) {
					profile["coworkTabEnabled"] = false
					profile["modelDiscoveryEnabled"] = true
				})
			},
			names: []string{"Allow Cowork", "model discovery"},
		},
		{
			name: "gateway",
			edit: func(t *testing.T, paths Paths) {
				editJSON(t, paths.Profile, func(profile map[string]any) {
					profile["inferenceGatewayBaseUrl"] = "https://external-edit.example.test"
				})
			},
			names: []string{"gateway base URL"},
		},
		{
			name: "credential helper arguments",
			edit: func(t *testing.T, paths Paths) {
				editJSON(t, paths.Profile, func(profile map[string]any) {
					profile["inferenceCredentialHelperArgs"] = "credential claude-desktop fingerprint"
				})
			},
			names: []string{"credential helper arguments"},
		},
		{
			name: "models",
			edit: func(t *testing.T, paths Paths) {
				editJSON(t, paths.Profile, func(profile map[string]any) {
					profile["inferenceModels"] = []any{map[string]any{"name": "claude-haiku-5-5"}}
				})
			},
			names: []string{"models"},
		},
		{
			name: "deployment mode",
			edit: func(t *testing.T, paths Paths) {
				editJSON(t, paths.StandardConfig, func(standard map[string]any) { delete(standard, "deploymentMode") })
			},
			names: []string{"Claude Desktop deployment mode"},
		},
		{
			name: "selected profile",
			edit: func(t *testing.T, paths Paths) {
				editJSON(t, paths.Metadata, func(metadata map[string]any) { metadata["appliedId"] = "personal" })
			},
			names: []string{"selected Claude Desktop profile"},
		},
		{
			name: "profile entry",
			edit: func(t *testing.T, paths Paths) {
				editJSON(t, paths.Metadata, func(metadata map[string]any) { delete(metadata, "entries") })
			},
			names: []string{"Claude Desktop profile entry"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			paths, desired := desktopFixture(t)
			test.edit(t, paths)
			before := readFileBytes(t, paths.Profile)
			_, err := Prepare(paths, &desired)
			if err == nil {
				t.Fatal("a changed managed setting was accepted")
			}
			for _, name := range append([]string{"changed outside AIGW", "aigw sync"}, test.names...) {
				if !strings.Contains(err.Error(), name) {
					t.Fatalf("refusal %q does not name %q", err, name)
				}
			}
			if got := readFileBytes(t, paths.Profile); string(got) != string(before) {
				t.Fatal("a refused plan changed the profile")
			}
		})
	}
}

// Without a recorded request AIGW cannot name the changed setting, so it asks
// for the whole AIGW profile to be restored.
func TestUnattributableChangesAreRefused(t *testing.T) {
	earlierRelease := func(t *testing.T, edit func(map[string]any)) (Paths, Desired) {
		t.Helper()
		paths, desired := desktopFixture(t)
		writeEarlierReleaseRecord(t, paths)
		editJSON(t, paths.Profile, edit)
		return paths, desired
	}
	tests := []struct {
		name  string
		setup func(*testing.T) (Paths, Desired)
	}{
		{
			name: "truncated profile",
			setup: func(t *testing.T) (Paths, Desired) {
				paths, desired := desktopFixture(t)
				writeFile(t, paths.Profile, []byte("{\n"))
				return paths, desired
			},
		},
		{
			name: "earlier-release record and a changed gateway",
			setup: func(t *testing.T) (Paths, Desired) {
				return earlierRelease(t, func(profile map[string]any) {
					profile["inferenceGatewayBaseUrl"] = "https://external-edit.example.test"
				})
			},
		},
		{
			name: "earlier-release record and no models",
			setup: func(t *testing.T) (Paths, Desired) {
				return earlierRelease(t, func(profile map[string]any) { profile["inferenceModels"] = []any{} })
			},
		},
		{
			name: "legacy profile",
			setup: func(t *testing.T) (Paths, Desired) {
				paths, desired := writeLegacyProjection(t)
				legacyProfile, _ := legacyPaths(paths)
				editJSON(t, legacyProfile, func(profile map[string]any) {
					profile["inferenceGatewayBaseUrl"] = "https://external-edit.example.test"
				})
				return paths, desired
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			paths, desired := test.setup(t)
			if _, err := Prepare(paths, &desired); err == nil || !strings.Contains(err.Error(), "restore the AIGW profile in Claude Desktop, then run `aigw sync`") {
				t.Fatalf("Prepare() error = %v", err)
			}
		})
	}
}
