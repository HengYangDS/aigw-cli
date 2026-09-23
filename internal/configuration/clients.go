package configuration

import (
	"fmt"
	"slices"
	"strings"
)

// EndpointProtocol identifies the upstream protocol required by an admitted client.
type EndpointProtocol string

const (
	// ProtocolAnthropic identifies the native Anthropic Messages-compatible endpoint.
	ProtocolAnthropic EndpointProtocol = "anthropic"
	// ProtocolOpenAIResponses identifies the OpenAI Responses-compatible endpoint.
	ProtocolOpenAIResponses EndpointProtocol = "openai_responses"
	// ProtocolOpenAIChatCompletions identifies OpenAI Chat Completions-compatible endpoints.
	ProtocolOpenAIChatCompletions EndpointProtocol = "openai_chat_completions"
)

// ClientSpec is the canonical admission record for an implemented client
// adapter. A provider or model alone never admits a client. Every adapter must
// prove its own configuration, credential, protocol, activation, verification,
// rollback, and uninstall boundaries.
type ClientSpec struct {
	ID                     string
	Label                  string
	EndpointProtocols      []EndpointProtocol
	RestartAfterProjection bool
	QualifiedModes         []string
	QualifiedPlatforms     []string
}

var admittedClientSpecs = []ClientSpec{
	{ID: ClientClaude, Label: "Claude", EndpointProtocols: []EndpointProtocol{ProtocolAnthropic}},
	{
		ID:                     ClientClaudeDesktop,
		Label:                  "Claude Desktop",
		EndpointProtocols:      []EndpointProtocol{ProtocolAnthropic},
		RestartAfterProjection: true,
		QualifiedModes:         []string{"Cowork", "Code"},
		QualifiedPlatforms:     []string{"macOS"},
	},
	{ID: ClientCodex, Label: "Codex", EndpointProtocols: []EndpointProtocol{ProtocolOpenAIResponses}},
	{ID: ClientHermes, Label: "Hermes", EndpointProtocols: []EndpointProtocol{ProtocolOpenAIResponses, ProtocolAnthropic, ProtocolOpenAIChatCompletions}},
}

// AdmittedClientSpecs returns an independent copy of every supported client contract.
func AdmittedClientSpecs() []ClientSpec {
	result := slices.Clone(admittedClientSpecs)
	for index := range result {
		result[index].EndpointProtocols = slices.Clone(result[index].EndpointProtocols)
		result[index].QualifiedModes = slices.Clone(result[index].QualifiedModes)
		result[index].QualifiedPlatforms = slices.Clone(result[index].QualifiedPlatforms)
	}
	return result
}

// AdmittedClientIDs returns admitted client identifiers in stable order.
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

// ClientSpecFor resolves one admitted client contract by identifier.
func ClientSpecFor(id string) (ClientSpec, bool) {
	for _, spec := range admittedClientSpecs {
		if spec.ID == id {
			spec.EndpointProtocols = slices.Clone(spec.EndpointProtocols)
			spec.QualifiedModes = slices.Clone(spec.QualifiedModes)
			spec.QualifiedPlatforms = slices.Clone(spec.QualifiedPlatforms)
			return spec, true
		}
	}
	return ClientSpec{}, false
}

// IsAdmittedClient reports whether an identifier has an admitted client contract.
func IsAdmittedClient(id string) bool {
	_, ok := ClientSpecFor(id)
	return ok
}

// ResolveEndpoint selects an explicitly requested protocol or the sole available
// supported endpoint. Multiple choices require a Route-level selection.
func (s ClientSpec) ResolveEndpoint(account Account, requested EndpointProtocol) (string, EndpointProtocol, error) {
	return s.resolveEndpoint(account, nil, requested)
}

// ResolveRouteEndpoint selects one endpoint admitted by the client, Account,
// and Route interface declaration.
func (s ClientSpec) ResolveRouteEndpoint(account Account, route Route, requested EndpointProtocol) (string, EndpointProtocol, error) {
	return s.resolveEndpoint(account, routeAdmittedProtocols(route), requested)
}

func (s ClientSpec) resolveEndpoint(account Account, admitted []EndpointProtocol, requested EndpointProtocol) (string, EndpointProtocol, error) {
	protocol := requested
	if protocol == "" && len(s.EndpointProtocols) == 1 {
		protocol = s.EndpointProtocols[0]
	}
	if protocol == "" {
		for _, candidate := range s.EndpointProtocols {
			if account.Endpoints.For(candidate) == "" || admitted != nil && !slices.Contains(admitted, candidate) {
				continue
			}
			if protocol != "" {
				return "", "", fmt.Errorf("client %q has multiple compatible endpoints; select a route protocol", s.ID)
			}
			protocol = candidate
		}
	}
	if protocol == "" {
		return "", "", fmt.Errorf("account %q has no compatible endpoint for client %q", account.ID, s.ID)
	}
	if !slices.Contains(s.EndpointProtocols, protocol) {
		return "", "", fmt.Errorf("client %q does not support endpoint protocol %q", s.ID, protocol)
	}
	if admitted != nil && !slices.Contains(admitted, protocol) {
		return "", "", fmt.Errorf("route does not admit endpoint protocol %q for client %q", protocol, s.ID)
	}
	switch protocol {
	case ProtocolAnthropic, ProtocolOpenAIResponses, ProtocolOpenAIChatCompletions:
	default:
		return "", "", fmt.Errorf("client %q has unsupported endpoint protocol %q", s.ID, protocol)
	}
	endpoint := account.Endpoints.For(protocol)
	if endpoint == "" {
		return "", "", &RuntimeMissingEndpointError{AccountID: account.ID, Protocol: protocol}
	}
	return strings.TrimRight(endpoint, "/"), protocol, nil
}

