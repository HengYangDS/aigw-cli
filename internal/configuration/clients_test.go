package configuration

import (
	"reflect"
	"testing"
)

func TestAdmittedClientRegistryIsTheSingleProtocolBoundary(t *testing.T) {
	want := []ClientSpec{
		{ID: ClientClaude, Label: "Claude", EndpointProtocol: ProtocolAnthropic},
		{ID: ClientCodex, Label: "Codex", EndpointProtocol: ProtocolOpenAIResponses},
	}
	if got := AdmittedClientSpecs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("client specs = %#v, want %#v", got, want)
	}
	if got := AdmittedClientIDs(); !reflect.DeepEqual(got, []string{ClientClaude, ClientCodex}) {
		t.Fatalf("client ids = %#v", got)
	}
	if !IsAdmittedClient(ClientClaude) || IsAdmittedClient("gemini") {
		t.Fatal("registry must admit only implemented adapters")
	}
	if spec, ok := ClientSpecFor(ClientCodex); !ok || spec.EndpointProtocol != ProtocolOpenAIResponses {
		t.Fatalf("Codex client spec = %#v, %v", spec, ok)
	}
}

func TestAdmittedClientRegistryReturnsDefensiveCopies(t *testing.T) {
	clients := AdmittedClientSpecs()
	clients[0].ID = "mutated"
	if AdmittedClientSpecs()[0].ID != ClientClaude {
		t.Fatal("caller mutation changed the registered client boundary")
	}
}

func TestProfileModelsUseAdmittedClientIDsAsKeys(t *testing.T) {
	profile := Profile{Models: Models{
		ClientClaude: "claude-test",
		ClientCodex:  "gpt-test",
	}}
	if profile.ModelFor(ClientClaude) != "claude-test" || profile.ModelFor(ClientCodex) != "gpt-test" {
		t.Fatalf("models = %#v", profile.Models)
	}
}
