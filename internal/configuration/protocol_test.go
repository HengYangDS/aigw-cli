package configuration

import (
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestProfileSelectsProtocolIndependentlyOfClientBrand(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Team", Endpoints: Endpoints{OpenAIResponses: "https://responses.test/v1", Anthropic: "https://messages.test", OpenAIChatCompletions: "https://chat.test/v1"}}
	cfg.Profiles["hermes"] = Profile{Label: "Hermes", Account: "team", Model: "model-test"}
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
	if _, err := cfg.ResolveRuntime("hermes", ""); err == nil || !strings.Contains(err.Error(), "protocol") {
		t.Fatalf("ambiguous endpoint did not require protocol selection: %v", err)
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

func TestManifestEquivalenceIncludesEveryEndpointAndProtocol(t *testing.T) {
	left := Account{Label: "Team", Endpoints: Endpoints{OpenAIChatCompletions: "https://first.test/v1"}}
	right := left
	right.Endpoints.OpenAIChatCompletions = "https://second.test/v1"
	if equivalentAccount(left, right) {
		t.Fatal("manifest import treated different Chat Completions endpoints as equivalent")
	}
	profile := Profile{Account: "team", Model: "model"}
	if !equivalentProfile(profile, profile) {
		t.Fatal("manifest import treated equal provider model identities as different")
	}
}
