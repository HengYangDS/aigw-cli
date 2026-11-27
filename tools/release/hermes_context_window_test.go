package main

import (
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/store"

	"go.yaml.in/yaml/v4"
)

// requireHermesModelWindows requires that every model which AIGW lists under
// its own provider entries in Hermes's file carries the context window that
// the configuration gives it, as the program that owns that setting leaves
// the file after a synchronization. It returns how many listed models carry a
// declared window rather than the default. A rollback that follows it runs
// with the setting written, whatever form the published predecessor then
// gives the file.
func (j *journeyFixture) requireHermesModelWindows() int {
	j.testing.Helper()
	cfg, err := store.NewStore(j.config, clients.Admission()).Load()
	if err != nil {
		j.testing.Fatal(err)
	}
	spec, _ := clients.Admission().Spec(hermes.ID)
	windows := map[string]map[string]int{}
	for _, providerID := range cfg.ProviderIDs() {
		for _, protocol := range spec.EndpointProtocols {
			entry := map[string]int{}
			for _, ref := range cfg.ProviderModelRefs(providerID) {
				model := cfg.Models[ref]
				entry[model.ID] = model.Window()
			}
			windows["aigw-"+providerID+"-"+protocol.Name()] = entry
		}
	}
	var projected struct {
		Providers map[string]struct {
			Models map[string]struct {
				ContextLength int `yaml:"context_length"`
			} `yaml:"models"`
		} `yaml:"providers"`
	}
	if err := yaml.Unmarshal(readFile(j.testing, j.clientProjectionPaths(hermes.ID)[0]), &projected); err != nil {
		j.testing.Fatalf("Hermes's file does not list each entry's models with their settings: %v", err)
	}
	owned, declared := 0, 0
	for id, entry := range projected.Providers {
		want, ours := windows[id]
		if !ours {
			continue
		}
		owned++
		if len(entry.Models) == 0 {
			j.testing.Fatalf("Hermes's entry %s lists no model", id)
		}
		for model, settings := range entry.Models {
			window, known := want[model]
			if !known || settings.ContextLength != window {
				j.testing.Fatalf("Hermes's entry %s gives %s a context window of %d, want %d", id, model, settings.ContextLength, window)
			}
			if window != domain.DefaultContextWindow {
				declared++
			}
		}
	}
	if owned == 0 {
		j.testing.Fatal("Hermes's file holds no provider entry of AIGW's")
	}
	return declared
}
