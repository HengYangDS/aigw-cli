package claudedesktop

import (
	"strings"
	"testing"

	"aigw-cli/internal/domain"
)

// Each entry of Desktop's profile names the Anthropic model that its model
// serves, so that Desktop's sessions run a provider's own ID at that model's
// window; an entry whose ID is its family names none.
func TestClaudeDesktopEntriesNameTheAnthropicModelBehindThem(t *testing.T) {
	cfg := domain.NewConfig(testAdmission())
	if err := cfg.AppendProvider("suhe", domain.Provider{Label: "Suhe", Endpoints: domain.Endpoints{Anthropic: "https://suhe.test"}}); err != nil {
		t.Fatal(err)
	}
	for _, model := range []domain.Model{
		{Provider: "suhe", ID: "claude-code-fable-5-2", Family: "claude-fable-5-2"},
		{Provider: "suhe", ID: "claude-opus-5-5", Family: "claude-opus-5-5"},
	} {
		model.Protocols = map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}
		if err := cfg.AppendModel(model); err != nil {
			t.Fatal(err)
		}
	}
	cfg.SetSelectedModel(ID, domain.NewModelRef("suhe", "claude-code-fable-5-2"), domain.ProtocolAnthropic)
	runtime, err := cfg.ResolveRuntime(ID, domain.ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	models := claudeDesktopModels(cfg, runtime)
	want := map[string]string{"claude-code-fable-5-2": "claude-fable-5-2", "claude-opus-5-5": ""}
	if len(models) != len(want) {
		t.Fatalf("Desktop lists %+v", models)
	}
	for _, model := range models {
		if behind, listed := want[model.Name]; !listed || model.BehavesAs != behind {
			t.Fatalf("entry %s names %q as the model behind it, want %q", model.Name, model.BehavesAs, behind)
		}
	}
}

// The model behind an entry is written to Desktop's profile and is part of
// what AIGW owns: an entry without it writes none, and an edit of it outside
// AIGW is managed drift.
func TestClaudeDesktopProfileOwnsTheModelBehindAnEntry(t *testing.T) {
	paths, desired := desktopFixture(t)
	desired.Models = []Model{{Name: "claude-code-fable-5-2", Label: "Suhe · Fable 5.2 · CC", BehavesAs: "claude-fable-5-2"}, {Name: "claude-opus-5-5"}}
	applyProjection(t, paths, desired)
	entries, _ := readJSON(t, paths.Profile)["inferenceModels"].([]any)
	if len(entries) != 2 {
		t.Fatalf("the profile lists %v", entries)
	}
	first, _ := entries[0].(map[string]any)
	second, _ := entries[1].(map[string]any)
	if first["behavesAs"] != "claude-fable-5-2" {
		t.Fatalf("the channel's entry is %v, want behavesAs claude-fable-5-2", first)
	}
	if _, named := second["behavesAs"]; named {
		t.Fatalf("an entry that is its own family names a model behind it: %v", second)
	}
	if plan, err := Prepare(paths, &desired); err != nil || plan.Action != ActionUnchanged {
		t.Fatalf("the written profile plans %v, %v; want unchanged", plan.Action, err)
	}

	editJSON(t, paths.Profile, func(profile map[string]any) {
		models, _ := profile["inferenceModels"].([]any)
		entry, _ := models[0].(map[string]any)
		entry["behavesAs"] = "claude-opus-5-5"
	})
	if _, err := Prepare(paths, &desired); err == nil || !strings.Contains(err.Error(), "changed outside AIGW") {
		t.Fatalf("an edited model behind an entry: error = %v, want managed drift", err)
	}
}
