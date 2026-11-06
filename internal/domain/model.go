// Package domain owns AIGW's configuration model: its schema, the clients a
// configuration admits, validation, the endpoint requests that prove an
// Account's Token, the semantics of the token-free team manifest and the
// redaction rules. It imports only the standard library, names no client and
// performs no I/O.
package domain

import (
	"crypto/sha256"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
)

// Authentication identifies which boundary owns credentials for a Route.
type Authentication string

// Capability identifies one behavior qualified for an exact Route interface.
type Capability string

// ModelLifecycle identifies a reviewed Route's current admission disposition.
type ModelLifecycle string

const (
	// ConfigVersion is the only configuration schema version accepted by this build.
	ConfigVersion = 6
	// PublishedConfigVersion is the published 0.1.0 schema, which migration
	// accepts.
	PublishedConfigVersion = 3
	// LegacyConfigVersion is the immediate predecessor schema, which migration
	// accepts. Normal configuration reads accept ConfigVersion only.
	LegacyConfigVersion = 5

	// AuthenticationProviderToken selects an AIGW-managed account token.
	AuthenticationProviderToken Authentication = "account-token"
	// AuthenticationClientNative delegates authentication to the selected client's native credential chain.
	AuthenticationClientNative Authentication = "client-native"

	// CapabilityText identifies ordinary text input and output.
	CapabilityText Capability = "text"
	// CapabilityReasoning identifies native reasoning controls and results.
	CapabilityReasoning Capability = "reasoning"
	// CapabilityStreaming identifies incremental response delivery.
	CapabilityStreaming Capability = "streaming"
	// CapabilityTools identifies native tool invocation and results.
	CapabilityTools Capability = "tools"
	// CapabilityStructuredOutput identifies schema-constrained output.
	CapabilityStructuredOutput Capability = "structured_output"
	// CapabilityContinuation identifies response continuation semantics.
	CapabilityContinuation Capability = "continuation"
	// CapabilityCompaction identifies context compaction semantics.
	CapabilityCompaction Capability = "compaction"
	// CapabilityMultimodalInput identifies non-text model input.
	CapabilityMultimodalInput Capability = "multimodal_input"
	// CapabilityToolSearch identifies the Anthropic host tool-search shape:
	// the tool-search beta, deferred tool loading, and tool_reference results.
	CapabilityToolSearch Capability = "tool_search"

	// ModelAdmitted is the default lifecycle state for a configured Route.
	ModelAdmitted ModelLifecycle = "admitted"
	// ModelDeprecated retains existing bindings while excluding new recommendations.
	ModelDeprecated ModelLifecycle = "deprecated"
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
var modelProviderPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

// Config is the typed source of truth for providers, families, models,
// recommendations, and client bindings.
type Config struct {
	Version         int                             `json:"version"                   toml:"version"`
	Providers       map[string]Provider             `json:"accounts,omitempty"        toml:"accounts,omitempty"`
	Families        map[string]Family               `json:"models"                    toml:"models"`
	Models          map[string]Model                `json:"routes"                    toml:"routes"`
	Recommendations map[string]ClientRecommendation `json:"recommendations,omitempty" toml:"recommendations,omitempty"`
	Clients         map[string]ClientBinding        `json:"clients,omitempty"         toml:"clients,omitempty"`
	// admission is the client set that this configuration admits. It is not
	// serialized: NewConfig and the store that loads a configuration set it.
	admission Admission
}

// Provider defines one provider capability and its protocol endpoints without containing credentials.
type Provider struct {
	ID           string        `json:"id,omitempty"            toml:"-"`
	Label        string        `json:"label"                   toml:"label"`
	Order        int           `json:"order,omitempty"         toml:"order,omitempty"`
	Endpoints    Endpoints     `json:"endpoints"               toml:"endpoints"`
	BalanceProbe *BalanceProbe `json:"account_probe,omitempty" toml:"account_probe,omitempty"`
}

// Family identifies one canonical upstream product independently of a Provider,
// protocol, provider channel, recommendation, or client selection.
type Family struct {
	ID    string `json:"id,omitempty" toml:"-"`
	Label string `json:"label"        toml:"label"`
	Order int    `json:"order,omitempty" toml:"order,omitempty"`
}

// Model identifies how one Provider exposes one Family independently
// of client selection and native configuration.
type Model struct {
	ID            string                            `json:"id,omitempty"             toml:"-"`
	Label         string                            `json:"label"                    toml:"label,omitempty"`
	Purpose       string                            `json:"purpose,omitempty"        toml:"purpose,omitempty"`
	Provider      string                            `json:"account"                  toml:"account"`
	Family        string                            `json:"model"                    toml:"model"`
	UpstreamModel string                            `json:"upstream_model,omitempty" toml:"upstream_model,omitempty"`
	Protocols     map[EndpointProtocol][]Capability `json:"interfaces,omitempty"      toml:"interfaces,omitempty"`
	Lifecycle     ModelLifecycle                    `json:"lifecycle,omitempty"       toml:"lifecycle,omitempty"`
}

// UpstreamModelID returns the exact identifier sent to this Route's provider.
func (model Model) UpstreamModelID() string {
	if model.UpstreamModel != "" {
		return model.UpstreamModel
	}
	return model.Family
}

// LifecycleState returns the Route's explicit lifecycle or its admitted default.
func (model Model) LifecycleState() ModelLifecycle {
	if model.Lifecycle != "" {
		return model.Lifecycle
	}
	return ModelAdmitted
}

// Declares reports whether the Route qualified capability on protocol.
func (model Model) Declares(protocol EndpointProtocol, capability Capability) bool {
	return slices.Contains(model.Protocols[protocol], capability)
}

// AdmittedProtocols returns the Route's wire interfaces in stable order.
func (model Model) AdmittedProtocols() []EndpointProtocol {
	protocols := make([]EndpointProtocol, 0, len(model.Protocols))
	for protocol := range model.Protocols {
		protocols = append(protocols, protocol)
	}
	slices.Sort(protocols)
	return protocols
}

// Runtime is the resolved, immutable input used to project or invoke one client route.
type Runtime struct {
	RetryPolicy       *RetryPolicy     `json:"retry_policy,omitempty"`
	ModelID           string           `json:"route_id"`
	ModelLabel        string           `json:"route_label"`
	ProviderID        string           `json:"account_id"`
	ProviderLabel     string           `json:"account_label"`
	Client            string           `json:"client"`
	Endpoint          string           `json:"endpoint"`
	UpstreamEndpoint  string           `json:"upstream_endpoint"`
	Protocol          EndpointProtocol `json:"protocol"`
	UpstreamModel     string           `json:"model,omitempty"`
	FamilyID          string           `json:"canonical_model_id,omitempty"`
	ModelProvider     string           `json:"model_provider"`
	Authentication    Authentication   `json:"authentication"`
	CredentialCommand string           `json:"-"`
}

// RequiresProviderToken reports whether the selected Route uses Account Token
// authentication on the wire. The zero value preserves the ordinary Account
// Token behavior for Routes created before authentication was explicit.
func (runtime Runtime) RequiresProviderToken() bool {
	return runtime.Authentication == "" || runtime.Authentication == AuthenticationProviderToken
}

// CredentialProjectionFingerprint hashes the client, Account and upstream identities.
// It detects stale credential projections; it is not a secret or caller authorization.
func (runtime Runtime) CredentialProjectionFingerprint(client string) string {
	endpoint := runtime.UpstreamEndpoint
	if endpoint == "" {
		endpoint = runtime.Endpoint
	}
	return upstreamIdentity(client, runtime.ProviderID, endpoint)
}

func upstreamIdentity(client, accountID, endpoint string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(client+"\x00"+accountID+"\x00"+endpoint)))
}

