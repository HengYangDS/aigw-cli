package main

import (
	"bytes"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"reflect"

	"aigw-cli/internal/platform"
)

// claudeOwnedKeysRecord is AIGW's record of the settings keys that it newly
// owns in Claude Code's settings, beside the settings file.
func (j *journeyFixture) claudeOwnedKeysRecord() string {
	return j.settings + ".aigw-owned-keys.json"
}

// modelOverrides reads the map that Claude Code's settings hold, and whether
// they hold one.
func (j *journeyFixture) modelOverrides() (map[string]string, bool) {
	j.testing.Helper()
	var settings map[string]json.RawMessage
	if err := json.Unmarshal(readFile(j.testing, j.settings), &settings); err != nil {
		j.testing.Fatal(err)
	}
	raw, present := settings["modelOverrides"]
	if !present {
		return nil, false
	}
	var overrides map[string]string
	if err := json.Unmarshal(raw, &overrides); err != nil {
		j.testing.Fatalf("Claude modelOverrides is not a map of strings: %s", raw)
	}
	return overrides, true
}

// requireModelOverrides requires that Claude Code's settings map the
// Anthropic model ID of the journey's selection to its provider's own ID,
// whichever program wrote the map: a predecessor that does not manage the key
// leaves it as its successor wrote it.
func (j *journeyFixture) requireModelOverrides() {
	j.testing.Helper()
	want := map[string]string{probeFamily: probeModel.ID()}
	if got, present := j.modelOverrides(); !present || !maps.Equal(got, want) {
		j.testing.Fatalf("Claude modelOverrides = %v (present %t), want %v", got, present, want)
	}
}

// requireOwnedModelOverrides requires the selection's map and AIGW's record
// of it, as a program that manages the key leaves them after a
// synchronization.
func (j *journeyFixture) requireOwnedModelOverrides() {
	j.testing.Helper()
	j.requireModelOverrides()
	if info, err := os.Stat(j.claudeOwnedKeysRecord()); err != nil || !info.Mode().IsRegular() {
		j.testing.Fatalf("Claude's record of owned settings keys is missing beside the written map: %v", err)
	}
}

// requireNoModelOverrides requires that a withdrawal took AIGW's map out of
// settings that held none before AIGW wrote it.
func (j *journeyFixture) requireNoModelOverrides() {
	j.testing.Helper()
	if got, present := j.modelOverrides(); present {
		j.testing.Fatalf("withdrawal left Claude modelOverrides %v in the user's settings", got)
	}
}

func (j *journeyFixture) requireSyncPreservesUserSettings(previousTheme, theme string) {
	j.testing.Helper()
	settings := map[string]json.RawMessage{}
	if err := json.Unmarshal(readFile(j.testing, j.settings), &settings); err != nil {
		j.testing.Fatal(err)
	}
	var observedTheme string
	if data, present := settings["theme"]; present {
		if err := json.Unmarshal(data, &observedTheme); err != nil {
			j.testing.Fatal(err)
		}
	}
	if observedTheme != previousTheme {
		j.testing.Fatalf("program update changed the user theme: got=%q want=%q", observedTheme, previousTheme)
	}
	themeData, err := json.Marshal(theme)
	if err != nil {
		j.testing.Fatal(err)
	}
	settings["theme"] = themeData
	userSettings, err := json.Marshal(settings)
	if err != nil {
		j.testing.Fatal(err)
	}
	if err := os.WriteFile(j.settings, userSettings, 0o600); err != nil {
		j.testing.Fatal(err)
	}
	j.run("sync")
	if got := readFile(j.testing, j.settings); !bytes.Equal(got, userSettings) {
		j.testing.Fatalf("sync changed the externally edited settings: got=%s want=%s", got, userSettings)
	}
	paths := []string{j.config, j.config + ".bak", verifiedSnapshot(j.config), j.settings, j.settings + ".aigw-state.json", j.claudeOwnedKeysRecord()}
	codexConfig := filepath.Join(j.root, "home", ".codex", "config.toml")
	paths = append(paths, codexConfig, codexConfig+".aigw-state.json", codexConfig+".aigw-model-catalog.json")
	before := make(map[string]platform.FileSnapshot, len(paths))
	for _, path := range paths {
		content, err := platform.CaptureFileSnapshot(path)
		if err != nil {
			j.testing.Fatal(err)
		}
		before[path] = content
	}
	for range 2 {
		var result struct {
			NextAction string `json:"next_action"`
		}
		if err := json.Unmarshal(j.run("sync", "--json"), &result); err != nil {
			j.testing.Fatal(err)
		}
		if result.NextAction != "aigw check" {
			j.testing.Fatalf("repeated sync is not terminal: %#v", result)
		}
		for _, path := range paths {
			after, err := platform.CaptureFileSnapshot(path)
			if err != nil {
				j.testing.Fatal(err)
			}
			if !reflect.DeepEqual(after, before[path]) {
				j.testing.Fatalf("repeated sync changed %s", path)
			}
		}
	}
}
