//go:build client_acceptance

package main

import (
	"encoding/json"
	"path/filepath"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/hermes"
)

// requireNativePreferences requires that each client still holds the native
// preferences its journey fixture set before AIGW projected into it.
func (j *journeyFixture) requireNativePreferences(client string) {
	j.testing.Helper()
	switch client {
	case codex.ID:
		j.requireNativeCodexPreferences()
	case claude.ID:
		var preferences struct {
			Effort  string `json:"effortLevel"`
			Compact int    `json:"autoCompactWindow"`
		}
		if err := json.Unmarshal(readFile(j.testing, j.settings), &preferences); err != nil {
			j.testing.Fatal(err)
		}
		if preferences.Effort != "high" || preferences.Compact != 180000 {
			j.testing.Fatalf("Claude preferences changed: %+v", preferences)
		}
	case hermes.ID:
		requireFileContains(
			j.testing,
			filepath.Join(environmentValues(j.environment)["HERMES_HOME"], "config.yaml"),
			"temperature: 0.3",
			"personal:",
			"backend: local",
			"updates:\n  check: false",
		)
	default:
		j.testing.Fatalf("unsupported native client %q", client)
	}
}
