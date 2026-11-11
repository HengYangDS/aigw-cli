package domain

import (
	"slices"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestModelSelectsProtocolIndependentlyOfClientBrand(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["team"] = Provider{Label: "Team", Endpoints: Endpoints{Responses: "https://responses.test/v1", Anthropic: "https://messages.test", Chat: "https://chat.test/v1"}}
	cfg.Models["hermes"] = Model{Label: "Hermes", Provider: "team", Family: "model-test", Protocols: map[EndpointProtocol][]Capability{ProtocolAnthropic: {}}}
	cfg.Clients[ClientHermes] = ClientBinding{Model: "hermes", Protocol: ProtocolAnthropic}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	runtime, err := cfg.ResolveRuntime("hermes", "")
	if err != nil || runtime.Endpoint != "https://messages.test" || runtime.Protocol != ProtocolAnthropic {
		t.Fatalf("explicit protocol resolution = %#v, %v", runtime, err)
	}
	data, err := toml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var restored Config
	if err := toml.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Clients[ClientHermes].Protocol != ProtocolAnthropic {
		t.Fatal("route protocol did not survive round-trip")
	}
	binding := cfg.Clients[ClientHermes]
	binding.Protocol = ""
	cfg.Clients[ClientHermes] = binding
	if runtime, err := cfg.ResolveRuntime("hermes", ""); err != nil || runtime.Protocol != ProtocolAnthropic {
		t.Fatalf("Route interface did not resolve the sole admitted protocol: %#v, %v", runtime, err)
	}
	route := cfg.Models["hermes"]
	route.Protocols = nil
	cfg.Models["hermes"] = route
	if _, err := cfg.ResolveRuntime("hermes", ""); err == nil || !strings.Contains(err.Error(), "compatible endpoint") {
		t.Fatalf("unqualified Route did not reject implicit endpoint inference: %v", err)
	}
}

func TestExplicitProtocolMustBeSupportedByClient(t *testing.T) {
	cfg := validConfig()
	binding := cfg.Clients[ClientCodex]
	binding.Protocol = ProtocolAnthropic
	cfg.Clients[ClientCodex] = binding
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "does not support") {
		t.Fatalf("Codex protocol validation = %v", err)
	}
}

func TestModelProtocolsConstrainClientCompatibilityAndRuntimeSelection(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["team"] = Provider{Label: "Team", Endpoints: Endpoints{
		Responses: "https://responses.test/v1",
		Chat:      "https://chat.test/v1",
	}}
	cfg.Models["chat-only"] = Model{
		Label:     "Chat only",
		Provider:  "team",
		Family:    "model-test",
		Protocols: map[EndpointProtocol][]Capability{ProtocolChat: {}},
	}
	cfg.Clients[ClientHermes] = ClientBinding{Model: "chat-only", Protocol: ProtocolChat}

	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if compatible, err := cfg.ProtocolClientIDs("chat-only"); err != nil || len(compatible) != 1 || compatible[0] != ClientHermes {
		t.Fatalf("compatible clients = %v, %v", compatible, err)
	}
	runtime, err := cfg.ResolveRuntime(ClientHermes, "")
	if err != nil || runtime.Protocol != ProtocolChat {
		t.Fatalf("chat-only runtime = %#v, %v", runtime, err)
	}
	cfg.Models["responses-only"] = Model{
		Label:     "Responses only",
		Provider:  "team",
		Family:    "another-model",
		Protocols: map[EndpointProtocol][]Capability{ProtocolResponses: {}},
	}
	runtime, err = cfg.ResolveRuntime(ClientHermes, "responses-only")
	if err != nil || runtime.Protocol != ProtocolResponses {
		t.Fatalf("explicit Route did not replace the previous incompatible protocol: %#v, %v", runtime, err)
	}

	invalid := cfg.Clone()
	binding := invalid.Clients[ClientHermes]
	binding.Protocol = ProtocolResponses
	invalid.Clients[ClientHermes] = binding
	if err := invalid.Validate(); err == nil || !strings.Contains(err.Error(), "does not admit") {
		t.Fatalf("unsupported Route protocol error = %v", err)
	}
}