// SameUpstream compares the Account, endpoint and protocol identity independently
// of model choice, serialized defaults and a client's forwarding destination.
func (runtime Runtime) SameUpstream(other Runtime) bool {
	return runtime.Client == other.Client && runtime.Protocol == other.Protocol &&
		runtime.CredentialProjectionFingerprint(runtime.Client) == other.CredentialProjectionFingerprint(other.Client)
}

// ConnectionFingerprint identifies one client's upstream, effective destination
// and native provider/authentication contract independently of model and retries.
func (runtime Runtime) ConnectionFingerprint() string {
	identity := strings.Join([]string{runtime.CredentialProjectionFingerprint(runtime.Client), runtime.Endpoint,
		string(runtime.Protocol), runtime.ModelProvider, string(runtime.Authentication)}, "\x00")
	return fmt.Sprintf("%x", sha256.Sum256([]byte(identity)))
}

// BalanceProbe declares an optional provider-owned diagnostic API independently from inference traffic.
type BalanceProbe struct {
	Kind    string `json:"kind"     toml:"kind"`
	BaseURL string `json:"base_url" toml:"base_url"`
}

// Endpoints declares the protocol-specific upstream URLs offered by an account.
type Endpoints struct {
	OpenAIChatCompletions string `json:"openai_chat_completions,omitempty" toml:"openai_chat_completions,omitempty"`
	OpenAIResponses       string `json:"openai_responses,omitempty" toml:"openai_responses,omitempty"`
	Anthropic             string `json:"anthropic,omitempty"        toml:"anthropic,omitempty"`
}

