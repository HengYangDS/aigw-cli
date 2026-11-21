package domain

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// A model that declares no context window has the default one, and a model
// that declares one has that.
func TestModelWindowIsTheDeclaredOneOrTheDefault(t *testing.T) {
	if got := (Model{}).Window(); got != DefaultContextWindow {
		t.Errorf("the window of a model that declares none = %d, want the default %d", got, DefaultContextWindow)
	}
	if got := (Model{ContextWindow: 256000}).Window(); got != 256000 {
		t.Errorf("the window of a model that declares 256000 = %d", got)
	}
}

// A context window that a model declares is a whole number of tokens above
// zero.
func TestValidateContextWindowAdmitsOnlyANumberAboveZero(t *testing.T) {
	for window, valid := range map[int]bool{1: true, 256000: true, DefaultContextWindow: true, 0: false, -1: false} {
		if err := ValidateContextWindow(window); (err == nil) != valid {
			t.Errorf("ValidateContextWindow(%d) = %v, want valid %v", window, err, valid)
		}
	}
}

// A configuration whose model declares a context window below zero is
// refused by the model's name; a declared window above zero is valid.
func TestValidateRefusesAContextWindowBelowZero(t *testing.T) {
	ref := NewModelRef("dmx", "claude-test")
	cfg := validConfig()
	model := cfg.Models[ref]
	model.ContextWindow = 256000
	cfg.Models[ref] = model
	if err := cfg.Validate(); err != nil {
		t.Fatalf("a declared window above zero = %v", err)
	}
	model.ContextWindow = -1
	cfg.Models[ref] = model
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "model dmx/claude-test: a context window is a whole number of tokens above zero, not -1") {
		t.Fatalf("a window below zero = %v, want a refusal that names the model and its context window", err)
	}
	if entry, named := EntryOf(err); !named || entry != "model `dmx/claude-test`" {
		t.Fatalf("the refusal names the entry %q, %v", entry, named)
	}
}

// The runtime of a selection carries the selected model's window: the default
// for a model that declares none, the declared one otherwise, and that of a
// model that a client tries without selecting it. The runtime's JSON document
// does not hold it.
func TestResolveRuntimeCarriesTheSelectedModelsWindow(t *testing.T) {
	cfg := validConfig()
	runtime, err := cfg.ResolveRuntime(ClientClaude, ModelRef{})
	if err != nil || runtime.ContextWindow != DefaultContextWindow {
		t.Fatalf("the runtime of a model that declares no window carries %d, %v; want the default %d", runtime.ContextWindow, err, DefaultContextWindow)
	}
	ref := NewModelRef("backup", "gpt-test")
	model := cfg.Models[ref]
	model.ContextWindow = 400000
	cfg.Models[ref] = model
	for client, explicit := range map[string]ModelRef{ClientCodex: {}, ClientHermes: ref} {
		runtime, err := cfg.ResolveRuntime(client, explicit)
		if err != nil || runtime.Model != ref || runtime.ContextWindow != 400000 {
			t.Fatalf("%s's runtime of %s carries the window %d, %v; want 400000", client, runtime.Model, runtime.ContextWindow, err)
		}
		document, err := json.Marshal(runtime)
		if err != nil || strings.Contains(string(document), "400000") {
			t.Fatalf("the runtime's JSON document holds the window: %s, %v", document, err)
		}
	}
}

// A team file that gives a model another window changes the model, which
// takes the file's window, and the same file applied again changes nothing.
// A file that declares the default for a model that declares none changes the
// model too: the configuration then holds a declaration that it did not.
func TestMergeTeamFileComparesAModelsWindow(t *testing.T) {
	ref := NewModelRef("team", "gpt")
	file := teamFile(t)
	cfg, _ := mergeTeam(t, localTeamConfig(t), file, false)
	declare := func(window int) Config {
		declared := file.Clone()
		model := declared.Models[ref]
		model.ContextWindow = window
		declared.Models[ref] = model
		return declared
	}
	merged, report := mergeTeam(t, cfg, declare(256000), false)
	if got := merged.Models[ref].Window(); got != 256000 || !slices.Equal(report.ChangedModels, []ModelRef{ref}) || len(report.ChangedLabels) != 0 {
		t.Fatalf("a lower window in the file: the model has %d, changed models %v, changed labels %v", got, report.ChangedModels, report.ChangedLabels)
	}
	if _, report := mergeTeam(t, merged, declare(256000), false); len(report.ChangedModels) != 0 {
		t.Fatalf("the same file applied again changed %v", report.ChangedModels)
	}
	merged, report = mergeTeam(t, cfg, declare(DefaultContextWindow), false)
	if got := merged.Models[ref].ContextWindow; got != DefaultContextWindow || !slices.Equal(report.ChangedModels, []ModelRef{ref}) {
		t.Fatalf("a file that declares the default: the model declares %d, changed models %v", got, report.ChangedModels)
	}
}
