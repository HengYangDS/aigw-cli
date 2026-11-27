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
	if err := cfg.SetForwardingRoot(ClientCodex, "http://127.0.0.1:8792//"); err != nil {
		t.Fatal(err)
	}
	if root := cfg.Clients[ClientCodex].ForwardingRoot; root != "http://127.0.0.1:8792" {
		t.Fatalf("the root keeps its trailing slashes: %q", root)
	}
	forwarded, err := cfg.ResolveRuntime(ClientCodex, ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	if forwarded.Endpoint != "http://127.0.0.1:8792/https/backup.test/v1" || forwarded.UpstreamEndpoint != direct.Endpoint {
		t.Fatalf("forwarded endpoint identities = %#v", forwarded)
	}
	if forwarded.ProviderID != direct.ProviderID || forwarded.UpstreamModel != direct.UpstreamModel || forwarded.CredentialProjectionFingerprint(ClientCodex) != direct.CredentialProjectionFingerprint(ClientCodex) {
		t.Fatalf("client forwarding changed the upstream credential or model identity: %#v", forwarded)
	}
	claude, err := cfg.ResolveRuntime(ClientClaude, ModelRef{})
	if err != nil || claude.Endpoint != cfg.Providers["dmx"].Endpoints.Anthropic {
		t.Fatalf("unrelated client changed: %#v, %v", claude, err)
	}
}

// A root that is not HTTPS or loopback HTTP, or that carries user
// information, a query or a fragment, is refused when it is set and when a
// file holds it, and the refusal names no part of it. Forwarding without a
// selection is refused too.
func TestClientForwardingRequiresAValidRoot(t *testing.T) {
	for _, root := range []string{"http://remote.test:8792", "https://user:password@proxy.test", "https://proxy.test/?api_key=secret", "https://proxy.test/#part", "proxy.test:8792"} {
		t.Run(root, func(t *testing.T) {
			cfg := validConfig()
			if err := cfg.SetForwardingRoot(ClientCodex, root); err == nil || strings.Contains(err.Error(), root) {
				t.Fatalf("an invalid forwarding root was set or named: %v", err)
			}
			if cfg.Clients[ClientCodex].ForwardingRoot != "" {
				t.Fatal("a refused root changed the binding")
			}
			// A file can hold the same root; validation refuses it there.
			binding := cfg.Clients[ClientCodex]
			binding.ForwardingRoot = root
			cfg.Clients[ClientCodex] = binding
			if err := cfg.Validate(); err == nil || strings.Contains(err.Error(), root) {
				t.Fatalf("an invalid forwarding root was accepted or named: %v", err)
			}
		})
	}
	for _, root := range []string{"https://proxy.test", "http://localhost:8792", "http://[::1]:8792/prefix", "http://127.0.0.2:8792"} {
		if err := ValidateForwardingRoot(root); err != nil {
			t.Errorf("the root %s was refused: %v", root, err)
		}
	}
	cfg := validConfig()
	cfg.Clients[ClientCodex] = ClientBinding{ForwardingRoot: "http://127.0.0.1:8792"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("forwarding without a selected model was accepted")
	}
}

// Forwarding follows the upstream: an explicit model of another provider is
// tried through the forwarding with its own destination, and selecting it
// keeps the forwarding.
func TestClientForwardingFollowsAnotherProvider(t *testing.T) {
	cfg := validConfig()
	if err := cfg.SetForwardingRoot(ClientCodex, "http://127.0.0.1:8792"); err != nil {
		t.Fatal(err)
	}
	cfg.Models[NewModelRef("dmx", "other-model")] = testModel("Other", "dmx", "other-model", ProtocolResponses)
	qualified, err := cfg.ResolveRuntime(ClientCodex, NewModelRef("dmx", "other-model"))
	if err != nil || qualified.Endpoint != "http://127.0.0.1:8792/https/example.test/v1" || qualified.UpstreamEndpoint != cfg.Providers["dmx"].Endpoints.Responses {
		t.Fatalf("an explicit model of another provider = %#v, %v", qualified, err)
	}
	cfg.SetSelectedModel(ClientCodex, NewModelRef("dmx", "other-model"), "")
	if cfg.Clients[ClientCodex].ForwardingRoot == "" {
		t.Fatal("selecting another provider's model dropped the forwarding")
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestClientForwardingSurvivesEquivalentAutomaticProtocolSelection(t *testing.T) {
	cfg := validConfig()
	if err := cfg.SetForwardingRoot(ClientCodex, "http://127.0.0.1:8792"); err != nil {
		t.Fatal(err)
	}
	binding := cfg.Clients[ClientCodex]
	cfg.SetSelectedModel(ClientCodex, binding.Model, "")
	if cfg.Clients[ClientCodex].ForwardingRoot != binding.ForwardingRoot {
		t.Fatal("normalizing the same implicit protocol discarded client forwarding")
	}
}

func TestCurrentSchemaKeepsClientConcernsOutOfModels(t *testing.T) {
	if ConfigVersion != 9 {
		t.Fatalf("config version = %d, want 9", ConfigVersion)
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

func TestSetClientActivationPreservesSelectionAndClientOptions(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Clients[ClientCodex] = ClientBinding{
		Model:             NewModelRef("team", "reasoning-model"),
		Protocol:          ProtocolResponses,
		CredentialCommand: "/existing/helper",
	}

	cfg.SetClientActivation(ClientCodex, true, "/opt/codex", []string{"/one/config.toml", "/two/config.toml"})

	got := cfg.Clients[ClientCodex]
	if got.Model != NewModelRef("team", "reasoning-model") || got.Protocol != ProtocolResponses || got.CredentialCommand != "/existing/helper" {
		t.Fatalf("activation replaced client selection or options: %#v", got)
	}
	if !got.Enabled || got.Program != "/opt/codex" || !reflect.DeepEqual(got.Targets, []string{"/one/config.toml", "/two/config.toml"}) {
		t.Fatalf("activation state = %#v", got)
	}
}
