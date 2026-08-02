package configuration

import (
	"fmt"
	"strings"
)

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

// AdmittedClientUsage renders the stable client choices for CLI help and
// errors. Extra choices are command-local values such as "all"; admitted
// client IDs remain owned by this registry.
func AdmittedClientUsage(extra ...string) string {
	return naturalChoices(append(AdmittedClientIDs(), extra...))
}

// AdmittedClientLabelUsage renders presentation labels while preserving
// command-local extra values such as "all".
func AdmittedClientLabelUsage(extra ...string) string {
	labels := make([]string, 0, len(admittedClientSpecs)+len(extra))
	for _, spec := range admittedClientSpecs {
		labels = append(labels, spec.Label)
	}
	return naturalChoices(append(labels, extra...))
}

func naturalChoices(choices []string) string {
	switch len(choices) {
	case 0:
		return ""
	case 1:
		return choices[0]
	case 2:
		return strings.Join(choices, " or ")
	default:
		return strings.Join(choices[:len(choices)-1], ", ") + ", or " + choices[len(choices)-1]
	}
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

// Endpoint resolves this client's declared protocol endpoint from an Account.
// Client identity selects protocol only here; provider identity never changes
// client behavior.
func (s ClientSpec) Endpoint(account Account) (string, error) {
	var endpoint string
	switch s.EndpointProtocol {
	case ProtocolAnthropic:
		endpoint = account.Endpoints.Anthropic
	case ProtocolOpenAIResponses:
		endpoint = account.Endpoints.OpenAIResponses
	default:
		return "", fmt.Errorf("client %q has unsupported endpoint protocol %q", s.ID, s.EndpointProtocol)
	}
	if endpoint == "" {
		return "", &RuntimeMissingEndpointError{AccountID: account.ID, Protocol: s.EndpointProtocol}
	}
	return strings.TrimRight(endpoint, "/"), nil
}
