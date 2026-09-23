// Package configuration owns AIGW's complete configuration domain: its
// schema, validation, persistence, and token-free interchange format.
package configuration

import (
	"crypto/sha256"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"sort"
)

// Authentication identifies which boundary owns credentials for a Route.
type Authentication string

// Capability identifies one behavior qualified for an exact Route interface.
type Capability string

const (
	// ConfigVersion is the only configuration schema version accepted by this build.
	ConfigVersion = 6
	// ClientClaude identifies the admitted Claude Code client.
	ClientClaude = "claude"
	// ClientClaudeDesktop identifies Claude Desktop's independent third-party inference surface.
	ClientClaudeDesktop = "claude-desktop"
	// ClientCodex identifies the admitted Codex client family.
	ClientCodex = "codex"
	// ClientHermes identifies the independently configured Hermes Agent.
	ClientHermes = "hermes"
	// ModelProviderAIGW is the stable provider identifier written into AIGW-owned Codex projections.
	ModelProviderAIGW = "aigw"

	// AuthenticationAccountToken selects an AIGW-managed account token.
	AuthenticationAccountToken Authentication = "account-token"
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
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
var modelProviderPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

// Config is the typed source of truth for Accounts, Models, Routes, recommendations,
// and client bindings.
type Config struct {
	Version         int                             `json:"version"                   toml:"version"`
	Accounts        map[string]Account              `json:"accounts,omitempty"        toml:"accounts,omitempty"`
	Models          map[string]Model                `json:"models"                    toml:"models"`
	Routes          map[string]Route                `json:"routes"                    toml:"routes"`
	Recommendations map[string]ClientRecommendation `json:"recommendations,omitempty" toml:"recommendations,omitempty"`
	Clients         map[string]ClientBinding        `json:"clients,omitempty"         toml:"clients,omitempty"`
}

// Account defines one provider capability and its protocol endpoints without containing credentials.
type Account struct {
	ID           string        `json:"id,omitempty"            toml:"-"`
	Label        string        `json:"label"                   toml:"label"`
	Endpoints    Endpoints     `json:"endpoints"               toml:"endpoints"`
	AccountProbe *AccountProbe `json:"account_probe,omitempty" toml:"account_probe,omitempty"`
}

// Model identifies one canonical upstream product independently of an Account,
// protocol, provider channel, recommendation, or client selection.
type Model struct {
	ID    string `json:"id,omitempty" toml:"-"`
	Label string `json:"label"        toml:"label"`
}

// Route identifies how one Account exposes one canonical Model independently
// of client selection and native configuration.
type Route struct {
	ID            string                            `json:"id,omitempty"             toml:"-"`
	Label         string                            `json:"label"                    toml:"label"`
	Purpose       string                            `json:"purpose,omitempty"        toml:"purpose,omitempty"`
	Account       string                            `json:"account"                  toml:"account"`
	Model         string                            `json:"model"                    toml:"model"`
	UpstreamModel string                            `json:"upstream_model,omitempty" toml:"upstream_model,omitempty"`
	Interfaces    map[EndpointProtocol][]Capability `json:"interfaces,omitempty"      toml:"interfaces,omitempty"`
}

// AdmittedProtocols returns the Route's wire interfaces in stable order.
func (route Route) AdmittedProtocols() []EndpointProtocol {
	protocols := make([]EndpointProtocol, 0, len(route.Interfaces))
	for protocol := range route.Interfaces {
		protocols = append(protocols, protocol)
	}
	slices.Sort(protocols)
	return protocols
}

// Runtime is the resolved, immutable input used to project or invoke one client profile.
type Runtime struct {
	RouteID           string           `json:"profile_id"`
	RouteLabel        string           `json:"profile_label"`
	AccountID         string           `json:"account_id"`
	AccountLabel      string           `json:"account_label"`
	Client            string           `json:"client"`
	Endpoint          string           `json:"endpoint"`
	Protocol          EndpointProtocol `json:"protocol"`
	Model             string           `json:"model,omitempty"`
	ModelProvider     string           `json:"model_provider"`
	Authentication    Authentication   `json:"authentication"`
	CredentialCommand string           `json:"-"`
}

// RequiresAccountToken reports whether the selected Profile uses Account Token
// authentication on the wire. The zero value preserves the ordinary Account
// Token behavior for Profiles created before authentication was explicit.
func (runtime Runtime) RequiresAccountToken() bool {
	return runtime.Authentication == "" || runtime.Authentication == AuthenticationAccountToken
}

// CredentialProjectionFingerprint hashes the client, Account and endpoint identities.
// It detects stale credential projections; it is not a secret or caller authorization.
func (runtime Runtime) CredentialProjectionFingerprint(client string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(client+"\x00"+runtime.AccountID+"\x00"+runtime.Endpoint)))
}

