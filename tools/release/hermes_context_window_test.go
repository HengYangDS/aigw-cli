package main

import (
	"strings"

	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/store"

	"go.yaml.in/yaml/v4"
)

// hermesDeclaredWindows counts, for each Hermes provider entry that cfg gives
// AIGW, the window of every model listed under it: one entry per provider and
// protocol, named as the adapter names it, with the models that both admit.
func hermesDeclaredWindows(cfg domain.Config) map[string]map[string]int {
	spec, _ := clients.Admission().Spec(hermes.ID)
	windows := map[string]map[string]int{}
	for _, providerID := range cfg.ProviderIDs() {
		for _, ref := range cfg.ProviderModelRefs(providerID) {
			model := cfg.Models[ref]
			for _, protocol := range spec.CompatibleModelProtocols(cfg.Providers[providerID], model) {
				entry := "aigw-" + providerID + "-" + protocol.Name()
				if windows[entry] == nil {
					windows[entry] = map[string]int{}
				}
				windows[entry][model.ID] = model.Window()
			}
		}
	}
	return windows
}

// requireHermesModelWindows requires that every model which AIGW lists under
// its own provider entries in Hermes's file carries the context window that
// the configuration gives it, as the program that owns that setting leaves
// the file after a synchronization, and that no entry of AIGW's names a
// provider or model that the configuration does not hold. It returns how many
// listed models carry a declared window rather than the default.
func (j *journeyFixture) requireHermesModelWindows() int {
	j.testing.Helper()
	cfg, err := store.NewStore(j.config, clients.Admission()).Load()
	if err != nil {
		j.testing.Fatal(err)
	}
	windows := hermesDeclaredWindows(cfg)
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
		if !strings.HasPrefix(id, "aigw-") {
			continue
		}
		want, known := windows[id]
		if !known {
			j.testing.Fatalf("Hermes's file holds entry %s, which the configuration does not give AIGW", id)
		}
		owned++
		if len(entry.Models) == 0 {
			j.testing.Fatalf("Hermes's entry %s lists no model", id)
		}
		for model, settings := range entry.Models {
			window, listed := want[model]
			if !listed {
				j.testing.Fatalf("Hermes's entry %s lists %s, which the configuration does not give it", id, model)
			}
			if settings.ContextLength != window {
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
