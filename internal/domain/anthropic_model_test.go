package domain

import (
	"maps"
	"testing"
)

// A model serves the Anthropic model ID that is its family when that family
// begins with claude- and is not the model's own ID; any other model serves
// none.
func TestAnthropicModelIsTheFamilyOfAClaudeChannel(t *testing.T) {
	for _, test := range []struct {
		name, id, family, want string
	}{
		{"a channel by prefix", "claude-code-fable-5-2", "claude-fable-5-2", "claude-fable-5-2"},
		{"a channel by suffix", "claude-opus-5-5-cc", "claude-opus-5-5", "claude-opus-5-5"},
		{"a channel of another spelling", "Claude-Code-Haiku-5-5", "claude-haiku-5-5", "claude-haiku-5-5"},
		{"a model that names its family", "claude-opus-5-5", "claude-opus-5-5", ""},
		{"a model that is its own family", "claude-sonnet-5-5", "", ""},
		{"a model of another vendor", "codex-gpt-6.2-sol", "gpt-6.2-sol", ""},
		{"a family that only looks like one", "my-claude-opus", "my-claude", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := (Model{ID: test.id, Family: test.family}).AnthropicModel(); got != test.want {
				t.Fatalf("AnthropicModel() of %s in family %q = %q, want %q", test.id, test.family, got, test.want)
			}
		})
	}
}

// A client's map sends each Anthropic model ID that a model of the selected
// provider serves on the selected protocol to one ID of that provider: the
// selected model's own ID for its family, the first in configuration order
// for any other. A model of another provider or protocol, and one that
// serves no Anthropic model ID, stays out.
func TestAnthropicModelsMapsEachFamilyOfTheSelectedProviderOnce(t *testing.T) {
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
	if got := runtime.AnthropicModels; !maps.Equal(got, want) {
		t.Fatalf("AnthropicModels = %v, want %v", got, want)
	}

	// The selected model names its family: no other model takes the family.
	cfg.SetSelectedModel(ClientClaude, NewModelRef("suhe", "claude-opus-5-5"), ProtocolAnthropic)
	if runtime, err = cfg.ResolveRuntime(ClientClaude, ModelRef{}); err != nil {
		t.Fatal(err)
	}
	if got := runtime.AnthropicModels; !maps.Equal(got, map[string]string{"claude-fable-5-2": "claude-code-fable-5-2"}) {
		t.Fatalf("with a selection that names its family, AnthropicModels = %v", got)
	}
}