// NewConfig returns an empty configuration that admits the given clients, with
// all collection invariants initialized.
func NewConfig(admission Admission) Config {
	return Config{
		Version:         ConfigVersion,
		Providers:       map[string]Provider{},
		Families:        map[string]Family{},
		Models:          map[string]Model{},
		Recommendations: map[string]ClientRecommendation{},
		Clients:         map[string]ClientBinding{},
		admission:       admission,
	}
}

// Admission returns the client set that this configuration admits.
func (c *Config) Admission() Admission { return c.admission }

// SetAdmission makes admission the client set that this configuration admits.
// A store calls it on a configuration that it decodes, because a decoder cannot
// carry the admission into the value it fills.
func (c *Config) SetAdmission(admission Admission) { c.admission = admission }

// Clone returns an independent configuration value. Config is the semantic
// owner of this copy operation so mutation workflows do not reproduce its
// nested map and slice shape.
func (c *Config) Clone() Config {
	out := NewConfig(c.admission)
	out.Version = c.Version
	for name, account := range c.Providers {
		if account.BalanceProbe != nil {
			probe := *account.BalanceProbe
			account.BalanceProbe = &probe
		}
		out.Providers[name] = account
	}
	maps.Copy(out.Families, c.Families)
	for name, route := range c.Models {
		route.Protocols = cloneInterfaces(route.Protocols)
		out.Models[name] = route
	}
	for client, recommendation := range c.Recommendations {
		recommendation.Alternatives = slices.Clone(recommendation.Alternatives)
		out.Recommendations[client] = recommendation
	}
	for name, adapter := range c.Clients {
		adapter.Targets = append([]string(nil), adapter.Targets...)
		adapter.RetryPolicy = adapter.RetryPolicy.Clone()
		out.Clients[name] = adapter
	}
	return out
}

func cloneInterfaces(interfaces map[EndpointProtocol][]Capability) map[EndpointProtocol][]Capability {
	if interfaces == nil {
		return nil
	}
	cloned := make(map[EndpointProtocol][]Capability, len(interfaces))
	for protocol, capabilities := range interfaces {
		cloned[protocol] = slices.Clone(capabilities)
	}
	return cloned
}

// ModelIDs returns canonical lexical identity order, independent of presentation.
func (c *Config) ModelIDs() []string {
	return slices.Sorted(maps.Keys(c.Models))
}