// AccountProbe declares an optional provider-owned diagnostic API independently from inference traffic.
type AccountProbe struct {
	Kind    string `json:"kind"     toml:"kind"`
	BaseURL string `json:"base_url" toml:"base_url"`
}

// Endpoints declares the protocol-specific upstream URLs offered by an account.
type Endpoints struct {
	OpenAIChatCompletions string `json:"openai_chat_completions,omitempty" toml:"openai_chat_completions,omitempty"`
	OpenAIResponses       string `json:"openai_responses,omitempty" toml:"openai_responses,omitempty"`
	Anthropic             string `json:"anthropic,omitempty"        toml:"anthropic,omitempty"`
}

// NewConfig returns an empty configuration with all collection invariants initialized.
func NewConfig() Config {
	return Config{
		Version:         ConfigVersion,
		Accounts:        map[string]Account{},
		Models:          map[string]Model{},
		Routes:          map[string]Route{},
		Recommendations: map[string]ClientRecommendation{},
		Clients:         map[string]ClientBinding{},
	}
}

// Clone returns an independent configuration value. Config is the semantic
// owner of this copy operation so mutation workflows do not reproduce its
// nested map and slice shape.
func (c *Config) Clone() Config {
	out := NewConfig()
	out.Version = c.Version
	for name, account := range c.Accounts {
		if account.AccountProbe != nil {
			probe := *account.AccountProbe
			account.AccountProbe = &probe
		}
		out.Accounts[name] = account
	}
	maps.Copy(out.Models, c.Models)
	for name, route := range c.Routes {
		route.Interfaces = cloneInterfaces(route.Interfaces)
		out.Routes[name] = route
	}
	for client, recommendation := range c.Recommendations {
		recommendation.Alternatives = slices.Clone(recommendation.Alternatives)
		out.Recommendations[client] = recommendation
	}
	for name, adapter := range c.Clients {
		adapter.Targets = append([]string(nil), adapter.Targets...)
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

// RouteIDs returns the stable lexical order used by every CLI projection.
func (c *Config) RouteIDs() []string {
	ids := make([]string, 0, len(c.Routes))
	for id := range c.Routes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// ClientForRoute returns the sole admitted client compatible with a Route.
// Callers must ask for an explicit client when more than one is compatible.
func (c *Config) ClientForRoute(name string) (string, error) {
	compatible, err := c.CompatibleClientIDs(name)
	if err != nil {
		return "", err
	}
	if len(compatible) != 1 {
		return "", fmt.Errorf("route %q is compatible with %d clients; select one explicitly", name, len(compatible))
	}
	return compatible[0], nil
}

// CompatibleClientIDs returns admitted clients that can resolve one Route
// without guessing between several compatible protocols.
func (c *Config) CompatibleClientIDs(name string) ([]string, error) {
	route, ok := c.Routes[name]
	if !ok {
		return nil, fmt.Errorf("unknown route %q", name)
	}
	account, ok := c.Accounts[route.Account]
	if !ok {
		return nil, fmt.Errorf("route %q references unknown account %q", name, route.Account)
	}
	account.ID = route.Account
	compatible := make([]string, 0, len(admittedClientSpecs))
	for _, client := range AdmittedClientIDs() {
		if len(mustClientSpec(client).CompatibleRouteProtocols(account, route)) > 0 {
			compatible = append(compatible, client)
		}
	}
	return compatible, nil
}

// SelectedRoute returns the Route explicitly selected by one client.
func (c *Config) SelectedRoute(client string) string { return c.Clients[client].Route }

// SetSelectedRoute changes one client's selected Route while preserving
// enabled intent and native options.
func (c *Config) SetSelectedRoute(client, routeID string) {
	if c.Clients == nil {
		c.Clients = map[string]ClientBinding{}
	}
	binding := c.Clients[client]
	if binding.Route == "" {
		binding = binding.withSelection(c.recommendedSelection(client))
	}
	binding.Route = routeID
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

// RecommendedRoute returns the team recommendation for one client.
func (c *Config) RecommendedRoute(client string) string {
	return c.Recommendations[client].Primary.Route
}

// SetRecommendedRoute changes one client recommendation while preserving
// its protocol and authentication choices.
func (c *Config) SetRecommendedRoute(client, routeID string) {
	if c.Recommendations == nil {
		c.Recommendations = map[string]ClientRecommendation{}
	}
	recommendation := c.Recommendations[client]
	selection := recommendation.Primary
	selection.Route = routeID
	recommendation.Primary = selection
	c.Recommendations[client] = recommendation
}

// SelectedClientsForRoute returns the stable client set selecting one Route.
func (c *Config) SelectedClientsForRoute(routeID string) []string {
	clients := make([]string, 0, len(c.Clients))
	for _, client := range AdmittedClientIDs() {
		if c.Clients[client].Route == routeID {
			clients = append(clients, client)
		}
	}
	return clients
}

// ResolveAccount accepts either an Account ID or a Route ID and returns the
// referenced Account with its map identity populated.
func (c *Config) ResolveAccount(reference string) (string, Account, error) {
	if account, ok := c.Accounts[reference]; ok {
		account.ID = reference
		return reference, account, nil
	}
	route, ok := c.Routes[reference]
	if !ok {
		return "", Account{}, fmt.Errorf("unknown account or route %q", reference)
	}
	account, ok := c.Accounts[route.Account]
	if !ok {
		return "", Account{}, fmt.Errorf("route %q references unknown account %q", reference, route.Account)
	}
	account.ID = route.Account
	return route.Account, account, nil
}

// FirstRouteForClient returns the first stable Route that can resolve the
// requested client's model or endpoint without changing its binding.
func (c *Config) FirstRouteForClient(client string) string {
	for _, id := range c.RouteIDs() {
		if _, err := c.ResolveRuntime(client, id); err == nil {
			return id
		}
	}
	return ""
}

// EnabledClientIDs returns the stable client scope explicitly enabled by this configuration.
func (c *Config) EnabledClientIDs() []string {
	clients := make([]string, 0, len(c.Clients))
	for _, client := range AdmittedClientIDs() {
		if c.Clients[client].Enabled {
			clients = append(clients, client)
		}
	}
	return clients
}

// ClientUsesAccount reports whether the selected client binding uses an Account.
func (c *Config) ClientUsesAccount(client, accountID string) bool {
	runtime, err := c.ResolveRuntime(client, "")
	return err == nil && runtime.AccountID != "" && runtime.AccountID == accountID
}

// SelectedAccountIDs returns the stable set of Accounts selected by resolvable
// client bindings. Catalogue Accounts that no active binding selects are
// capabilities available for later connection, not current runtime
// dependencies.
func (c *Config) SelectedAccountIDs() []string {
	selected := map[string]bool{}
	for _, client := range AdmittedClientIDs() {
		runtime, err := c.ResolveRuntime(client, "")
		if err == nil && runtime.AccountID != "" {
			selected[runtime.AccountID] = true
		}
	}
	accountIDs := make([]string, 0, len(selected))
	for accountID := range selected {
		accountIDs = append(accountIDs, accountID)
	}
	sort.Strings(accountIDs)
	return accountIDs
}

// RequiredAccountTokenIDs returns the stable set of Accounts whose Tokens are
// required by enabled Client Bindings. Client-native Routes remain client
// credential concerns and therefore never create an AIGW Token requirement.
func (c *Config) RequiredAccountTokenIDs() []string {
	required := map[string]bool{}
	for _, client := range c.EnabledClientIDs() {
		runtime, err := c.ResolveRuntime(client, "")
		if err == nil && runtime.AccountID != "" && runtime.RequiresAccountToken() {
			required[runtime.AccountID] = true
		}
	}
	accountIDs := make([]string, 0, len(required))
	for accountID := range required {
		accountIDs = append(accountIDs, accountID)
	}
	sort.Strings(accountIDs)
	return accountIDs
}

// SelectRoutesForConnectedAccounts preserves the complete capability catalogue
// while filling unselected bindings whose authentication is currently usable. A
// client-native Route is usable without an AIGW Account Token; an account-token
// Route is usable only when its Account is connected. The first usable reviewed
// recommendation wins. Existing selections are never replaced.
func (c *Config) SelectRoutesForConnectedAccounts(accountIDs []string, clients ...string) (Config, error) {
	selected := c.Clone()
	connected := make(map[string]bool, len(accountIDs))
	for _, accountID := range accountIDs {
		if _, ok := selected.Accounts[accountID]; !ok {
			return Config{}, fmt.Errorf("unknown account %q", accountID)
		}
		connected[accountID] = true
	}
	if len(clients) == 0 {
		clients = slices.Sorted(maps.Keys(c.Recommendations))
	}
	for _, client := range clients {
		binding := selected.clientBinding(client)
		if binding.Route != "" {
			continue
		}
		selection := selected.routeForAvailableAuthentication(client, connected)
		if selection.Route == "" {
			continue
		}
		binding = binding.withSelection(selection)
		binding.Enabled = true
		selected.Clients[client] = binding
	}
	return selected, nil
}

func (c *Config) routeForAvailableAuthentication(client string, connected map[string]bool) ClientSelection {
	recommendation := c.Recommendations[client]
	for _, selection := range append([]ClientSelection{recommendation.Primary}, recommendation.Alternatives...) {
		if selection.Route == "" {
			continue
		}
		runtime, err := c.resolveSelection(client, selection)
		if err != nil {
			continue
		}
		if !runtime.RequiresAccountToken() || connected[runtime.AccountID] {
			return selection
		}
	}
	return ClientSelection{}
}

func (c *Config) recommendedSelection(client string) ClientSelection {
	return c.Recommendations[client].Primary
}

// ResolveRuntime resolves one Client Binding to its validated Account and Route runtime.
func (c *Config) ResolveRuntime(client, explicitRoute string) (Runtime, error) {
	binding := c.clientBinding(client)
	selection := binding.selection()
	if explicitRoute == "" && selection.Route == "" {
		return Runtime{}, &RuntimeBindingUnselectedError{Client: client}
	}
	if explicitRoute == "" || explicitRoute == selection.Route {
		return c.resolveSelection(client, selection)
	}
	selection = ClientSelection{Route: explicitRoute}
	route, ok := c.Routes[explicitRoute]
	if !ok {
		return Runtime{}, fmt.Errorf("unknown route %q", explicitRoute)
	}
	if binding.Route != "" {
		bound := c.Routes[binding.Route]
		if bound.Account == route.Account && routeAdmitsProtocol(route, binding.Protocol) {
			selection.Protocol = binding.Protocol
		}
	} else {
		recommended := c.recommendedSelection(client)
		if recommendedRoute, exists := c.Routes[recommended.Route]; exists && recommendedRoute.Account == route.Account && routeAdmitsProtocol(route, recommended.Protocol) {
			selection = recommended
		}
	}
	selection.Route = explicitRoute
	return c.resolveSelection(client, selection)
}

// ResolveRouteProtocol resolves one reviewed Route through an exact client
// protocol while retaining that client's authentication and credential policy.
func (c *Config) ResolveRouteProtocol(client, routeID string, protocol EndpointProtocol) (Runtime, error) {
	binding := c.clientBinding(client)
	return c.resolveSelection(client, ClientSelection{
		Route: routeID, Protocol: protocol,
		ModelProvider: binding.ModelProvider, Authentication: binding.Authentication,
	})
}

func routeAdmitsProtocol(route Route, protocol EndpointProtocol) bool {
	_, admitted := route.Interfaces[protocol]
	return admitted
}
func (c *Config) clientBinding(client string) ClientBinding {
	return c.Clients[client]
}

func (c *Config) resolveSelection(client string, selection ClientSelection) (Runtime, error) {
	name := selection.Route
	route, ok := c.Routes[name]
	if !ok {
		return Runtime{}, fmt.Errorf("unknown route %q", name)
	}
	account, ok := c.Accounts[route.Account]
	if !ok {
		return Runtime{}, &RuntimeRouteUnknownAccountError{RouteID: name, AccountID: route.Account}
	}
	account.ID = route.Account
	spec, admitted := ClientSpecFor(client)
	if !admitted {
		return Runtime{}, fmt.Errorf("unknown client %q", client)
	}
	endpoint, protocol, err := spec.ResolveRouteEndpoint(account, route, selection.Protocol)
	if err != nil {
		return Runtime{}, err
	}
	return Runtime{
		RouteID:           name,
		RouteLabel:        routeLabel(route, c.Models[route.Model]),
		AccountID:         account.ID,
		AccountLabel:      account.Label,
		Client:            client,
		Endpoint:          endpoint,
		Protocol:          protocol,
		Model:             upstreamModel(route),
		ModelProvider:     selectedModelProvider(client, selection),
		Authentication:    selectedAuthentication(selection),
		CredentialCommand: c.Clients[client].CredentialCommand,
	}, nil
}

func upstreamModel(route Route) string {
	if route.UpstreamModel != "" {
		return route.UpstreamModel
	}
	return route.Model
}

func routeLabel(route Route, model Model) string {
	if route.Label != "" {
		return route.Label
	}
	if model.Label != "" {
		return model.Label
	}
	return route.Model
}

func selectedModelProvider(client string, selection ClientSelection) string {
	if selection.ModelProvider != "" {
		return selection.ModelProvider
	}
	if client == ClientCodex {
		return ModelProviderAIGW
	}
	return ""
}

func selectedAuthentication(selection ClientSelection) Authentication {
	if selection.Authentication != "" {
		return selection.Authentication
	}
	return AuthenticationAccountToken
}

func mustClientSpec(client string) ClientSpec {
	spec, _ := ClientSpecFor(client)
	return spec
}

// EndpointFor returns the endpoint whose protocol is required by the requested admitted client.
func (account Account) EndpointFor(client string) (string, error) {
	spec, ok := ClientSpecFor(client)
	if !ok {
		return "", fmt.Errorf("unknown client %q", client)
	}
	endpoint, _, err := spec.ResolveEndpoint(account, "")
	return endpoint, err
}
