// Package domain owns AIGW's configuration model: its schema, the clients a
// configuration admits, validation, the endpoint requests that prove a
// provider's Token, the semantics of the token-free team file and the
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

// Authentication identifies which boundary owns the credentials of a
// client's selection.
type Authentication string

// Capability identifies one behavior qualified for a model on one protocol.
type Capability string

const (
	// ConfigVersion is the only schema version of aigw.toml and of a team
	// file that this build reads and writes.
	ConfigVersion = 8

	// AuthenticationProviderToken sends the provider's Token, which AIGW
	// stores.
	AuthenticationProviderToken Authentication = "provider-token"
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
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
var modelProviderPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

// Config is the typed source of truth for providers and their models,
// families, recommendations and clients. Its maps find an entry by its
// identity; the display order of providers, families and each provider's
// models is the order of the file that it was read from, followed by the
// entries that were added since (see ordering.go).
type Config struct {
	Version         int
	Providers       map[string]Provider
	Families        map[string]Family
	Models          map[ModelRef]Model
	Recommendations map[string]ClientRecommendation
	Clients         map[string]ClientBinding
	// admission is the client set that this configuration admits. NewConfig
	// and the store that loads a configuration set it.
	admission Admission
	// order is the display order that the file gave.
	order displayOrder
}

// Provider is a service that AIGW holds a Token for: a label, one endpoint per
// protocol and an optional balance probe. It holds no credential.
type Provider struct {
	ID           string
	Label        string
	Endpoints    Endpoints
	BalanceProbe *BalanceProbe
}

// Family is the canonical model that several providers' models are, so that
// they can be compared, independently of a provider, protocol or client.
type Family struct {
	ID    string
	Label string
}

// Model is one provider's model, named `provider/model`: ID is the provider's
// own model ID, byte for byte. Family is empty for a model that is its own
// family. Protocols maps each protocol on which AIGW may use the model to the
// capabilities qualified there.
type Model struct {
	Provider   string
	ID         string
	Family     string
	Label      string
	Protocols  map[EndpointProtocol][]Capability
	Deprecated bool
}

// FamilyID returns the model's family: its declared family, or the model ID
// for a model that is its own family.
func (model Model) FamilyID() string {
	if model.Family != "" {
		return model.Family
	}
	return model.ID
}

// Declares reports whether the model qualified capability on protocol.
func (model Model) Declares(protocol EndpointProtocol, capability Capability) bool {
	return slices.Contains(model.Protocols[protocol], capability)
}

// AdmittedProtocols returns the model's protocols in stable order.
func (model Model) AdmittedProtocols() []EndpointProtocol {
	return slices.Sorted(maps.Keys(model.Protocols))
}

// Runtime is the resolved, immutable input used to configure or invoke one
// client's selected model.
type Runtime struct {
	RetryPolicy       *RetryPolicy     `json:"retry_policy,omitempty"`
	Model             ModelRef         `json:"model"`
	ModelLabel        string           `json:"model_label"`
	ProviderID        string           `json:"provider"`
	ProviderLabel     string           `json:"provider_label"`
	Client            string           `json:"client"`
	Endpoint          string           `json:"endpoint"`
	UpstreamEndpoint  string           `json:"upstream_endpoint"`
	Protocol          EndpointProtocol `json:"protocol"`
	UpstreamModel     string           `json:"upstream_model,omitempty"`
	FamilyID          string           `json:"family,omitempty"`
	ModelProvider     string           `json:"model_provider"`
	Authentication    Authentication   `json:"authentication"`
	CredentialCommand string           `json:"-"`
}

// RequiresProviderToken reports whether the selection sends the provider's
// Token. A selection that names no authentication sends it.
func (runtime Runtime) RequiresProviderToken() bool {
	return runtime.Authentication == "" || runtime.Authentication == AuthenticationProviderToken
}

// CredentialProjectionFingerprint hashes the client, provider and upstream
// endpoint. It detects a stale credential command in a client's files; it is
// not a secret or caller authorization.
func (runtime Runtime) CredentialProjectionFingerprint(client string) string {
	endpoint := runtime.UpstreamEndpoint
	if endpoint == "" {
		endpoint = runtime.Endpoint
	}
	return upstreamIdentity(client, runtime.ProviderID, endpoint)
}

func upstreamIdentity(client, providerID, endpoint string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(client+"\x00"+providerID+"\x00"+endpoint)))
}

