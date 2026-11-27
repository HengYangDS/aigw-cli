package hermes

import (
	"path/filepath"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
)

// A forwarded Hermes lists each provider's Responses entry at the
// destination that the forwarding composes from that provider's endpoint,
// since Hermes can switch to any listed provider within a session; the
// forwarding service relays no other protocol, so the other entries keep
// their providers' endpoints. Another provider whose Responses endpoint the
// grammar cannot name, here one with a query, is listed at that endpoint:
// the forwarding refuses only the selection that it cannot carry.
func TestHermesListsEachResponsesEntryThroughTheForwarding(t *testing.T) {
	cfg := domain.NewConfig(testAdmission())
	if err := cfg.AppendProvider("dmxapi", domain.Provider{Label: "DMXAPI", Endpoints: domain.Endpoints{
		Responses: "https://www.dmxapi.cn/v1", Anthropic: "https://www.dmxapi.cn",
	}}); err != nil {
		t.Fatal(err)
	}
	if err := cfg.AppendProvider("suheai", domain.Provider{Label: "Suhe.AI", Endpoints: domain.Endpoints{Responses: "https://suheai.cn/v1"}}); err != nil {
		t.Fatal(err)
	}
	const outside = "https://azure.test/openai/v1?api-version=preview"
	if err := cfg.AppendProvider("azure", domain.Provider{Label: "Azure", Endpoints: domain.Endpoints{Responses: outside}}); err != nil {
		t.Fatal(err)
	}
	for _, model := range []domain.Model{
		{Provider: "dmxapi", ID: "gpt-6.2-sol", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolResponses: {}}},
		{Provider: "dmxapi", ID: "claude-opus-5-5", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}},
		{Provider: "suheai", ID: "gpt-6.2-sol", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolResponses: {}}},
		{Provider: "azure", ID: "gpt-6.2-sol", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolResponses: {}}},
	} {
		if err := cfg.AppendModel(model); err != nil {
			t.Fatal(err)
		}
	}
	cfg.SetSelectedModel(ID, domain.NewModelRef("dmxapi", "gpt-6.2-sol"), domain.ProtocolResponses)
	if err := cfg.SetForwardingRoot(ID, "http://127.0.0.1:8792"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("the forwarded configuration is refused: %v", err)
	}
	selected, err := cfg.ResolveRuntime(ID, domain.ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	store := secret.NewMemoryStore()
	for _, provider := range []string{"dmxapi", "suheai", "azure"} {
		if err := store.Set(provider, "fixture-token"); err != nil {
			t.Fatal(err)
		}
	}
	desired, err := hermesDesired(adapter.Dependencies{Secrets: store, AIGWExecutable: filepath.Join(t.TempDir(), "aigw")}, cfg, selected)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"aigw-dmxapi-responses": "http://127.0.0.1:8792/https/www.dmxapi.cn/v1",
		"aigw-dmxapi-anthropic": "https://www.dmxapi.cn",
		"aigw-suheai-responses": "http://127.0.0.1:8792/https/suheai.cn/v1",
		"aigw-azure-responses":  outside,
	}
	if len(desired.Providers) != len(want) {
		t.Fatalf("Hermes lists %#v, want %v", desired.Providers, want)
	}
	for _, provider := range desired.Providers {
		if provider.Endpoint != want[provider.ID] {
			t.Errorf("Hermes lists %s at %s, want %s", provider.ID, provider.Endpoint, want[provider.ID])
		}
	}
}
