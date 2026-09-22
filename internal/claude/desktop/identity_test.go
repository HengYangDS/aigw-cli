package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestProfileIdentityMatchesClaudeDesktopContract(t *testing.T) {
	if !regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[1-8][a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$`).MatchString(profileID) {
		t.Fatalf("profile ID %q is not a Claude Desktop configuration UUID", profileID)
	}
}

func TestProjectionMigratesTheInvalidLegacyProfileIdentity(t *testing.T) {
	root := t.TempDir()
	paths := PathsForLibrary(filepath.Join(root, "Claude-3p", "configLibrary"))
	legacyProfile, legacyState := legacyPaths(paths)
	desired := Desired{
		BaseURL:              "https://gateway.example.test/v1",
		CredentialExecutable: filepath.Join(root, "aigw"),
		CredentialArguments:  []string{"credential", "claude-desktop", "fingerprint"},
		Models:               []Model{{Name: "claude-fable-5-1"}},
	}
	standard := document{"deploymentMode": raw("3p")}
	thirdParty := document{"deploymentMode": raw("3p")}
	profile, err := encodeProfile(desired)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := json.Marshal(map[string]string{"id": legacyProfileID, "name": profileName})
	if err != nil {
		t.Fatal(err)
	}
	metadata := document{"appliedId": raw(legacyProfileID), "entries": raw([]json.RawMessage{entry})}
	hash, err := managedHash(legacyProfileID, standard, thirdParty, profile, metadata)
	if err != nil {
		t.Fatal(err)
	}
	state, err := encode(ownershipState{Version: 1, WriterID: "aigw-cli", ManagedSHA256: hash})
	if err != nil {
		t.Fatal(err)
	}
	writeJSON(t, paths.StandardConfig, standard)
	writeJSON(t, paths.ThirdPartyConfig, thirdParty)
	writeJSON(t, paths.Metadata, metadata)
	writeFile(t, legacyProfile, profile)
	writeFile(t, legacyState, state)

	plan, err := Prepare(paths, &desired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	requireAbsent(t, legacyProfile, legacyState)
	requirePresent(t, paths.Profile, paths.State)
	var migrated struct {
		AppliedID string `json:"appliedId"`
		Entries   []struct {
			ID string `json:"id"`
		} `json:"entries"`
	}
	metadataBytes, err := os.ReadFile(paths.Metadata)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(metadataBytes, &migrated); err != nil {
		t.Fatal(err)
	}
	if migrated.AppliedID != profileID || len(migrated.Entries) != 1 || migrated.Entries[0].ID != profileID {
		t.Fatalf("migrated metadata = %#v", migrated)
	}

	withdrawal, err := Prepare(paths, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := withdrawal.Apply(); err != nil {
		t.Fatal(err)
	}
	requireAbsent(t, paths.StandardConfig, paths.ThirdPartyConfig, paths.Profile, paths.State, paths.Metadata, legacyProfile, legacyState)
}

func TestProjectionRejectsAmbiguousProfileOwnership(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T, Paths, string, string)
		want  string
	}{
		{
			name: "both ownership states",
			setup: func(t *testing.T, paths Paths, _ string, legacyState string) {
				writeFile(t, paths.State, []byte("{}"))
				writeFile(t, legacyState, []byte("{}"))
			},
			want: "multiple Claude Desktop ownership states are present",
		},
		{
			name: "legacy profile beside current state",
			setup: func(t *testing.T, paths Paths, legacyProfile, _ string) {
				writeFile(t, paths.State, []byte("{}"))
				writeFile(t, legacyProfile, []byte("{}"))
			},
			want: "legacy Claude Desktop profile exists beside the current ownership state",
		},
		{
			name: "current profile beside legacy state",
			setup: func(t *testing.T, paths Paths, _ string, legacyState string) {
				writeFile(t, paths.Profile, []byte("{}"))
				writeFile(t, legacyState, []byte("{}"))
			},
			want: "current Claude Desktop profile exists beside the legacy ownership state",
		},
		{
			name: "profile without ownership state",
			setup: func(t *testing.T, paths Paths, _, _ string) {
				writeFile(t, paths.Profile, []byte("{}"))
			},
			want: "Claude Desktop AIGW profile already exists without AIGW ownership",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			paths := PathsForLibrary(filepath.Join(t.TempDir(), "Claude-3p", "configLibrary"))
			legacyProfile, legacyState := legacyPaths(paths)
			test.setup(t, paths, legacyProfile, legacyState)

			_, err := Prepare(paths, &Desired{})
			if err == nil || err.Error() != test.want {
				t.Fatalf("Prepare() error = %v, want %q", err, test.want)
			}
		})
	}
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func requirePresent(t *testing.T, paths ...string) {
	t.Helper()
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s: %v", path, err)
		}
	}
}

func requireAbsent(t *testing.T, paths ...string) {
	t.Helper()
	for _, path := range paths {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("unexpected %s: %v", path, err)
		}
	}
}