// SameUpstream compares the provider, endpoint and protocol identity
// independently of model choice, serialized defaults and a client's
// forwarding destination.
func (runtime Runtime) SameUpstream(other Runtime) bool {
	return runtime.Client == other.Client && runtime.Protocol == other.Protocol &&
		runtime.CredentialProjectionFingerprint(runtime.Client) == other.CredentialProjectionFingerprint(other.Client)
}

// BalanceProbe declares an optional provider-owned diagnostic API independently from inference traffic.
type BalanceProbe struct {
	Kind    string
	BaseURL string
}

// Endpoints declares a provider's endpoint for each protocol it serves.
type Endpoints struct {
	Anthropic string
	Responses string
	Chat      string
}

// NewConfig returns an empty configuration that admits the given clients, with
// all collection invariants initialized.
func NewConfig(admission Admission) Config {
	return Config{
		Version:         ConfigVersion,
		Providers:       map[string]Provider{},
		Families:        map[string]Family{},
		Models:          map[ModelRef]Model{},
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
	for name, provider := range c.Providers {
		if provider.BalanceProbe != nil {
			probe := *provider.BalanceProbe
			provider.BalanceProbe = &probe
		}
		out.Providers[name] = provider
	}
	maps.Copy(out.Families, c.Families)
	for ref, model := range c.Models {
		model.Protocols = cloneProtocols(model.Protocols)
		out.Models[ref] = model
	}
	for client, recommendation := range c.Recommendations {
		recommendation.Alternatives = slices.Clone(recommendation.Alternatives)
		out.Recommendations[client] = recommendation
	}
	for name, binding := range c.Clients {
		binding.Targets = append([]string(nil), binding.Targets...)
		binding.RetryPolicy = binding.RetryPolicy.Clone()
		out.Clients[name] = binding
	}
	out.order = c.order.clone()
	return out
}

func cloneProtocols(protocols map[EndpointProtocol][]Capability) map[EndpointProtocol][]Capability {
	if protocols == nil {
		return nil
	}
	cloned := make(map[EndpointProtocol][]Capability, len(protocols))
	for protocol, capabilities := range protocols {
		cloned[protocol] = slices.Clone(capabilities)
	}
	return cloned
}

// ProtocolClientIDs returns the clients that speak a protocol of the model
// that its provider serves. Protocol overlap permits selection; it does not
// qualify native client behavior.
func (c *Config) ProtocolClientIDs(ref ModelRef) ([]string, error) {
	model, ok := c.Models[ref]
	if !ok {
		return nil, fmt.Errorf("unknown model %s", ref)
	}
	provider, ok := c.Providers[model.Provider]
	if !ok {
		return nil, &RuntimeModelUnknownProviderError{Model: ref, ProviderID: model.Provider}
	}
	provider.ID = model.Provider
	specs := c.admission.Specs()
	compatible := make([]string, 0, len(specs))
	for _, spec := range specs {
		if len(spec.CompatibleModelProtocols(provider, model)) > 0 {
			compatible = append(compatible, spec.ID)
		}
	}
	return compatible, nil
}

// SelectedModel returns the model that one client selects, or the zero
// reference when it selects none.
func (c *Config) SelectedModel(client string) ModelRef { return c.Clients[client].Model }

// ModelLabel returns a model's explicit label, or derives one from its
// provider's and family's labels.
func (c *Config) ModelLabel(ref ModelRef) string {
	model, ok := c.Models[ref]
	if !ok {
		return ref.String()
	}
	return modelLabel(model, c.Providers[model.Provider], c.Families[model.Family])
}

// RecommendedModel returns the team recommendation for one client.
func (c *Config) RecommendedModel(client string) ModelRef {
	return c.Recommendations[client].Primary.Model
}

// SelectedClientsForModel returns the clients that select one model, in
// admission order.
func (c *Config) SelectedClientsForModel(ref ModelRef) []string {
	clients := make([]string, 0, len(c.Clients))
	for _, client := range c.admission.IDs() {
		if selected := c.Clients[client].Model; !selected.IsZero() && selected == ref {
			clients = append(clients, client)
		}
	}
	return clients
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

// SelectedProviderIDs returns the providers that the clients' resolvable
// selections use, in sorted order. Providers that no selection uses are
// available for later use, not current runtime dependencies.
func (c *Config) SelectedProviderIDs() []string {
	selected := map[string]bool{}
	for _, client := range c.admission.IDs() {
		runtime, err := c.ResolveRuntime(client, ModelRef{})
		if err == nil && runtime.ProviderID != "" {
			selected[runtime.ProviderID] = true
		}
	}
	return slices.Sorted(maps.Keys(selected))
}

// RequiredProviderTokenIDs returns the providers whose Tokens AIGW reads for
// the enabled clients, in sorted order. Client-native authentication and
// external readers keep their credential ownership.
func (c *Config) RequiredProviderTokenIDs() []string {
	required := map[string]bool{}
	for _, client := range c.EnabledClientIDs() {
		runtime, err := c.ResolveRuntime(client, ModelRef{})
		if err == nil && runtime.ProviderID != "" && runtime.UsesAIGWCredentialStore() {
			required[runtime.ProviderID] = true
		}
	}
	return slices.Sorted(maps.Keys(required))
}

// SelectModelsForConnectedProviders keeps every model while filling the
// selections of clients without one whose authentication is usable now. A
// client-native selection is usable without a Token; a provider-token
// selection only when its provider is connected. The first usable
// recommendation wins, and an existing selection is never replaced.
func (c *Config) SelectModelsForConnectedProviders(providerIDs []string, clients ...string) (Config, error) {
	selected := c.Clone()
	connected := make(map[string]bool, len(providerIDs))
	for _, providerID := range providerIDs {
		if _, ok := selected.Providers[providerID]; !ok {
			return Config{}, fmt.Errorf("unknown provider %q", providerID)
		}
		connected[providerID] = true
	}
	if len(clients) == 0 {
		clients = slices.Sorted(maps.Keys(c.Recommendations))
	}
	for _, client := range clients {
		binding := selected.clientBinding(client)
		if !binding.Model.IsZero() {
			continue
		}
		selection := selected.modelForAvailableAuthentication(client, connected)
		if selection.Model.IsZero() {
			continue
		}
		binding = binding.withSelection(selection)
		binding.Enabled = true
		selected.Clients[client] = binding
	}
	return selected, nil
}

func (c *Config) modelForAvailableAuthentication(client string, connected map[string]bool) ClientSelection {
	for _, selection := range c.Recommendations[client].Selections() {
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

// recommendedSelectionFor applies the client's recommended native options to
// ref when it is the recommended model, or when the recommended model shares
// its provider and ref declares the recommended protocol. Another provider's
// model keeps default options.
func (c *Config) recommendedSelectionFor(client string, ref ModelRef) ClientSelection {
	recommended := c.Recommendations[client].Primary
	if recommended.Model == ref {
		return recommended
	}
	model, exists := c.Models[ref]
	recommendedModel, recommendedExists := c.Models[recommended.Model]
	if !exists || !recommendedExists || recommendedModel.Provider != model.Provider || !modelAdmitsProtocol(model, recommended.Protocol) {
		return ClientSelection{Model: ref}
	}
	recommended.Model = ref
	return recommended
}

// ResolveRuntime resolves one client's selection, or explicit, a model that
// it tries without selecting it, to its provider and endpoint.
func (c *Config) ResolveRuntime(client string, explicit ModelRef) (Runtime, error) {
	binding := c.clientBinding(client)
	selection := binding.selection()
	if explicit.IsZero() && selection.Model.IsZero() {
		return Runtime{}, &RuntimeBindingUnselectedError{Client: client}
	}
	if explicit.IsZero() || explicit == selection.Model {
		return c.resolveSelection(client, selection)
	}
	selection = ClientSelection{Model: explicit}
	model, ok := c.Models[explicit]
	if !ok {
		return Runtime{}, fmt.Errorf("unknown model %s", explicit)
	}
	if !binding.Model.IsZero() {
		bound := c.Models[binding.Model]
		if bound.Provider == model.Provider && modelAdmitsProtocol(model, binding.Protocol) {
			selection.Protocol = binding.Protocol
		}
	} else {
		selection = c.recommendedSelectionFor(client, explicit)
	}
	return c.resolveSelection(client, selection)
}

// ResolveModelProtocol resolves one model through an exact client protocol
// while retaining that client's authentication and credential policy.
func (c *Config) ResolveModelProtocol(client string, ref ModelRef, protocol EndpointProtocol) (Runtime, error) {
	binding := c.clientBinding(client)
	return c.resolveSelection(client, ClientSelection{
		Model: ref, Protocol: protocol,
		ModelProvider: binding.ModelProvider, Authentication: binding.Authentication,
	})
}

// ResolveSelection resolves one recommendation choice of client, as
// validation does, to the protocol that it reaches.
func (c *Config) ResolveSelection(client string, selection ClientSelection) (EndpointProtocol, error) {
	runtime, err := c.resolveSelection(client, selection)
	return runtime.Protocol, err
}

func modelAdmitsProtocol(model Model, protocol EndpointProtocol) bool {
	_, admitted := model.Protocols[protocol]
	return admitted
}

func (c *Config) clientBinding(client string) ClientBinding {
	return c.Clients[client]
}

func (c *Config) resolveSelection(client string, selection ClientSelection) (Runtime, error) {
	ref := selection.Model
	model, ok := c.Models[ref]
	if !ok {
		return Runtime{}, fmt.Errorf("unknown model %s", ref)
	}
	provider, ok := c.Providers[model.Provider]
	if !ok {
		return Runtime{}, &RuntimeModelUnknownProviderError{Model: ref, ProviderID: model.Provider}
	}
	provider.ID = model.Provider
	spec, admitted := c.admission.Spec(client)
	if !admitted {
		return Runtime{}, fmt.Errorf("unknown client %q", client)
	}
	endpoint, protocol, err := spec.resolveModelEndpoint(provider, model, selection.Protocol)
	if err != nil {
		return Runtime{}, err
	}
	upstream := endpoint
	if forwarding := c.forwardingFor(client, provider.ID, upstream, protocol); forwarding != "" {
		if err := ValidateEndpoint(forwarding); err != nil {
			return Runtime{}, fmt.Errorf("client %q forwarding endpoint: %w", client, err)
		}
		endpoint = strings.TrimRight(forwarding, "/")
	}
	return Runtime{
		RetryPolicy:       c.retryPolicyFor(client, selection, upstream, protocol),
		Model:             ref,
		ModelLabel:        modelLabel(model, provider, c.Families[model.Family]),
		ProviderID:        provider.ID,
		ProviderLabel:     provider.Label,
		Client:            client,
		Endpoint:          endpoint,
		UpstreamEndpoint:  upstream,
		Protocol:          protocol,
		UpstreamModel:     model.ID,
		FamilyID:          model.FamilyID(),
		ModelProvider:     selectedModelProvider(spec, selection),
		Authentication:    selectedAuthentication(selection),
		CredentialCommand: c.Clients[client].CredentialCommand,
	}, nil
}

// forwardingFor returns a client's forwarding destination for a selection that
// reaches the upstream the binding recorded, and nothing for any other.
func (c *Config) forwardingFor(client, providerID, upstream string, protocol EndpointProtocol) string {
	binding := c.Clients[client]
	if binding.ForwardingEndpoint == "" || protocol != binding.ForwardingProtocol ||
		upstreamIdentity(client, providerID, upstream) != binding.ForwardingUpstreamIdentity {
		return ""
	}
	return binding.ForwardingEndpoint
}

// modelLabel is the model's explicit label, or `Provider · Family`, or
// `Provider · model-id` for a model that is its own family.
func modelLabel(model Model, provider Provider, family Family) string {
	if model.Label != "" {
		return model.Label
	}
	providerLabel := provider.Label
	if providerLabel == "" {
		providerLabel = model.Provider
	}
	familyLabel := family.Label
	if model.Family == "" || familyLabel == "" {
		familyLabel = model.FamilyID()
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
