package domain

import (
	"reflect"
	"strings"
	"testing"
)

func TestClientForwardingPreservesUpstreamCredentialIdentity(t *testing.T) {
	cfg := validConfig()
	direct, err := cfg.ResolveRuntime(ClientCodex, ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SetForwardingEndpoint(ClientCodex, "http://127.0.0.1:8792/backup/v1/"); err != nil {
		t.Fatal(err)
	}
	binding := cfg.Clients[ClientCodex]
	forwarded, err := cfg.ResolveRuntime(ClientCodex, ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	if forwarded.Endpoint != strings.TrimRight(binding.ForwardingEndpoint, "/") || forwarded.UpstreamEndpoint != direct.Endpoint {
		t.Fatalf("forwarded endpoint identities = %#v", forwarded)
	}
	if forwarded.ProviderID != direct.ProviderID || forwarded.UpstreamModel != direct.UpstreamModel || forwarded.ModelProvider != direct.ModelProvider || forwarded.CredentialProjectionFingerprint(ClientCodex) != direct.CredentialProjectionFingerprint(ClientCodex) {
		t.Fatalf("client forwarding changed the upstream credential or model identity: %#v", forwarded)
	}
	claude, err := cfg.ResolveRuntime(ClientClaude, ModelRef{})
	if err != nil || claude.Endpoint != cfg.Providers["dmx"].Endpoints.Anthropic {
		t.Fatalf("unrelated client changed: %#v, %v", claude, err)
	}
}

func TestClientForwardingRequiresAValidSelectedEndpoint(t *testing.T) {
	for _, endpoint := range []string{"http://remote.test/v1", "https://user:password@example.test/v1", "https://example.test/v1?api_key=secret"} {
		t.Run(endpoint, func(t *testing.T) {
			cfg := validConfig()
			if err := cfg.SetForwardingEndpoint(ClientCodex, endpoint); err == nil {
				t.Fatal("an invalid forwarding endpoint was selected")
			}
			// A sidecar can carry the same endpoint; validation refuses it there.
			binding := cfg.Clients[ClientCodex]
			binding.ForwardingEndpoint = endpoint
			cfg.Clients[ClientCodex] = binding
			if err := cfg.Validate(); err == nil {
				t.Fatal("invalid forwarding endpoint was accepted")
			}
		})
	}
	cfg := validConfig()
	cfg.Clients[ClientCodex] = ClientBinding{ForwardingEndpoint: "http://127.0.0.1:8792/v1"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("forwarding without a selected Route was accepted")
	}
}

func TestClientForwardingDoesNotRedirectAnExplicitOtherProvider(t *testing.T) {
	cfg := validConfig()
	if err := cfg.SetForwardingEndpoint(ClientCodex, "http://127.0.0.1:8792/backup/v1"); err != nil {
		t.Fatal(err)
	}
	cfg.Models[NewModelRef("dmx", "other-model")] = testModel("Other", "dmx", "other-model", ProtocolResponses)
	qualified, err := cfg.ResolveRuntime(ClientCodex, NewModelRef("dmx", "other-model"))
	if err != nil || qualified.Endpoint != cfg.Providers["dmx"].Endpoints.Responses || qualified.ProviderID != "dmx" {
		t.Fatalf("explicit other-Account qualification inherited the selected forwarding destination: %#v, %v", qualified, err)
	}
	cfg.SetSelectedModel(ClientCodex, NewModelRef("dmx", "other-model"), "")
	if cfg.Clients[ClientCodex].ForwardingEndpoint != "" {
		t.Fatal("switching Account retained an unrelated forwarding destination")
	}
}

func TestClientForwardingSurvivesEquivalentAutomaticProtocolSelection(t *testing.T) {
	cfg := validConfig()
	if err := cfg.SetForwardingEndpoint(ClientCodex, "http://127.0.0.1:8792/backup/v1"); err != nil {
		t.Fatal(err)
	}
	binding := cfg.Clients[ClientCodex]
	cfg.SetSelectedModel(ClientCodex, binding.Model, "")
	if cfg.Clients[ClientCodex].ForwardingEndpoint != binding.ForwardingEndpoint {
		t.Fatal("normalizing the same implicit protocol discarded client forwarding")
	}
}

func TestCurrentSchemaKeepsClientConcernsOutOfModels(t *testing.T) {
	if ConfigVersion != 8 {
		t.Fatalf("config version = %d, want 8", ConfigVersion)
	}
	for _, field := range []string{"Client", "ModelProvider", "Authentication", "UpstreamModel", "Purpose", "Lifecycle"} {
		if _, exists := reflect.TypeFor[Model]().FieldByName(field); exists {
			t.Errorf("Model still has %s", field)
		}
	}
}

func TestClientBindingOwnsSelectionAndClientSpecificOptions(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["team"] = Provider{
		Label: "Team",
		Endpoints: Endpoints{
			Anthropic: "https://team.test/anthropic",
			Chat:      "https://team.test/openai/v1",
		},
	}
	cfg.Models[NewModelRef("team", "reasoning-model")] = Model{
		Label:    "Reasoning",
		Provider: "team",
		Family:   "reasoning-model", ID: "reasoning-model",
		Protocols: map[EndpointProtocol][]Capability{
			ProtocolAnthropic: {CapabilityText},
			ProtocolChat:      {CapabilityText},
		},
	}
	cfg.Clients[ClientClaude] = ClientBinding{
		Enabled:  true,
		Model:    NewModelRef("team", "reasoning-model"),
		Protocol: ProtocolAnthropic,
	}
	cfg.Clients[ClientHermes] = ClientBinding{
		Enabled:  true,
		Model:    NewModelRef("team", "reasoning-model"),
		Protocol: ProtocolAnthropic,
	}

	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	claude, err := cfg.ResolveRuntime(ClientClaude, ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	hermes, err := cfg.ResolveRuntime(ClientHermes, ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	if claude.Model != NewModelRef("team", "reasoning-model") || hermes.Model != NewModelRef("team", "reasoning-model") {
		t.Fatalf("shared Route resolution = %#v, %#v", claude, hermes)
	}
	if claude.Client != ClientClaude || hermes.Client != ClientHermes {
		t.Fatalf("client identity leaked from Route = %#v, %#v", claude, hermes)
	}
}

func TestClientBindingOwnsNativeAuthentication(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["native"] = Provider{
		Label:     "Native",
		Endpoints: Endpoints{Responses: "https://native.test/v1"},
	}
	cfg.Models[NewModelRef("native", "model-id")] = testModel("Model", "native", "model-id", ProtocolResponses)
	cfg.Clients[ClientCodex] = ClientBinding{
		Enabled:        true,
		Model:          NewModelRef("native", "model-id"),
		Protocol:       ProtocolResponses,
		ModelProvider:  "amazon-bedrock",
		Authentication: AuthenticationClientNative,
	}

	cfg.SetSelectedModel(ClientCodex, NewModelRef("native", "model-id"), ProtocolResponses)
	runtime, err := cfg.ResolveRuntime(ClientCodex, ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	if runtime.ModelProvider != "amazon-bedrock" || runtime.Authentication != AuthenticationClientNative {
		t.Fatalf("client-specific runtime options = %#v", runtime)
	}
	if runtime.RequiresProviderToken() {
		t.Fatal("client-native binding unexpectedly requires an Account Token")
	}
}

func TestSetClientActivationPreservesSelectionAndClientOptions(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Clients[ClientCodex] = ClientBinding{
		Model:             NewModelRef("team", "reasoning-model"),
		Protocol:          ProtocolResponses,
		ModelProvider:     "provider",
		Authentication:    AuthenticationClientNative,
		CredentialCommand: "/existing/helper",
	}

	cfg.SetClientActivation(ClientCodex, true, "/opt/codex", []string{"/one/config.toml", "/two/config.toml"})

	got := cfg.Clients[ClientCodex]
	if got.Model != NewModelRef("team", "reasoning-model") || got.Protocol != ProtocolResponses || got.ModelProvider != "provider" || got.Authentication != AuthenticationClientNative || got.CredentialCommand != "/existing/helper" {
		t.Fatalf("activation replaced client selection or options: %#v", got)
	}
	if !got.Enabled || got.Executable != "/opt/codex" || !reflect.DeepEqual(got.Targets, []string{"/one/config.toml", "/two/config.toml"}) {
		t.Fatalf("activation state = %#v", got)
	}
}
