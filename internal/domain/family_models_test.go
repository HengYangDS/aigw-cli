package domain

import (
	"maps"
	"slices"
	"testing"
)

// A model that serves its family under another ID is a variant of that
// family; one that is its own family, or names it, is none.
func TestVariantOfIsTheFamilyOfAProvidersChannel(t *testing.T) {
	for _, test := range []struct {
		name, id, family, want string
	}{
		{"a channel by prefix", "claude-code-fable-5-2", "claude-fable-5-2", "claude-fable-5-2"},
		{"a channel by suffix", "claude-opus-5-5-cc", "claude-opus-5-5", "claude-opus-5-5"},
		{"a channel of another spelling", "Claude-Code-Haiku-5-5", "claude-haiku-5-5", "claude-haiku-5-5"},
		{"a model that names its family", "claude-opus-5-5", "claude-opus-5-5", ""},
		{"a model that is its own family", "claude-sonnet-5-5", "", ""},
		{"a channel of another vendor", "codex-gpt-6.2-sol", "gpt-6.2-sol", "gpt-6.2-sol"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := (Model{ID: test.id, Family: test.family}).VariantOf(); got != test.want {
				t.Fatalf("VariantOf() of %s in family %q = %q, want %q", test.id, test.family, got, test.want)
			}
		})
	}
}

// A runtime's map sends each family that a model of the selected provider
// serves under another ID on the selected protocol to one ID of that
// provider: the selected model's own ID for its family, the first in
// configuration order for any other. A model of another provider or
// protocol, and one that is its own family, stays out.
func TestFamilyVariantsMapEachFamilyOfTheSelectedProviderOnce(t *testing.T) {
	cfg := NewConfig(testAdmission())
	for _, provider := range []string{"suhe", "other"} {
		if err := cfg.AppendProvider(provider, Provider{Label: provider, Endpoints: Endpoints{Anthropic: "https://" + provider + ".test", Responses: "https://" + provider + ".test/v1"}}); err != nil {
			t.Fatal(err)
		}
	}
	add := func(provider, id, family string, protocol EndpointProtocol) Model {
		model := Model{Provider: provider, ID: id, Family: family, Protocols: map[EndpointProtocol][]Capability{protocol: {}}}
		if err := cfg.AppendModel(model); err != nil {
			t.Fatal(err)
		}
		return model
	}
	add("suhe", "claude-code-fable-5-2", "claude-fable-5-2", ProtocolAnthropic)
	add("suhe", "claude-opus-5-5", "claude-opus-5-5", ProtocolAnthropic)
	add("suhe", "claude-opus-5-5-first", "claude-opus-5-5", ProtocolAnthropic)
	selected := add("suhe", "claude-code-opus-5-5", "claude-opus-5-5", ProtocolAnthropic)
	add("suhe", "claude-code-sonnet-5-5", "claude-sonnet-5-5", ProtocolResponses)
	add("suhe", "kimi-k3", "", ProtocolAnthropic)
	add("other", "claude-code-haiku-5-5", "claude-haiku-5-5", ProtocolAnthropic)
	cfg.SetSelectedModel(ClientClaude, selected.Ref(), ProtocolAnthropic)
	runtime, err := cfg.ResolveRuntime(ClientClaude, ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"claude-fable-5-2": "claude-code-fable-5-2", "claude-opus-5-5": "claude-code-opus-5-5"}
	if got := runtime.FamilyVariants; !maps.Equal(got, want) {
		t.Fatalf("FamilyVariants = %v, want %v", got, want)
	}

	// The selected model names its family: no other model takes the family.
	cfg.SetSelectedModel(ClientClaude, NewModelRef("suhe", "claude-opus-5-5"), ProtocolAnthropic)
	if runtime, err = cfg.ResolveRuntime(ClientClaude, ModelRef{}); err != nil {
		t.Fatal(err)
	}
	if got := runtime.FamilyVariants; !maps.Equal(got, map[string]string{"claude-fable-5-2": "claude-code-fable-5-2"}) {
		t.Fatalf("with a selection that names its family, FamilyVariants = %v", got)
	}
}

// The models that a client lists for a provider are the selected one and the
// first model of each other family on the selected protocol, in configuration
// order; the map covers exactly those. So a family whose first model is its
// own ID has no entry, although a channel of it follows, and of two channels
// of one family the first is taken.
func TestFamilyVariantsCoverTheListedModelOfEachFamily(t *testing.T) {
	cfg := NewConfig(testAdmission())
	if err := cfg.AppendProvider("suhe", Provider{Label: "Suhe", Endpoints: Endpoints{Anthropic: "https://suhe.test"}}); err != nil {
		t.Fatal(err)
	}
	for _, model := range []Model{
		{ID: "claude-code-opus-5-5", Family: "claude-opus-5-5"},
		{ID: "claude-fable-5-2", Family: "claude-fable-5-2"},
		{ID: "claude-code-fable-5-2", Family: "claude-fable-5-2"},
		{ID: "claude-sonnet-5-5-cc", Family: "claude-sonnet-5-5"},
		{ID: "claude-code-sonnet-5-5", Family: "claude-sonnet-5-5"},
	} {
		model.Provider, model.Protocols = "suhe", map[EndpointProtocol][]Capability{ProtocolAnthropic: {}}
		if err := cfg.AppendModel(model); err != nil {
			t.Fatal(err)
		}
	}
	selected := NewModelRef("suhe", "claude-code-opus-5-5")
	cfg.SetSelectedModel(ClientClaude, selected, ProtocolAnthropic)
	listed := cfg.FamilyModels(selected, ProtocolAnthropic)
	wantListed := []ModelRef{selected, NewModelRef("suhe", "claude-fable-5-2"), NewModelRef("suhe", "claude-sonnet-5-5-cc")}
	if !slices.Equal(listed, wantListed) {
		t.Fatalf("FamilyModels = %v, want %v", listed, wantListed)
	}
	runtime, err := cfg.ResolveRuntime(ClientClaude, ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"claude-opus-5-5": "claude-code-opus-5-5", "claude-sonnet-5-5": "claude-sonnet-5-5-cc"}
	if !maps.Equal(runtime.FamilyVariants, want) {
		t.Fatalf("FamilyVariants = %v, want %v", runtime.FamilyVariants, want)
	}
}