// ProtocolClientIDs returns clients whose declared protocol overlaps one Route.
// Protocol overlap permits selection; it does not qualify native client behavior.
func (c *Config) ProtocolClientIDs(name string) ([]string, error) {
	route, ok := c.Models[name]
	if !ok {
		return nil, fmt.Errorf("unknown route %q", name)
	}
	account, ok := c.Providers[route.Provider]
	if !ok {
		return nil, fmt.Errorf("route %q references unknown account %q", name, route.Provider)
	}
	account.ID = route.Provider
	specs := c.admission.Specs()
	compatible := make([]string, 0, len(specs))
	for _, spec := range specs {
		if len(spec.CompatibleModelProtocols(account, route)) > 0 {
			compatible = append(compatible, spec.ID)
		}
	}
	return compatible, nil
}

// SelectedModel returns the Route explicitly selected by one client.
func (c *Config) SelectedModel(client string) string { return c.Clients[client].Model }

// ModelLabel returns an explicit display override or derives Provider · Family
// from the two labels already declared by configuration.
func (c *Config) ModelLabel(routeID string) string {
	route, ok := c.Models[routeID]
	if !ok {
		return routeID
	}
	return modelLabel(route, c.Providers[route.Provider], c.Families[route.Family])
}

// RecommendedModel returns the team recommendation for one client.
func (c *Config) RecommendedModel(client string) string {
	return c.Recommendations[client].Primary.Model
}

// SetRecommendedModel changes one client recommendation while preserving
// its protocol and authentication choices.
func (c *Config) SetRecommendedModel(client, routeID string) {
	if c.Recommendations == nil {
		c.Recommendations = map[string]ClientRecommendation{}
	}
	recommendation := c.Recommendations[client]
	selection := recommendation.Primary
	selection.Model = routeID
	recommendation.Primary = selection
	c.Recommendations[client] = recommendation
}

// SelectedClientsForModel returns the stable client set selecting one Route.
func (c *Config) SelectedClientsForModel(routeID string) []string {
	clients := make([]string, 0, len(c.Clients))
	for _, client := range c.admission.IDs() {
		if c.Clients[client].Model == routeID {
			clients = append(clients, client)
		}
	}
	return clients
}

// ResolveProvider accepts either an Account ID or a Route ID and returns the
// referenced Account with its map identity populated.
func (c *Config) ResolveProvider(reference string) (string, Provider, error) {
	if account, ok := c.Providers[reference]; ok {
		account.ID = reference
		return reference, account, nil
	}
	route, ok := c.Models[reference]
	if !ok {
		return "", Provider{}, fmt.Errorf("unknown account or route %q", reference)
	}
	account, ok := c.Providers[route.Provider]
	if !ok {
		return "", Provider{}, fmt.Errorf("route %q references unknown account %q", reference, route.Provider)
	}
	account.ID = route.Provider
	return route.Provider, account, nil
}

// EnabledClientIDs returns the stable client scope explicitly enabled by this configuration.
func (c *Config) EnabledClientIDs() []string {
	clients := make([]string, 0, len(c.Clients))
	for _, client := range c.admission.IDs() {
		if c.Clients[client].Enabled {
			clients = append(clients, client)
		}
	}
	return clients
}

// SelectedProviderIDs returns the stable set of Accounts selected by resolvable
// client bindings. Catalog Accounts that no active binding selects are
// capabilities available for later connection, not current runtime
// dependencies.
func (c *Config) SelectedProviderIDs() []string {
	selected := map[string]bool{}
	for _, client := range c.admission.IDs() {
		runtime, err := c.ResolveRuntime(client, "")
		if err == nil && runtime.ProviderID != "" {
			selected[runtime.ProviderID] = true
		}
	}
	return slices.Sorted(maps.Keys(selected))
}

