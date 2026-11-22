package domain

import (
	"encoding/json"
	"fmt"
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

// A model stores a window only when it differs from the default: what it
// stores for the default is what a model that declares none stores, and a
// model that stores what a window gives has that window.
func TestDeclaredContextWindowStoresEveryWindowButTheDefault(t *testing.T) {
	if got, none := DeclaredContextWindow(DefaultContextWindow), (Model{}).ContextWindow; got != none {
		t.Errorf("a model stores %d for the default window, want %d, as a model that declares none does", got, none)
	}
	for _, window := range []int{1, 256000, DefaultContextWindow - 1, DefaultContextWindow, DefaultContextWindow + 1} {
		stored := DeclaredContextWindow(window)
		if got := (Model{ContextWindow: stored}).Window(); got != window {
			t.Errorf("a model that stores %d for a window of %d has the window %d", stored, window, got)
		}
	}
}

// windowRefusal is the refusal of a context window of tokens, which names
// both bounds.
func windowRefusal(tokens int) string {
	return fmt.Sprintf("a context window is a whole number of tokens from %d to %d, not %d", MinContextWindow, MaxContextWindow, tokens)
}

// A context window that a model declares is a whole number of tokens within
// the two bounds, which hold the default, and a refusal names both bounds.
func TestValidateContextWindowAdmitsOnlyAWindowWithinItsBounds(t *testing.T) {
	if MinContextWindow > DefaultContextWindow || DefaultContextWindow > MaxContextWindow {
		t.Fatalf("the bounds %d and %d do not hold the default %d", MinContextWindow, MaxContextWindow, DefaultContextWindow)
	}
	for window, valid := range map[int]bool{
		MinContextWindow: true, DefaultContextWindow: true, MaxContextWindow: true,
		MinContextWindow - 1: false, MaxContextWindow + 1: false, 0: false, -1: false,
	} {
		err := ValidateContextWindow(window)
		if (err == nil) != valid || (err != nil && err.Error() != windowRefusal(window)) {
			t.Errorf("ValidateContextWindow(%d) = %v, want valid %v or the refusal %q", window, err, valid, windowRefusal(window))
		}
	}
}

// A configuration whose model declares a context window outside the bounds
// is refused by the model's name; a declared window within them is valid.
func TestValidateRefusesAContextWindowOutsideItsBounds(t *testing.T) {
	ref := NewModelRef("dmx", "claude-test")
	cfg := validConfig()
	model := cfg.Models[ref]
	model.ContextWindow = 256000
	cfg.Models[ref] = model
	if err := cfg.Validate(); err != nil {
		t.Fatalf("a declared window within the bounds = %v", err)
	}
	for _, window := range []int{-1, MinContextWindow - 1, MaxContextWindow + 1} {
		model.ContextWindow = window
		cfg.Models[ref] = model
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "model dmx/claude-test: "+windowRefusal(window)) {
			t.Fatalf("a window of %d = %v, want a refusal that names the model and both bounds", window, err)
		}
		if entry, named := EntryOf(err); !named || entry != "model `dmx/claude-test`" {
			t.Fatalf("the refusal names the entry %q, %v", entry, named)
		}
	}
}

