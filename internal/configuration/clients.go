package configuration

type EndpointProtocol string

const (
	ProtocolAnthropic       EndpointProtocol = "anthropic"
	ProtocolOpenAIResponses EndpointProtocol = "openai_responses"
)

// ClientSpec is the canonical admission record for an implemented client
// adapter. The current product admits Claude and Codex for its enterprise use
// case; this registry is extensible, but a provider or model alone never admits
// another client. A future adapter must prove its own configuration, credential,
// protocol, verification, rollback, and uninstall boundaries.
type ClientSpec struct {
	ID               string
	Label            string
	EndpointProtocol EndpointProtocol
}

var admittedClientSpecs = []ClientSpec{
	{ID: ClientClaude, Label: "Claude", EndpointProtocol: ProtocolAnthropic},
	{ID: ClientCodex, Label: "Codex", EndpointProtocol: ProtocolOpenAIResponses},
}

func AdmittedClientSpecs() []ClientSpec {
	return append([]ClientSpec(nil), admittedClientSpecs...)
}

func AdmittedClientIDs() []string {
	clients := make([]string, 0, len(admittedClientSpecs))
	for _, spec := range admittedClientSpecs {
		clients = append(clients, spec.ID)
	}
	return clients
}

func ClientSpecFor(id string) (ClientSpec, bool) {
	for _, spec := range admittedClientSpecs {
		if spec.ID == id {
			return spec, true
		}
	}
	return ClientSpec{}, false
}

func IsAdmittedClient(id string) bool {
	_, ok := ClientSpecFor(id)
	return ok
}
