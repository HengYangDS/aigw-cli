package hermes

import (
	"slices"
	"testing"

	"aigw-cli/internal/domain"
)

// TestHermesProviderIDsJoinTheProviderAndTheProtocolName pins the Hermes
// provider IDs of provider dmxapi: aigw, the provider ID and the protocol's
// own name, built without a table of names, one per protocol that its models
// declare, in the protocols' order.
func TestHermesProviderIDsJoinTheProviderAndTheProtocolName(t *testing.T) {
	cfg := domain.NewConfig(testAdmission())
	cfg.Providers["dmxapi"] = domain.Provider{Label: "DMXAPI", Endpoints: domain.Endpoints{
		Anthropic: "https://www.dmxapi.cn", Responses: "https://www.dmxapi.cn/v1", Chat: "https://www.dmxapi.cn/v1",
	}}
	model := domain.Model{Provider: "dmxapi", ID: "gpt-6.1-sol", Protocols: map[domain.EndpointProtocol][]domain.Capability{
		domain.ProtocolChat: {}, domain.ProtocolResponses: {}, domain.ProtocolAnthropic: {},
	}}
	cfg.Models[model.Ref()] = model
	catalog, err := hermesCatalog(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(catalog))
	for _, provider := range catalog {
		ids = append(ids, provider.ID)
	}
	if want := []string{"aigw-dmxapi-anthropic", "aigw-dmxapi-responses", "aigw-dmxapi-chat"}; !slices.Equal(ids, want) {
		t.Fatalf("Hermes provider IDs = %q, want %q", ids, want)
	}
	for _, protocol := range []domain.EndpointProtocol{domain.ProtocolAnthropic, domain.ProtocolResponses, domain.ProtocolChat} {
		if got := hermesProviderID("dmxapi", protocol); got != "aigw-dmxapi-"+protocol.Name() {
			t.Errorf("the Hermes provider of %s = %q", protocol, got)
		}
	}
}
