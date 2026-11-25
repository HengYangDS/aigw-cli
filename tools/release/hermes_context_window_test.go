package main

import (
	"strings"

	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/domain"

	"go.yaml.in/yaml/v4"
)

// requireHermesModelWindows requires that every model which AIGW lists under
// its own provider entries in Hermes's file carries its context window, as
// the program that owns that setting leaves the file after a synchronization.
// The journey's team file declares no window, so each is the default. A
// rollback that follows it runs with the setting written, whatever form the
// published predecessor then gives the file.
func (j *journeyFixture) requireHermesModelWindows() {
	j.testing.Helper()
	path := j.clientProjectionPaths(hermes.ID)[0]
	var projected struct {
		Providers map[string]struct {
			Models map[string]struct {
				ContextLength int `yaml:"context_length"`
			} `yaml:"models"`
		} `yaml:"providers"`
	}
	if err := yaml.Unmarshal(readFile(j.testing, path), &projected); err != nil {
		j.testing.Fatalf("Hermes's file does not list each entry's models with their settings: %v", err)
	}
	owned := 0
	for id, entry := range projected.Providers {
		if !strings.HasPrefix(id, "aigw-") {
			continue
		}
		owned++
		if len(entry.Models) == 0 {
			j.testing.Fatalf("Hermes's entry %s lists no model", id)
		}
		for model, settings := range entry.Models {
			if settings.ContextLength != domain.DefaultContextWindow {
				j.testing.Fatalf("Hermes's entry %s gives %s a context window of %d, want the default %d", id, model, settings.ContextLength, domain.DefaultContextWindow)
			}
		}
	}
	if owned == 0 {
		j.testing.Fatal("Hermes's file holds no provider entry of AIGW's")
	}
}
