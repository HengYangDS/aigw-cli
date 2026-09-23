package configuration

import (
	"reflect"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestCurrentSchemaRemovesLegacySelectionFields(t *testing.T) {
	if ConfigVersion != 6 {
		t.Fatalf("config version = %d, want 6", ConfigVersion)
	}
	for _, field := range []string{"Profiles", "RecommendedRoutes"} {
		if _, exists := reflect.TypeFor[Config]().FieldByName(field); exists {
			t.Errorf("Config still exposes legacy field %s", field)
		}
	}
	for _, field := range []string{"Client", "ModelProvider", "Authentication"} {
		if _, exists := reflect.TypeFor[Route]().FieldByName(field); exists {
			t.Errorf("Route still exposes client concern %s", field)
		}
	}
}

func TestConfigSerializationUsesClientBindingsOnly(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{
		Label:     "Team",
		Endpoints: Endpoints{Anthropic: "https://team.test/anthropic"},
	}
	cfg.Routes["reasoning"] = Route{
		Label:   "Reasoning",
		Account: "team",
		Model:   "reasoning-model",
		Interfaces: map[EndpointProtocol][]Capability{
			ProtocolAnthropic:             {CapabilityText},
			ProtocolOpenAIChatCompletions: {CapabilityText},
		},
	}
	cfg.Clients[ClientClaude] = ClientBinding{
		Enabled:  true,
		Route:    "reasoning",
		Protocol: ProtocolAnthropic,
	}

	encoded, err := toml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := toml.Unmarshal(encoded, &document); err != nil {
		t.Fatal(err)
	}
	for _, legacy := range []string{"profiles", "recommended_routes", "adapters"} {
		if _, exists := document[legacy]; exists {
			t.Fatalf("serialized configuration retained legacy %q state:\n%s", legacy, encoded)
		}
	}
	clients, ok := document["clients"].(map[string]any)
	if !ok {
		t.Fatalf("serialized configuration has no clients table:\n%s", encoded)
	}
	if _, ok := clients[ClientClaude]; !ok {
		t.Fatalf("serialized configuration has no Claude binding:\n%s", encoded)
	}
	routes, ok := document["routes"].(map[string]any)
	if !ok {
		t.Fatalf("serialized configuration has no routes table:\n%s", encoded)
	}
	route, ok := routes["reasoning"].(map[string]any)
	if !ok {
		t.Fatalf("serialized configuration has no reasoning route:\n%s", encoded)
	}
	for _, clientConcern := range []string{"client", "protocol", "model_provider", "authentication"} {
		if _, exists := route[clientConcern]; exists {
			t.Fatalf("serialized Route retained client concern %q:\n%s", clientConcern, encoded)
		}
	}
}

func TestClientBindingOwnsSelectionAndClientSpecificOptions(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{
		Label: "Team",
		Endpoints: Endpoints{
			Anthropic:             "https://team.test/anthropic",
			OpenAIChatCompletions: "https://team.test/openai/v1",
		},
	}
	cfg.Routes["reasoning"] = Route{
		Label:   "Reasoning",
		Account: "team",
		Model:   "reasoning-model",
		Interfaces: map[EndpointProtocol][]Capability{
			ProtocolAnthropic:             {CapabilityText},
			ProtocolOpenAIChatCompletions: {CapabilityText},
		},
	}
	cfg.Clients[ClientClaude] = ClientBinding{
		Enabled:  true,
		Route:    "reasoning",
		Protocol: ProtocolAnthropic,
	}
	cfg.Clients[ClientHermes] = ClientBinding{
		Enabled:  true,
		Route:    "reasoning",
		Protocol: ProtocolAnthropic,
	}

	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	claude, err := cfg.ResolveRuntime(ClientClaude, "")
	if err != nil {
		t.Fatal(err)
	}
	hermes, err := cfg.ResolveRuntime(ClientHermes, "")
	if err != nil {
		t.Fatal(err)
	}
	if claude.RouteID != "reasoning" || hermes.RouteID != "reasoning" {
		t.Fatalf("shared Route resolution = %#v, %#v", claude, hermes)
	}
	if claude.Client != ClientClaude || hermes.Client != ClientHermes {
		t.Fatalf("client identity leaked from Route = %#v, %#v", claude, hermes)
	}
}

func TestClientBindingOwnsNativeAuthentication(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["native"] = Account{
		Label:     "Native",
		Endpoints: Endpoints{OpenAIResponses: "https://native.test/v1"},
	}
	cfg.Routes["model"] = testRoute("Model", "native", "model-id", ProtocolOpenAIResponses)
	cfg.Clients[ClientCodex] = ClientBinding{
		Enabled:        true,
		Route:          "model",
		Protocol:       ProtocolOpenAIResponses,
		ModelProvider:  "amazon-bedrock",
		Authentication: AuthenticationClientNative,
	}

	runtime, err := cfg.ResolveRuntime(ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if runtime.ModelProvider != "amazon-bedrock" || runtime.Authentication != AuthenticationClientNative {
		t.Fatalf("client-specific runtime options = %#v", runtime)
	}
	if runtime.RequiresAccountToken() {
		t.Fatal("client-native binding unexpectedly requires an Account Token")
	}
}

func TestSetClientActivationPreservesSelectionAndClientOptions(t *testing.T) {
	cfg := NewConfig()
	cfg.Clients[ClientCodex] = ClientBinding{
		Route:             "reasoning",
		Protocol:          ProtocolOpenAIResponses,
		ModelProvider:     "provider",
		Authentication:    AuthenticationClientNative,
		CredentialCommand: "/existing/helper",
	}

	cfg.SetClientActivation(ClientCodex, true, "/opt/codex", []string{"/one/config.toml", "/two/config.toml"})

	got := cfg.Clients[ClientCodex]
	if got.Route != "reasoning" || got.Protocol != ProtocolOpenAIResponses || got.ModelProvider != "provider" || got.Authentication != AuthenticationClientNative || got.CredentialCommand != "/existing/helper" {
		t.Fatalf("activation replaced client selection or options: %#v", got)
	}
	if !got.Enabled || got.Executable != "/opt/codex" || !reflect.DeepEqual(got.Targets, []string{"/one/config.toml", "/two/config.toml"}) {
		t.Fatalf("activation state = %#v", got)
	}
}
