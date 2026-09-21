package configuration

import (
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestProfileSelectsProtocolIndependentlyOfClientBrand(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Team", Endpoints: Endpoints{OpenAIResponses: "https://responses.test/v1", Anthropic: "https://messages.test", OpenAIChatCompletions: "https://chat.test/v1"}}
	cfg.Profiles["hermes"] = Profile{Label: "Hermes", Account: "team", Model: "model-test", Protocols: []EndpointProtocol{ProtocolAnthropic}}
	cfg.Clients[ClientHermes] = ClientBinding{Profile: "hermes", Protocol: ProtocolAnthropic}
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
		t.Fatal("profile protocol did not survive round-trip")
	}
	binding := cfg.Clients[ClientHermes]
	binding.Protocol = ""
	cfg.Clients[ClientHermes] = binding
	if runtime, err := cfg.ResolveRuntime("hermes", ""); err != nil || runtime.Protocol != ProtocolAnthropic {
		t.Fatalf("Profile capability did not resolve the sole admitted protocol: %#v, %v", runtime, err)
	}
	profile := cfg.Profiles["hermes"]
	profile.Protocols = nil
	cfg.Profiles["hermes"] = profile
	if _, err := cfg.ResolveRuntime("hermes", ""); err == nil || !strings.Contains(err.Error(), "protocol") {
		t.Fatalf("unqualified ambiguous Profile did not require protocol selection: %v", err)
	}
	cfg.Accounts["team"] = Account{Label: "Team", Endpoints: Endpoints{OpenAIChatCompletions: "https://chat.test/v1"}}
	runtime, err = cfg.ResolveRuntime("hermes", "")
	if err != nil || runtime.Protocol != ProtocolOpenAIChatCompletions {
		t.Fatalf("only available protocol = %#v, %v", runtime, err)
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

func TestProfileProtocolsConstrainClientCompatibilityAndRuntimeSelection(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Team", Endpoints: Endpoints{
		OpenAIResponses:       "https://responses.test/v1",
		OpenAIChatCompletions: "https://chat.test/v1",
	}}
	cfg.Profiles["chat-only"] = Profile{
		Label:     "Chat only",
		Account:   "team",
		Model:     "model-test",
		Protocols: []EndpointProtocol{ProtocolOpenAIChatCompletions},
	}
	cfg.Clients[ClientHermes] = ClientBinding{Profile: "chat-only", Protocol: ProtocolOpenAIChatCompletions}

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
	cfg.Profiles["responses-only"] = Profile{
		Label:     "Responses only",
		Account:   "team",
		Model:     "another-model",
		Protocols: []EndpointProtocol{ProtocolOpenAIResponses},
	}
	runtime, err = cfg.ResolveRuntime(ClientHermes, "responses-only")
	if err != nil || runtime.Protocol != ProtocolOpenAIResponses {
		t.Fatalf("explicit Profile did not replace the previous incompatible protocol: %#v, %v", runtime, err)
	}

	invalid := cfg.Clone()
	binding := invalid.Clients[ClientHermes]
	binding.Protocol = ProtocolOpenAIResponses
	invalid.Clients[ClientHermes] = binding
	if err := invalid.Validate(); err == nil || !strings.Contains(err.Error(), "does not admit") {
		t.Fatalf("unsupported Profile protocol error = %v", err)
	}
}

func TestProfileProtocolsRequireAUniqueKnownSet(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		protocols []EndpointProtocol
	}{
		{name: "empty", protocols: []EndpointProtocol{}},
		{name: "duplicate", protocols: []EndpointProtocol{ProtocolOpenAIResponses, ProtocolOpenAIResponses}},
		{name: "unknown", protocols: []EndpointProtocol{"future"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := validConfig()
			profile := cfg.Profiles["dmx"]
			profile.Protocols = testCase.protocols
			cfg.Profiles["dmx"] = profile
			if err := cfg.Validate(); err == nil {
				t.Fatalf("accepted invalid Profile protocols: %v", testCase.protocols)
			}
		})
	}
}

func TestProfileTierUsesTheCuratedFlagshipOrDailyVocabulary(t *testing.T) {
	for _, tier := range []ModelTier{"", ModelTierFlagship, ModelTierDaily} {
		cfg := validConfig()
		profile := cfg.Profiles["dmx"]
		profile.Tier = tier
		cfg.Profiles["dmx"] = profile
		if err := cfg.Validate(); err != nil {
			t.Fatalf("tier %q: %v", tier, err)
		}
	}
	cfg := validConfig()
	profile := cfg.Profiles["dmx"]
	profile.Tier = "premium"
	cfg.Profiles["dmx"] = profile
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "tier") {
		t.Fatalf("unknown tier error = %v", err)
	}
}

func TestManifestEquivalenceIncludesEveryEndpointAndProtocol(t *testing.T) {
	left := Account{Label: "Team", Endpoints: Endpoints{OpenAIChatCompletions: "https://first.test/v1"}}
	right := left
	right.Endpoints.OpenAIChatCompletions = "https://second.test/v1"
	if equivalentAccount(left, right) {
		t.Fatal("manifest import treated different Chat Completions endpoints as equivalent")
	}
	profile := Profile{Account: "team", Model: "model", Protocols: []EndpointProtocol{ProtocolOpenAIResponses}}
	if !equivalentProfile(profile, profile) {
		t.Fatal("manifest import treated equal provider model identities as different")
	}
}
