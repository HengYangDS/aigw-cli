package desktop

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectionOwnsTheToolSearchSetting(t *testing.T) {
	root := t.TempDir()
	paths := PathsForLibrary(filepath.Join(root, "Claude-3p", "configLibrary"))
	desired := Desired{
		BaseURL:              "https://gateway.example.test/v1",
		CredentialExecutable: filepath.Join(root, "aigw"),
		CredentialArguments:  []string{"credential", "claude-desktop", "fingerprint"},
		Models:               []Model{{Name: "claude-opus-5-5"}},
		ToolSearch:           true,
	}
	applyProjection(t, paths, desired)
	requireJSONValues(t, paths.Profile, map[string]any{"toolSearchEnabled": true})

	desired.ToolSearch = false
	applyProjection(t, paths, desired)
	if _, present := readJSON(t, paths.Profile)["toolSearchEnabled"]; present {
		t.Fatal("tool search stayed in the profile after its qualification was withdrawn")
	}

	profile := readJSON(t, paths.Profile)
	profile["toolSearchEnabled"] = true
	writeJSON(t, paths.Profile, profile)
	if _, err := Prepare(paths, &desired); err == nil || !strings.Contains(err.Error(), "changed outside AIGW") {
		t.Fatalf("tool search enabled outside AIGW: error = %v, want managed drift", err)
	}
}

func applyProjection(t *testing.T, paths Paths, desired Desired) {
	t.Helper()
	plan, err := Prepare(paths, &desired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
}