// RequiredProviderTokenIDs returns the stable set of Accounts whose Tokens are
// retrieved by AIGW for enabled Client Bindings. Client-native authentication
// and external readers retain their credential ownership.
func (c *Config) RequiredProviderTokenIDs() []string {
	required := map[string]bool{}
	for _, client := range c.EnabledClientIDs() {
		runtime, err := c.ResolveRuntime(client, "")
		if err == nil && runtime.ProviderID != "" && runtime.UsesAIGWCredentialStore() {
			required[runtime.ProviderID] = true
		}
	}
	return slices.Sorted(maps.Keys(required))
}

// SelectModelsForConnectedProviders preserves the complete capability catalog
// while filling unselected bindings whose authentication is currently usable. A
// client-native Route is usable without an AIGW Account Token; an account-token
// Route is usable only when its Account is connected. The first usable reviewed
// recommendation wins. Existing selections are never replaced.
func (c *Config) SelectModelsForConnectedProviders(accountIDs []string, clients ...string) (Config, error) {
	selected := c.Clone()
	connected := make(map[string]bool, len(accountIDs))
	for _, accountID := range accountIDs {
		if _, ok := selected.Providers[accountID]; !ok {
			return Config{}, fmt.Errorf("unknown account %q", accountID)
		}
		connected[accountID] = true
	}
	if len(clients) == 0 {
		clients = slices.Sorted(maps.Keys(c.Recommendations))
	}
	for _, client := range clients {
		binding := selected.clientBinding(client)
		if binding.Model != "" {
			continue
		}
		selection := selected.modelForAvailableAuthentication(client, connected)
		if selection.Model == "" {
			continue
		}
		binding = binding.withSelection(selection)
		binding.Enabled = true
		selected.Clients[client] = binding
	}
	return selected, nil
}

func (c *Config) modelForAvailableAuthentication(client string, connected map[string]bool) ClientSelection {
	recommendation := c.Recommendations[client]
	for _, selection := range append([]ClientSelection{recommendation.Primary}, recommendation.Alternatives...) {
		if selection.Model == "" {
			continue
		}
		runtime, err := c.resolveSelection(client, selection)
		if err != nil {
			continue
		}
		if !runtime.RequiresProviderToken() || connected[runtime.ProviderID] {
			return selection
		}
	}
	return ClientSelection{}
}

func (c *Config) recommendedSelection(client string) ClientSelection {
	return c.Recommendations[client].Primary
}

// recommendedSelectionFor applies the client's recommended native options to
// routeID when it is the recommended Route, or when the recommended Route
// shares its Account and routeID admits the recommended protocol. Another
// Account's Route keeps default options.
func (c *Config) recommendedSelectionFor(client, routeID string) ClientSelection {
	recommended := c.recommendedSelection(client)
	if recommended.Model == routeID {
		return recommended
	}
	route, exists := c.Models[routeID]
	recommendedRoute, recommendedExists := c.Models[recommended.Model]
	if !exists || !recommendedExists || recommendedRoute.Provider != route.Provider || !modelAdmitsProtocol(route, recommended.Protocol) {
		return ClientSelection{Model: routeID}
	}
	recommended.Model = routeID
	return recommended
}

// ResolveRuntime resolves one Client Binding to its validated Account and Route runtime.
func (c *Config) ResolveRuntime(client, explicitRoute string) (Runtime, error) {
	binding := c.clientBinding(client)
	selection := binding.selection()
	if explicitRoute == "" && selection.Model == "" {
		return Runtime{}, &RuntimeBindingUnselectedError{Client: client}
	}
	if explicitRoute == "" || explicitRoute == selection.Model {
		return c.resolveSelection(client, selection)
	}
	selection = ClientSelection{Model: explicitRoute}
	route, ok := c.Models[explicitRoute]
	if !ok {
		return Runtime{}, fmt.Errorf("unknown route %q", explicitRoute)
	}
	if binding.Model != "" {
		bound := c.Models[binding.Model]
		if bound.Provider == route.Provider && modelAdmitsProtocol(route, binding.Protocol) {
			selection.Protocol = binding.Protocol
		}
	} else {
		selection = c.recommendedSelectionFor(client, explicitRoute)
	}
	return c.resolveSelection(client, selection)
}