// A runtime lists the selection's FamilyModels in their order, each with its
// family, its name without the provider's and, for a client that AIGW tells
// a window, the window that the model declares or the default one.
func TestRuntimeListsTheSelectionsFamilyModels(t *testing.T) {
	cfg := NewConfig(testAdmission())
	if err := cfg.AppendProvider("dmx", Provider{Label: "DMX", Endpoints: Endpoints{Anthropic: "https://dmx.test", Responses: "https://dmx.test/v1"}}); err != nil {
		t.Fatal(err)
	}
	for _, model := range []Model{
		{ID: "gpt-6.2-sol", Family: "gpt-6.2-sol", Protocols: map[EndpointProtocol][]Capability{ProtocolResponses: {}}},
		{ID: "gpt-6.2-sol-cdx", Family: "gpt-6.2-sol", ContextWindow: 400_000, Protocols: map[EndpointProtocol][]Capability{ProtocolResponses: {}}},
		{ID: "claude-opus-5-5", Family: "claude-opus-5-5", Protocols: map[EndpointProtocol][]Capability{ProtocolAnthropic: {}}},
		{ID: "gpt-6.1-terra", Family: "gpt-6.1-terra", Label: "Terra", Protocols: map[EndpointProtocol][]Capability{ProtocolResponses: {}}},
		{ID: "deepseek-v4.1", Protocols: map[EndpointProtocol][]Capability{ProtocolResponses: {}}},
	} {
		model.Provider = "dmx"
		if err := cfg.AppendModel(model); err != nil {
			t.Fatal(err)
		}
	}
	selected := NewModelRef("dmx", "gpt-6.2-sol-cdx")
	cfg.SetSelectedModel(ClientCodex, selected, ProtocolResponses)
	runtime, err := cfg.ResolveRuntime(ClientCodex, ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, listed := range runtime.Listed {
		ids = append(ids, listed.ID)
	}
	if !slices.Equal(ids, []string{"gpt-6.2-sol-cdx", "gpt-6.1-terra", "deepseek-v4.1"}) {
		t.Fatalf("Listed models = %q, want the selection's FamilyModels on the Responses protocol", ids)
	}
	for index, ref := range cfg.FamilyModels(selected, ProtocolResponses) {
		model := cfg.Models[ref]
		want := ListedModel{ID: model.ID, Family: model.FamilyID(), Name: withoutProvider(modelLabel(model, cfg.Providers["dmx"], cfg.Families[model.Family]), Provider{ID: "dmx", Label: "DMX"}), Window: model.Window()}
		if runtime.Listed[index] != want {
			t.Errorf("Listed[%d] = %#v, want %#v", index, runtime.Listed[index], want)
		}
	}
	if first := runtime.Listed[0]; first.Name != runtime.ModelName() || first.Window != runtime.ContextWindow || first.Family != runtime.FamilyID {
		t.Fatalf("the first listed model %#v is not the selection %#v", first, runtime)
	}
	if runtime.Listed[1].Name != "Terra" || runtime.Listed[2].Family != "deepseek-v4.1" {
		t.Fatalf("an explicit label or a model that is its own family is listed as %#v", runtime.Listed[1:])
	}

	// A client that AIGW tells no window lists none.
	cfg.SetSelectedModel(ClientClaude, NewModelRef("dmx", "claude-opus-5-5"), ProtocolAnthropic)
	claude, err := cfg.ResolveRuntime(ClientClaude, ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	if len(claude.Listed) != 1 || claude.Listed[0].Window != 0 || claude.Listed[0].ID != "claude-opus-5-5" {
		t.Fatalf("Claude's listed models = %#v", claude.Listed)
	}
}

// The listed models are no part of the selected route: a runtime that lists
// other models resolves the same route, and Equal says so.
func TestRuntimeEqualityLeavesTheListedModelsOut(t *testing.T) {
	route := Runtime{Client: "codex", Model: NewModelRef("gateway", "sol"), UpstreamModel: "sol", ContextWindow: DefaultContextWindow}
	listing := route
	listing.Listed = []ListedModel{{ID: "sol", Family: "sol", Name: "Sol", Window: DefaultContextWindow}, {ID: "terra", Family: "terra", Name: "Terra", Window: DefaultContextWindow}}
	if !route.Equal(listing) || !listing.Equal(route) {
		t.Fatal("a runtime that lists other models resolves another route")
	}
	moved := listing
	moved.UpstreamModel = "terra"
	if listing.Equal(moved) {
		t.Fatal("a runtime with another upstream model resolves the same route")
	}
	if len(listing.Listed) != 2 {
		t.Fatal("Equal changed the runtime that it compared")
	}
}
