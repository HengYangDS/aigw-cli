package domain

type EndpointProtocol string

const (
	ProtocolAnthropic       EndpointProtocol = "anthropic"
	ProtocolOpenAIResponses EndpointProtocol = "openai_responses"
)

type ModelSlot string

const (
	ModelSlotClaude ModelSlot = "claude"
	ModelSlotCodex  ModelSlot = "codex"
)

// ClientSpec is the canonical admission record for an implemented client
// adapter. Adding a candidate model or endpoint never adds a ClientSpec: the
// adapter must first prove its dedicated configuration, credential, protocol,
// verification, and rollback boundaries.
type ClientSpec struct {
	ID               string
	Label            string
	EndpointProtocol EndpointProtocol
	ModelSlot        ModelSlot
}

var admittedClientSpecs = []ClientSpec{
	{ID: ClientClaude, Label: "Claude", EndpointProtocol: ProtocolAnthropic, ModelSlot: ModelSlotClaude},
	{ID: ClientCodex, Label: "Codex", EndpointProtocol: ProtocolOpenAIResponses, ModelSlot: ModelSlotCodex},
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
