package configuration

import "testing"

func TestClientBindingOwnsSelectionAndClientSpecificOptions(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{
		Label: "Team",
		Endpoints: Endpoints{
			Anthropic:             "https://team.test/anthropic",
			OpenAIChatCompletions: "https://team.test/openai/v1",
		},
	}
	cfg.Profiles["reasoning"] = Profile{
		Label:   "Reasoning",
		Account: "team",
		Model:   "reasoning-model",
	}
	cfg.Clients[ClientClaude] = ClientBinding{
		Enabled:  true,
		Profile:  "reasoning",
		Protocol: ProtocolAnthropic,
	}
	cfg.Clients[ClientHermes] = ClientBinding{
		Enabled:  true,
		Profile:  "reasoning",
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
	if claude.ProfileID != "reasoning" || hermes.ProfileID != "reasoning" {
		t.Fatalf("shared profile resolution = %#v, %#v", claude, hermes)
	}
	if claude.Client != ClientClaude || hermes.Client != ClientHermes {
		t.Fatalf("client identity leaked from Profile = %#v, %#v", claude, hermes)
	}
}

func TestClientBindingOwnsNativeAuthentication(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["native"] = Account{
		Label:     "Native",
		Endpoints: Endpoints{OpenAIResponses: "https://native.test/v1"},
	}
	cfg.Profiles["model"] = Profile{Label: "Model", Account: "native", Model: "model-id"}
	cfg.Clients[ClientCodex] = ClientBinding{
		Enabled:        true,
		Profile:        "model",
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
