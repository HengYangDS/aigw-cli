package domain_test

import (
	"reflect"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

func TestAdmittedClientRegistryIsTheSingleProtocolBoundary(t *testing.T) {
	want := []domain.ClientSpec{
		{ID: domain.ClientClaude, Label: "Claude", EndpointProtocol: domain.ProtocolAnthropic, ModelSlot: domain.ModelSlotClaude},
		{ID: domain.ClientCodex, Label: "Codex", EndpointProtocol: domain.ProtocolOpenAIResponses, ModelSlot: domain.ModelSlotCodex},
	}
	if got := domain.AdmittedClientSpecs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("client specs = %#v, want %#v", got, want)
	}
	if got := domain.AdmittedClientIDs(); !reflect.DeepEqual(got, []string{domain.ClientClaude, domain.ClientCodex}) {
		t.Fatalf("client ids = %#v", got)
	}
	if !domain.IsAdmittedClient(domain.ClientClaude) || domain.IsAdmittedClient("gemini") {
		t.Fatal("registry must admit only implemented adapters")
	}
	if spec, ok := domain.ClientSpecFor(domain.ClientCodex); !ok || spec.EndpointProtocol != domain.ProtocolOpenAIResponses {
		t.Fatalf("Codex client spec = %#v, %v", spec, ok)
	}
}

func TestAdmittedClientRegistryReturnsDefensiveCopies(t *testing.T) {
	clients := domain.AdmittedClientSpecs()
	clients[0].ID = "mutated"
	if domain.AdmittedClientSpecs()[0].ID != domain.ClientClaude {
		t.Fatal("caller mutation changed the registered client boundary")
	}
}