// The runtime of a client that AIGW tells a model's window carries the
// selected model's: the default for a model that declares none, the declared
// one otherwise, and that of a model that the client tries without selecting
// it. The runtime's JSON document does not hold it. The runtime of any other
// client carries none, whatever its model declares.
func TestResolveRuntimeCarriesTheSelectedModelsWindow(t *testing.T) {
	cfg := validConfig()
	runtime, err := cfg.ResolveRuntime(ClientCodex, ModelRef{})
	if err != nil || runtime.ContextWindow != DefaultContextWindow {
		t.Fatalf("the runtime of a model that declares no window carries %d, %v; want the default %d", runtime.ContextWindow, err, DefaultContextWindow)
	}
	selected := cfg.Models[cfg.Clients[ClientClaude].Model]
	selected.ContextWindow = 300000
	cfg.Models[selected.Ref()] = selected
	if runtime, err := cfg.ResolveRuntime(ClientClaude, ModelRef{}); err != nil || runtime.ContextWindow != 0 {
		t.Fatalf("the runtime of a client that is told no window carries %d, %v", runtime.ContextWindow, err)
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

// storing returns a copy of cfg in which the model that ref names stores
// window as its context window.
func storing(cfg Config, ref ModelRef, window int) Config {
	stored := cfg.Clone()
	model := stored.Models[ref]
	model.ContextWindow = window
	stored.Models[ref] = model
	return stored
}

// A team file's model brings its window. The file's declaration replaces the
// local one, and a file that declares none removes a window that the operator
// declared; the merge reports the model as changed in both cases, and names
// each window that changes with the one before it. A file whose model has the
// local one's window changes nothing, and a model that only the configuration
// has keeps its window.
func TestMergeTeamFileGivesAModelTheFilesWindow(t *testing.T) {
	filed, localOnly := NewModelRef("team", "gpt"), NewModelRef("team", "spare")
	file := teamFile(t)
	// The first merge takes the file's label, so that the merges below change
	// the model by its window alone.
	base, _ := mergeTeam(t, localTeamConfig(t), file, false)
	none := (Model{}).ContextWindow
	for name, test := range map[string]struct {
		local, file, want int
		changed           bool
	}{
		"a window for a model that declares none":  {local: none, file: 256000, want: 256000, changed: true},
		"a window in place of the operator's":      {local: 200000, file: 256000, want: 256000, changed: true},
		"no window in place of the operator's":     {local: 200000, file: none, want: DefaultContextWindow, changed: true},
		"the window that the model declares":       {local: 256000, file: 256000, want: 256000},
		"no window for a model that declares none": {local: none, file: none, want: DefaultContextWindow},
	} {
		cfg := storing(storing(base, filed, test.local), localOnly, 300000)
		merged, report := mergeTeam(t, cfg, storing(file, filed, test.file), false)
		if got := merged.Models[filed].Window(); got != test.want {
			t.Errorf("%s: the model has the window %d, want %d", name, got, test.want)
		}
		var changed []ModelRef
		var windows []ChangedWindow
		if test.changed {
			changed = []ModelRef{filed}
			windows = []ChangedWindow{{Model: filed, Local: (Model{ContextWindow: test.local}).Window(), File: test.want}}
		}
		if !slices.Equal(report.ChangedModels, changed) || len(report.ChangedLabels) != 0 {
			t.Errorf("%s: changed models %v, want %v; changed labels %v", name, report.ChangedModels, changed, report.ChangedLabels)
		}
		if !slices.Equal(report.ChangedWindows, windows) {
			t.Errorf("%s: changed windows %+v, want %+v", name, report.ChangedWindows, windows)
		}
		if got := merged.Models[localOnly].Window(); got != 300000 {
			t.Errorf("%s: a model that only the configuration has is left with the window %d, want its own 300000", name, got)
		}
	}
}

// On a protocol whose endpoint the provider keeps local, the model keeps its
// local capabilities, and still takes the file's window, which the merge
// names beside the operator's.
func TestMergeTeamFileNamesTheWindowThatItReplacesOnAKeptEndpoint(t *testing.T) {
	ref := NewModelRef("team", "sonnet")
	cfg := storing(localTeamConfig(t), ref, 200000)
	file := teamFile(t)
	file.Models[ref] = Model{Provider: "team", Family: "sonnet", ID: "sonnet",
		Protocols: map[EndpointProtocol][]Capability{ProtocolAnthropic: {CapabilityText, CapabilityToolSearch}}}
	merged, report := mergeTeam(t, cfg, file, false)
	if got := merged.Models[ref].Protocols[ProtocolAnthropic]; !slices.Equal(got, []Capability{CapabilityText}) {
		t.Fatalf("the local capabilities were not kept: %v", got)
	}
	want := []ChangedWindow{{Model: ref, Local: 200000, File: DefaultContextWindow}}
	if merged.Models[ref].Window() != DefaultContextWindow || !slices.Equal(report.ChangedWindows, want) {
		t.Fatalf("the model has the window %d, changed windows %+v; want the file's %d and %+v", merged.Models[ref].Window(), report.ChangedWindows, DefaultContextWindow, want)
	}
}

// A window declared as the default is the window of a model that declares
// none, so the merge sees no difference between the two, whichever of the
// team file and the configuration spells the default out.
func TestMergeTeamFileSeesNoDifferenceBetweenAnExplicitDefaultAndNone(t *testing.T) {
	ref := NewModelRef("team", "gpt")
	file := teamFile(t)
	base, _ := mergeTeam(t, localTeamConfig(t), file, false)
	for name, merge := range map[string][2]Config{
		"the file spells the default out":          {base, storing(file, ref, DefaultContextWindow)},
		"the configuration spells the default out": {storing(base, ref, DefaultContextWindow), file},
	} {
		merged, report := mergeTeam(t, merge[0], merge[1], false)
		if got := merged.Models[ref].Window(); got != DefaultContextWindow || len(report.ChangedModels) != 0 || len(report.ChangedWindows) != 0 {
			t.Errorf("%s: the model has the window %d, changed models %v, changed windows %v; want the default and no change", name, got, report.ChangedModels, report.ChangedWindows)
		}
	}
}