// ResolveModelProtocol resolves one reviewed Route through an exact client
// protocol while retaining that client's authentication and credential policy.
func (c *Config) ResolveModelProtocol(client, routeID string, protocol EndpointProtocol) (Runtime, error) {
	binding := c.clientBinding(client)
	return c.resolveSelection(client, ClientSelection{
		Model: routeID, Protocol: protocol,
		ModelProvider: binding.ModelProvider, Authentication: binding.Authentication,
	})
}

func modelAdmitsProtocol(route Model, protocol EndpointProtocol) bool {
	_, admitted := route.Protocols[protocol]
	return admitted
}
func (c *Config) clientBinding(client string) ClientBinding {
	return c.Clients[client]
}

func (c *Config) resolveSelection(client string, selection ClientSelection) (Runtime, error) {
	name := selection.Model
	route, ok := c.Models[name]
	if !ok {
		return Runtime{}, fmt.Errorf("unknown route %q", name)
	}
	account, ok := c.Providers[route.Provider]
	if !ok {
		return Runtime{}, &RuntimeModelUnknownProviderError{ModelID: name, ProviderID: route.Provider}
	}
	account.ID = route.Provider
	spec, admitted := c.admission.Spec(client)
	if !admitted {
		return Runtime{}, fmt.Errorf("unknown client %q", client)
	}
	endpoint, protocol, err := spec.resolveModelEndpoint(account, route, selection.Protocol)
	if err != nil {
		return Runtime{}, err
	}
	upstream := endpoint
	if forwarding := c.forwardingFor(client, account.ID, upstream, protocol); forwarding != "" {
		if err := validateEndpoint(forwarding); err != nil {
			return Runtime{}, fmt.Errorf("client %q forwarding endpoint: %w", client, err)
		}
		endpoint = strings.TrimRight(forwarding, "/")
	}
	return Runtime{
		RetryPolicy:       c.retryPolicyFor(client, selection, upstream, protocol),
		ModelID:           name,
		ModelLabel:        modelLabel(route, account, c.Families[route.Family]),
		ProviderID:        account.ID,
		ProviderLabel:     account.Label,
		Client:            client,
		Endpoint:          endpoint,
		UpstreamEndpoint:  upstream,
		Protocol:          protocol,
		UpstreamModel:     route.UpstreamModelID(),
		FamilyID:          route.Family,
		ModelProvider:     selectedModelProvider(spec, selection),
		Authentication:    selectedAuthentication(selection),
		CredentialCommand: c.Clients[client].CredentialCommand,
	}, nil
}

// forwardingFor returns a client's forwarding destination for a selection that
// reaches the upstream the binding recorded, and nothing for any other.
func (c *Config) forwardingFor(client, accountID, upstream string, protocol EndpointProtocol) string {
	binding := c.Clients[client]
	if binding.ForwardingEndpoint == "" || protocol != binding.ForwardingProtocol ||
		upstreamIdentity(client, accountID, upstream) != binding.ForwardingUpstreamIdentity {
		return ""
	}
	return binding.ForwardingEndpoint
}

func modelLabel(model Model, provider Provider, family Family) string {
	if model.Label != "" {
		return model.Label
	}
	providerLabel := provider.Label
	if providerLabel == "" {
		providerLabel = model.Provider
	}
	familyLabel := family.Label
	if familyLabel == "" {
		familyLabel = model.Family
	}
	return providerLabel + " · " + familyLabel
}

func selectedModelProvider(spec ClientSpec, selection ClientSelection) string {
	if selection.ModelProvider != "" {
		return selection.ModelProvider
	}
	return spec.DefaultModelProvider
}

func selectedAuthentication(selection ClientSelection) Authentication {
	if selection.Authentication != "" {
		return selection.Authentication
	}
	return AuthenticationProviderToken
}
