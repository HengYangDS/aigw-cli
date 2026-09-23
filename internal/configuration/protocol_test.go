package configuration

import (
	"slices"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestRouteSelectsProtocolIndependentlyOfClientBrand(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Team", Endpoints: Endpoints{OpenAIResponses: "https://responses.test/v1", Anthropic: "https://messages.test", OpenAIChatCompletions: "https://chat.test/v1"}}
	cfg.Routes["hermes"] = Route{Label: "Hermes", Account: "team", Model: "model-test", Interfaces: map[EndpointProtocol][]Capability{ProtocolAnthropic: {}}}
	cfg.Clients[ClientHermes] = ClientBinding{Route: "hermes", Protocol: ProtocolAnthropic}
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
	route := cfg.Routes["hermes"]
	route.Interfaces = nil
	cfg.Routes["hermes"] = route
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

func TestRouteInterfacesConstrainClientCompatibilityAndRuntimeSelection(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Team", Endpoints: Endpoints{
		OpenAIResponses:       "https://responses.test/v1",
		OpenAIChatCompletions: "https://chat.test/v1",
	}}
	cfg.Routes["chat-only"] = Route{
		Label:      "Chat only",
		Account:    "team",
		Model:      "model-test",
		Interfaces: map[EndpointProtocol][]Capability{ProtocolOpenAIChatCompletions: {}},
	}
	cfg.Clients[ClientHermes] = ClientBinding{Route: "chat-only", Protocol: ProtocolOpenAIChatCompletions}

	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if compatible, err := cfg.CompatibleClientIDs("chat-only"); err != nil || len(compatible) != 1 || compatible[0] != ClientHermes {
		t.Fatalf("compatible clients = %v, %v", compatible, err)
	}
	runtime, err := cfg.ResolveRuntime(ClientHermes, "")
	if err != nil || runtime.Protocol != ProtocolOpenAIChatCompletions {
		t.Fatalf("chat-only runtime = %#v, %v", runtime, err)
	}
	cfg.Routes["responses-only"] = Route{
		Label:      "Responses only",
		Account:    "team",
		Model:      "another-model",
		Interfaces: map[EndpointProtocol][]Capability{ProtocolOpenAIResponses: {}},
	}
	runtime, err = cfg.ResolveRuntime(ClientHermes, "responses-only")
	if err != nil || runtime.Protocol != ProtocolOpenAIResponses {
		t.Fatalf("explicit Route did not replace the previous incompatible protocol: %#v, %v", runtime, err)
	}

	invalid := cfg.Clone()
	binding := invalid.Clients[ClientHermes]
	binding.Protocol = ProtocolOpenAIResponses
	invalid.Clients[ClientHermes] = binding
	if err := invalid.Validate(); err == nil || !strings.Contains(err.Error(), "does not admit") {
		t.Fatalf("unsupported Route protocol error = %v", err)
	}
}

func TestRouteInterfacesRejectUnknownProtocols(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		interfaces map[EndpointProtocol][]Capability
	}{
		{name: "unknown", interfaces: map[EndpointProtocol][]Capability{"future": {}}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := validConfig()
			route := cfg.Routes["dmx"]
			route.Interfaces = testCase.interfaces
			cfg.Routes["dmx"] = route
			if err := cfg.Validate(); err == nil {
				t.Fatalf("accepted invalid Route interfaces: %v", testCase.interfaces)
			}
		})
	}
}

func TestRouteInterfacesKeepProtocolAndCapabilitySeparate(t *testing.T) {
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
			route := cfg.Routes["backup"]
			route.Interfaces = map[EndpointProtocol][]Capability{ProtocolOpenAIResponses: testCase.capabilities}
			cfg.Routes["backup"] = route
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

func TestRouteCapabilitiesRemainScopedToTheExactWireInterface(t *testing.T) {
	route := Route{
		Account: "team",
		Model:   "model",
		Interfaces: map[EndpointProtocol][]Capability{
			ProtocolOpenAIResponses:       {CapabilityText, CapabilityReasoning},
			ProtocolOpenAIChatCompletions: {CapabilityText},
		},
	}
	data, err := toml.Marshal(route)
	if err != nil {
		t.Fatal(err)
	}
	var restored Route
	if err := toml.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(restored.Interfaces[ProtocolOpenAIResponses], CapabilityReasoning) {
		t.Fatal("Responses reasoning qualification was lost")
	}
	if slices.Contains(restored.Interfaces[ProtocolOpenAIChatCompletions], CapabilityReasoning) {
		t.Fatal("Responses reasoning qualification leaked into Chat Completions")
	}
}

func TestManifestEquivalenceIncludesEveryEndpointAndProtocol(t *testing.T) {
	left := Account{Label: "Team", Endpoints: Endpoints{OpenAIChatCompletions: "https://first.test/v1"}}
	right := left
	right.Endpoints.OpenAIChatCompletions = "https://second.test/v1"
	if equivalentAccount(left, right) {
		t.Fatal("manifest import treated different Chat Completions endpoints as equivalent")
	}
	profile := Route{Account: "team", Model: "model", Interfaces: map[EndpointProtocol][]Capability{ProtocolOpenAIResponses: {}}}
	if !equivalentRoute(profile, profile) {
		t.Fatal("manifest import treated equal provider model identities as different")
	}
}
