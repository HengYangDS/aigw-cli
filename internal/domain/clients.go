package domain

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
	// SessionsKeepRoute marks a client whose existing sessions keep the route
	// they were created with, so a projection change reaches new sessions only.
	SessionsKeepRoute  bool
	QualifiedModes     []string
	QualifiedPlatforms []string
	// ProductName is the full product name where it differs from Label.
	ProductName string
	// TargetFile names the native configuration file that an explicit target
	// names; a client with one is enabled only with at least one target.
	TargetFile string
	// TargetsUsage explains repeated targets in the target option's help.
	TargetsUsage string
	// RetryBudgets marks a client that honors per-connection retry budgets.
	RetryBudgets bool
	// DefaultModelProvider is the model provider that a selection uses when it
	// names none. Only a client with one accepts an explicit model provider.
	DefaultModelProvider string
	// ClientNativeAuthentication marks a client that can authenticate with its
	// own native credentials instead of an Account Token.
	ClientNativeAuthentication bool
	// PublishedProfileProtocol is the endpoint protocol of this client's
	// profiles in the published version 3 configuration, which migration turns
	// into Routes. A client without one had no published profiles.
	PublishedProfileProtocol EndpointProtocol
	// ModelDiscoveryOptional marks a client that works without its endpoint's
	// model discovery, so a missing discovery endpoint still counts as reachable.
	ModelDiscoveryOptional bool
	// Doctor is how doctor words this client's checks.
	Doctor DoctorWording
}

// DoctorWording is how doctor words one client's checks; the zero value keeps
// the generic Healthy and Check failed.
type DoctorWording struct {
	// AdapterStates names the adapter's state and its missing executable or
	// configuration file.
	AdapterStates bool
	// RouteBinding reports an unresolvable route as an unresolvable binding.
	RouteBinding bool
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

// ResolveEndpoint selects an explicitly requested protocol or the sole available
// supported endpoint. Multiple choices require a Route-level selection.
func (s ClientSpec) ResolveEndpoint(account Account, requested EndpointProtocol) (string, EndpointProtocol, error) {
	return s.resolveEndpoint(account, nil, requested)
}

// resolveRouteEndpoint selects one endpoint admitted by the client, Account,
// and Route interface declaration.
func (s ClientSpec) resolveRouteEndpoint(account Account, route Route, requested EndpointProtocol) (string, EndpointProtocol, error) {
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
	RetryPolicy        *RetryPolicy `json:"retry_policy,omitempty" toml:"retry_policy,omitempty"`
	ForwardingEndpoint string       `json:"forwarding_endpoint,omitempty" toml:"-"`
	// ForwardingUpstreamIdentity and ForwardingProtocol record the upstream that
	// a forwarded binding reaches. SetForwardingEndpoint sets them with the
	// endpoint, and a binding forwards only while its Route still reaches them.
	ForwardingUpstreamIdentity string           `json:"-"                            toml:"-"`
	ForwardingProtocol         EndpointProtocol `json:"-"                            toml:"-"`
	Route                      string           `json:"route,omitempty"              toml:"route,omitempty"`
	Enabled                    bool             `json:"enabled"                      toml:"enabled"`
	Protocol                   EndpointProtocol `json:"protocol,omitempty"           toml:"protocol,omitempty"`
	ModelProvider              string           `json:"model_provider,omitempty"     toml:"model_provider,omitempty"`
	Authentication             Authentication   `json:"authentication,omitempty"     toml:"authentication,omitempty"`
	Executable                 string           `json:"executable,omitempty"         toml:"executable,omitempty"`
	Targets                    []string         `json:"targets,omitempty"            toml:"targets,omitempty"`
	CredentialCommand          string           `json:"credential_command,omitempty" toml:"credential_command,omitempty"`
}

func (binding ClientBinding) withoutForwarding() ClientBinding {
	binding.ForwardingEndpoint, binding.ForwardingUpstreamIdentity, binding.ForwardingProtocol = "", "", ""
	return binding
}

// SetForwardingEndpoint forwards one client to endpoint and records the
// upstream that its selected Route reaches now. An empty endpoint selects the
// direct connection and drops the recorded upstream with it.
func (c *Config) SetForwardingEndpoint(client, endpoint string) error {
	binding := c.Clients[client].withoutForwarding()
	if endpoint != "" {
		if err := validateEndpoint(endpoint); err != nil {
			return fmt.Errorf("client %q forwarding endpoint: %w", client, err)
		}
		// The recorded identity names the Account upstream, which forwarding
		// never replaces, so the current destination does not affect it.
		runtime, err := c.ResolveRuntime(client, "")
		if err != nil {
			return err
		}
		binding.ForwardingEndpoint = endpoint
		binding.ForwardingUpstreamIdentity = runtime.CredentialProjectionFingerprint(client)
		binding.ForwardingProtocol = runtime.Protocol
	}
	if c.Clients == nil {
		c.Clients = map[string]ClientBinding{}
	}
	c.Clients[client] = binding
	return nil
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

// SetSelectedRoute changes one client's Route and endpoint protocol while
// preserving enabled intent and native options. An empty protocol retains
// compatible selection or infers the Route's only admitted protocol.
func (c *Config) SetSelectedRoute(client, routeID string, protocol EndpointProtocol) {
	if c.Clients == nil {
		c.Clients = map[string]ClientBinding{}
	}
	binding := c.Clients[client]
	previous, previousErr := c.ResolveRuntime(client, "")
	if binding.Route == "" {
		binding = binding.withSelection(c.recommendedSelectionFor(client, routeID))
	}
	binding.Route = routeID
	if protocol != "" {
		binding.Protocol = protocol
	} else if route, exists := c.Routes[routeID]; exists && !routeAdmitsProtocol(route, binding.Protocol) {
		binding.Protocol = ""
		if protocols := route.AdmittedProtocols(); len(protocols) == 1 {
			binding.Protocol = protocols[0]
		}
	}
	c.Clients[client] = binding
	current, currentErr := c.ResolveRuntime(client, "")
	if previousErr != nil || currentErr != nil || !previous.SameUpstream(current) {
		binding = binding.withoutForwarding()
	}
	c.Clients[client] = binding
}

// SetClientActivation changes one client's enabled intent and native location
// while preserving its selected Route and client-specific options.
func (c *Config) SetClientActivation(client string, enabled bool, executable string, targets []string) {
	if c.Clients == nil {
		c.Clients = map[string]ClientBinding{}
	}
	binding := c.Clients[client]
	binding.Enabled = enabled
	binding.Executable = executable
	binding.Targets = append([]string(nil), targets...)
	c.Clients[client] = binding
}