// CompatibleProtocols returns the protocols this client and Account both
// expose, preserving the client's declared preference order.
func (s ClientSpec) CompatibleProtocols(account Account) []EndpointProtocol {
	return s.compatibleProtocols(account, nil)
}

// CompatibleRouteProtocols returns the client protocols admitted by both
// the Account endpoints and the Route's verified interface declaration.
func (s ClientSpec) CompatibleRouteProtocols(account Account, route Route) []EndpointProtocol {
	return s.compatibleProtocols(account, routeAdmittedProtocols(route))
}

func routeAdmittedProtocols(route Route) []EndpointProtocol {
	return route.AdmittedProtocols()
}

func (s ClientSpec) compatibleProtocols(account Account, admitted []EndpointProtocol) []EndpointProtocol {
	protocols := make([]EndpointProtocol, 0, len(s.EndpointProtocols))
	for _, protocol := range s.EndpointProtocols {
		if account.Endpoints.For(protocol) != "" && (admitted == nil || slices.Contains(admitted, protocol)) {
			protocols = append(protocols, protocol)
		}
	}
	return protocols
}

// For returns the configured endpoint for one wire protocol.
func (endpoints Endpoints) For(protocol EndpointProtocol) string {
	switch protocol {
	case ProtocolAnthropic:
		return endpoints.Anthropic
	case ProtocolOpenAIResponses:
		return endpoints.OpenAIResponses
	case ProtocolOpenAIChatCompletions:
		return endpoints.OpenAIChatCompletions
	default:
		return ""
	}
}

// ClientSelection identifies one Route and the client-specific choices needed
// to resolve it without coupling the Route to a client brand.
type ClientSelection struct {
	Route          string           `json:"route,omitempty"              toml:"route,omitempty"`
	Protocol       EndpointProtocol `json:"protocol,omitempty"           toml:"protocol,omitempty"`
	ModelProvider  string           `json:"model_provider,omitempty"     toml:"model_provider,omitempty"`
	Authentication Authentication   `json:"authentication,omitempty"     toml:"authentication,omitempty"`
}

// ClientRecommendation orders reviewed choices without creating an active
// selection or a global ranking attached to a Model or Route.
type ClientRecommendation struct {
	Primary      ClientSelection   `json:"primary"                toml:"primary"`
	Alternatives []ClientSelection `json:"alternatives,omitempty" toml:"alternatives,omitempty"`
}

// Selections returns the reviewed choices in preference order.
func (recommendation ClientRecommendation) Selections() []ClientSelection {
	selections := make([]ClientSelection, 0, 1+len(recommendation.Alternatives))
	if recommendation.Primary.Route != "" {
		selections = append(selections, recommendation.Primary)
	}
	return append(selections, recommendation.Alternatives...)
}

// ClientBinding records one client's explicit selection, enabled intent, and
// owned native targets.
type ClientBinding struct {
	Route             string           `json:"route,omitempty"              toml:"route,omitempty"`
	Enabled           bool             `json:"enabled"                      toml:"enabled"`
	Protocol          EndpointProtocol `json:"protocol,omitempty"           toml:"protocol,omitempty"`
	ModelProvider     string           `json:"model_provider,omitempty"     toml:"model_provider,omitempty"`
	Authentication    Authentication   `json:"authentication,omitempty"     toml:"authentication,omitempty"`
	Executable        string           `json:"executable,omitempty"         toml:"executable,omitempty"`
	Targets           []string         `json:"targets,omitempty"            toml:"targets,omitempty"`
	CredentialCommand string           `json:"credential_command,omitempty" toml:"credential_command,omitempty"`
}

func (binding ClientBinding) selection() ClientSelection {
	return ClientSelection{
		Route:          binding.Route,
		Protocol:       binding.Protocol,
		ModelProvider:  binding.ModelProvider,
		Authentication: binding.Authentication,
	}
}

func (binding ClientBinding) withSelection(selection ClientSelection) ClientBinding {
	binding.Route = selection.Route
	binding.Protocol = selection.Protocol
	binding.ModelProvider = selection.ModelProvider
	binding.Authentication = selection.Authentication
	return binding
}

// CredentialExecutable selects explicit host policy or the native AIGW executable.
// It changes client projection only; the credential command itself stays native.
func (runtime Runtime) CredentialExecutable(native string) string {
	if runtime.CredentialCommand != "" {
		return runtime.CredentialCommand
	}
	return native
}

// UsesAIGWCredentialStore distinguishes AIGW-owned retrieval from an explicitly
// configured client helper without changing the Provider authentication protocol.
func (runtime Runtime) UsesAIGWCredentialStore() bool {
	return runtime.RequiresAccountToken() && runtime.CredentialCommand == ""
}
