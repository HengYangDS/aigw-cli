package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"

	"aigw-cli/internal/platform"
)

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
	paths := []string{j.config, j.config + ".bak", verifiedSnapshot(j.config), j.settings, j.settings + ".aigw-state.json"}
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