func TestModelProtocolsRejectUnknownProtocols(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		interfaces map[EndpointProtocol][]Capability
	}{
		{name: "unknown", interfaces: map[EndpointProtocol][]Capability{"future": {}}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := validConfig()
			route := cfg.Models["dmx"]
			route.Protocols = testCase.interfaces
			cfg.Models["dmx"] = route
			if err := cfg.Validate(); err == nil {
				t.Fatalf("accepted invalid Route interfaces: %v", testCase.interfaces)
			}
		})
	}
}

func TestModelProtocolsKeepProtocolAndCapabilitySeparate(t *testing.T) {
	for _, testCase := range []struct {
		name         string
		capabilities []Capability
		wantError    string
	}{
		{name: "known", capabilities: []Capability{CapabilityText, CapabilityReasoning, CapabilityStreaming}},
		{name: "duplicate", capabilities: []Capability{CapabilityText, CapabilityText}, wantError: "repeats capability"},
		{name: "unknown", capabilities: []Capability{"future"}, wantError: "unknown capability"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := validConfig()
			route := cfg.Models["backup"]
			route.Protocols = map[EndpointProtocol][]Capability{ProtocolResponses: testCase.capabilities}
			cfg.Models["backup"] = route
			err := cfg.Validate()
			if testCase.wantError == "" && err != nil {
				t.Fatal(err)
			}
			if testCase.wantError != "" && (err == nil || !strings.Contains(err.Error(), testCase.wantError)) {
				t.Fatalf("error = %v, want %q", err, testCase.wantError)
			}
		})
	}
}

func TestModelToolSearchBelongsToTheAnthropicInterface(t *testing.T) {
	for _, protocol := range []EndpointProtocol{ProtocolAnthropic, ProtocolResponses, ProtocolChat} {
		t.Run(string(protocol), func(t *testing.T) {
			cfg := validConfig()
			account := cfg.Providers["dmx"]
			account.Endpoints.Chat = "https://example.test/chat/v1"
			cfg.Providers["dmx"] = account
			route := cfg.Models["dmx"]
			route.Protocols = map[EndpointProtocol][]Capability{ProtocolAnthropic: {}}
			route.Protocols[protocol] = []Capability{CapabilityTools, Capability("tool_search")}
			cfg.Models["dmx"] = route
			err := cfg.Validate()
			if protocol == ProtocolAnthropic {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "only the anthropic interface") {
				t.Fatalf("error = %v, want an Anthropic-only refusal", err)
			}
		})
	}
}

func TestModelCapabilitiesRemainScopedToTheExactWireInterface(t *testing.T) {
	route := Model{
		Provider: "team",
		Family:   "model",
		Protocols: map[EndpointProtocol][]Capability{
			ProtocolResponses: {CapabilityText, CapabilityReasoning},
			ProtocolChat:      {CapabilityText},
		},
	}
	data, err := toml.Marshal(route)
	if err != nil {
		t.Fatal(err)
	}
	var restored Model
	if err := toml.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(restored.Protocols[ProtocolResponses], CapabilityReasoning) {
		t.Fatal("Responses reasoning qualification was lost")
	}
	if slices.Contains(restored.Protocols[ProtocolChat], CapabilityReasoning) {
		t.Fatal("Responses reasoning qualification leaked into Chat Completions")
	}
}

func TestManifestEquivalenceIncludesEveryEndpointAndProtocol(t *testing.T) {
	left := Provider{Label: "Team", Endpoints: Endpoints{Chat: "https://first.test/v1"}}
	right := left
	right.Endpoints.Chat = "https://second.test/v1"
	if equivalentProvider(left, right) {
		t.Fatal("a team-file merge treated different Chat Completions endpoints as equivalent")
	}
	route := Model{Provider: "team", Family: "model", Protocols: map[EndpointProtocol][]Capability{ProtocolResponses: {}}}
	if !equivalentModel(route, route) {
		t.Fatal("a team-file merge treated equal provider model identities as different")
	}
}
