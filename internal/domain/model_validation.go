package domain

import (
	"errors"
	"fmt"
	"maps"
	"net/netip"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

// ValidIdentifier reports whether a user-defined identifier is safe for configuration and credential slots.
func ValidIdentifier(name string) bool { return identifierPattern.MatchString(name) }

// Normalize fills deterministic defaults and canonicalizes collection state in place.
func (c *Config) Normalize() {
	if c.Providers == nil {
		c.Providers = map[string]Provider{}
	}
	if c.Families == nil {
		c.Families = map[string]Family{}
	}
	if c.Models == nil {
		c.Models = map[string]Model{}
	}
	if c.Recommendations == nil {
		c.Recommendations = map[string]ClientRecommendation{}
	}
	if c.Clients == nil {
		c.Clients = map[string]ClientBinding{}
	}
	for _, id := range c.ModelIDs() {
		route := c.Models[id]
		if route.UpstreamModel == "" {
			route.UpstreamModel = route.Family
		}
		if route.Family != "" {
			if _, exists := c.Families[route.Family]; !exists {
				c.Families[route.Family] = Family{Label: route.Family}
			}
		}
		c.Models[id] = route
	}
}

// Validate checks schema, accounts, routes, recommendations, then client bindings without mutation.
// Within each collection, lexical key order determines the first diagnostic.
func (c *Config) Validate() error {
	if err := c.validateCollections(); err != nil {
		return err
	}
	if err := c.validateRecommendations(); err != nil {
		return err
	}
	return c.validateClientBindings()
}

// validateCollections checks the version and each provider, family and
// model. A configuration may hold providers without models, or nothing at
// all: `aigw provider add` starts from an empty home, and removing the last
// model or provider leaves a valid configuration whose clients select
// nothing.
func (c *Config) validateCollections() error {
	if c.Version != ConfigVersion {
		return &UnsupportedConfigVersionError{Version: c.Version, ExpectedVersion: ConfigVersion}
	}
	for _, name := range slices.Sorted(maps.Keys(c.Providers)) {
		if err := c.Providers[name].validate(name); err != nil {
			return err
		}
	}
	for _, name := range slices.Sorted(maps.Keys(c.Families)) {
		if err := c.Families[name].validate(name); err != nil {
			return err
		}
	}
	for _, name := range c.ModelIDs() {
		if err := c.Models[name].validate(name, c.Providers, c.Families); err != nil {
			return err
		}
	}
	return nil
}

func (c *Config) validateRecommendations() error {
	for _, client := range slices.Sorted(maps.Keys(c.Recommendations)) {
		if !c.admission.Admits(client) {
			return fmt.Errorf("unknown client recommendation %q; supported clients are %s", client, c.admission.Usage())
		}
		recommendation := c.Recommendations[client]
		if recommendation.Primary.Model == "" {
			return fmt.Errorf("client recommendation %q has no primary route", client)
		}
		for index, selection := range append([]ClientSelection{recommendation.Primary}, recommendation.Alternatives...) {
			if err := c.validateSelection(client, selection); err != nil {
				return fmt.Errorf("client recommendation %q choice %d: %w", client, index+1, err)
			}
			if c.Models[selection.Model].LifecycleState() == ModelDeprecated {
				return fmt.Errorf("client recommendation %q choice %d references deprecated route %q", client, index+1, selection.Model)
			}
		}
	}
	return nil
}

func (c *Config) validateClientBindings() error {
	for _, client := range slices.Sorted(maps.Keys(c.Clients)) {
		spec, admitted := c.admission.Spec(client)
		if !admitted {
			return fmt.Errorf("unknown client binding %q", client)
		}
		binding := c.clientBinding(client)
		if binding.Model == "" {
			if binding.Enabled || binding.Protocol != "" || binding.ModelProvider != "" || binding.Authentication != "" || binding.ForwardingEndpoint != "" || binding.RetryPolicy != nil {
				return fmt.Errorf("client binding %q must select a route before it can be enabled or define runtime options", client)
			}
			if err := binding.validate(client, spec); err != nil {
				return err
			}
			continue
		}
		if err := c.validateSelection(client, binding.selection()); err != nil {
			return fmt.Errorf("client binding %q: %w", client, err)
		}
		if err := binding.validate(client, spec); err != nil {
			return err
		}
		if err := c.validateForwarding(client, binding); err != nil {
			return err
		}
	}
	return nil
}

// validateForwarding refuses a forwarded binding whose Route no longer reaches
// the upstream recorded with its endpoint. Rebinding the destination silently
// would send the client's traffic somewhere the operator did not choose.
func (c *Config) validateForwarding(client string, binding ClientBinding) error {
	if binding.ForwardingEndpoint == "" {
		return nil
	}
	runtime, err := c.ResolveRuntime(client, "")
	if err != nil {
		return fmt.Errorf("client binding %q: %w", client, err)
	}
	if runtime.CredentialProjectionFingerprint(client) == binding.ForwardingUpstreamIdentity && runtime.Protocol == binding.ForwardingProtocol {
		return nil
	}
	direct := "aigw use --for " + client + " " + binding.Model + " --direct"
	if binding.RetryPolicy != nil {
		// The direct connection is a new connection, which a selected retry
		// policy would otherwise refuse.
		direct += " --native-retries"
	}
	return fmt.Errorf("client %q forwards to the upstream it was selected with, and this change would move that upstream; run `%s` first", client, direct)
}

func (c *Config) validateSelection(client string, selection ClientSelection) error {
	route, ok := c.Models[selection.Model]
	if !ok {
		return fmt.Errorf("references unknown route %q", selection.Model)
	}
	account := c.Providers[route.Provider]
	account.ID = route.Provider
	spec, _ := c.admission.Spec(client)
	if _, _, err := spec.resolveModelEndpoint(account, route, selection.Protocol); err != nil {
		return fmt.Errorf("route %q: %w", selection.Model, err)
	}
	return selection.validate(spec, c.admission)
}

func (selection ClientSelection) validate(spec ClientSpec, admission Admission) error {
	if selection.ModelProvider != "" && !modelProviderPattern.MatchString(selection.ModelProvider) {
		return fmt.Errorf("invalid model provider %q", selection.ModelProvider)
	}
	if selection.ModelProvider != "" && spec.DefaultModelProvider == "" {
		return fmt.Errorf("model_provider is only supported for %s", admission.usageWith(func(spec ClientSpec) bool { return spec.DefaultModelProvider != "" }))
	}
	switch authentication := selectedAuthentication(selection); authentication {
	case AuthenticationProviderToken:
	case AuthenticationClientNative:
		if !spec.ClientNativeAuthentication {
			return fmt.Errorf("client-native authentication is only supported for %s", admission.usageWith(func(spec ClientSpec) bool { return spec.ClientNativeAuthentication }))
		}
		if selection.ModelProvider == "" {
			return fmt.Errorf("client-native authentication requires model_provider")
		}
	default:
		return fmt.Errorf("invalid authentication %q", authentication)
	}
	return nil
}

func (binding ClientBinding) validate(client string, spec ClientSpec) error {
	if err := binding.RetryPolicy.validate(client, spec); err != nil {
		return err
	}
	if binding.ForwardingEndpoint != "" {
		if err := ValidateEndpoint(binding.ForwardingEndpoint); err != nil {
			return fmt.Errorf("client binding %q forwarding endpoint: %w", client, err)
		}
	}
	command := binding.CredentialCommand
	if command != "" && (!filepath.IsAbs(command) || strings.TrimSpace(command) != command || strings.ContainsFunc(command, unicode.IsControl)) {
		return fmt.Errorf("client binding %q credential_command must be one absolute executable path", client)
	}
	return nil
}

func (provider Provider) validate(name string) error {
	if !ValidIdentifier(name) {
		return fmt.Errorf("invalid account name %q; use letters, numbers, dot, dash, or underscore", name)
	}
	if name != strings.ToLower(name) {
		return fmt.Errorf("invalid account name %q; environment-backed account IDs must be lowercase", name)
	}
	if strings.TrimSpace(provider.Label) == "" {
		return fmt.Errorf("account %q has an empty label", name)
	}
	if provider.Order < 0 {
		return fmt.Errorf("account %q order must not be negative", name)
	}
	if provider.Endpoints.OpenAIResponses == "" && provider.Endpoints.Anthropic == "" && provider.Endpoints.OpenAIChatCompletions == "" {
		return fmt.Errorf("account %q must define at least one endpoint", name)
	}
	for _, endpoint := range []struct {
		protocol EndpointProtocol
		url      string
	}{
		{ProtocolAnthropic, provider.Endpoints.Anthropic},
		{ProtocolOpenAIResponses, provider.Endpoints.OpenAIResponses},
		{ProtocolOpenAIChatCompletions, provider.Endpoints.OpenAIChatCompletions},
	} {
		if endpoint.url == "" {
			continue
		}
		if err := ValidateEndpoint(endpoint.url); err != nil {
			return fmt.Errorf("account %q endpoint %s: %w", name, endpoint.protocol, err)
		}
	}
	if provider.BalanceProbe == nil {
		return nil
	}
	if !ValidIdentifier(provider.BalanceProbe.Kind) {
		return fmt.Errorf("account %q has invalid account probe provider %q", name, provider.BalanceProbe.Kind)
	}
	if err := ValidateEndpoint(provider.BalanceProbe.BaseURL); err != nil {
		return fmt.Errorf("account %q account probe: %w", name, err)
	}
	return nil
}

func (family Family) validate(name string) error {
	if !ValidIdentifier(name) {
		return fmt.Errorf("invalid model name %q; use letters, numbers, dot, dash, or underscore", name)
	}
	if strings.TrimSpace(family.Label) == "" {
		return fmt.Errorf("model %q has an empty label", name)
	}
	if family.Order < 0 {
		return fmt.Errorf("model %q order must not be negative", name)
	}
	return nil
}

func (model Model) validate(name string, providers map[string]Provider, families map[string]Family) error {
	if !ValidIdentifier(name) {
		return fmt.Errorf("invalid route name %q; use letters, numbers, dot, dash, or underscore", name)
	}
	if model.Provider == "" {
		return fmt.Errorf("route %q must reference an account", name)
	}
	provider, ok := providers[model.Provider]
	if !ok {
		return fmt.Errorf("route %q references unknown account %q", name, model.Provider)
	}
	if strings.TrimSpace(model.Family) == "" {
		return fmt.Errorf("route %q must reference a model", name)
	}
	if len(families) > 0 {
		if _, ok := families[model.Family]; !ok {
			return fmt.Errorf("route %q references unknown model %q", name, model.Family)
		}
	}
	if model.UpstreamModel != "" && strings.TrimSpace(model.UpstreamModel) == "" {
		return fmt.Errorf("route %q has an empty upstream model", name)
	}
	switch model.LifecycleState() {
	case ModelAdmitted, ModelDeprecated:
	default:
		return fmt.Errorf("route %q has invalid lifecycle %q", name, model.Lifecycle)
	}
	protocols := modelAdmittedProtocols(model)
	seen := make(map[EndpointProtocol]bool, len(protocols))
	for _, protocol := range protocols {
		switch protocol {
		case ProtocolAnthropic, ProtocolOpenAIResponses, ProtocolOpenAIChatCompletions:
		default:
			return fmt.Errorf("route %q has unknown protocol %q", name, protocol)
		}
		if seen[protocol] {
			return fmt.Errorf("route %q repeats protocol %q", name, protocol)
		}
		if provider.Endpoints.For(protocol) == "" {
			return fmt.Errorf("route %q: %w", name, &RuntimeMissingEndpointError{ProviderID: model.Provider, Protocol: protocol})
		}
		seen[protocol] = true
	}
	for protocol, capabilities := range model.Protocols {
		seenCapabilities := make(map[Capability]bool, len(capabilities))
		for _, capability := range capabilities {
			if !capability.Known() {
				return fmt.Errorf("route %q interface %q has unknown capability %q", name, protocol, capability)
			}
			if seenCapabilities[capability] {
				return fmt.Errorf("route %q interface %q repeats capability %q", name, protocol, capability)
			}
			if !protocol.Admits(capability) {
				return fmt.Errorf("route %q interface %q declares %q, which only the %s interface admits", name, protocol, capability, ProtocolAnthropic)
			}
			seenCapabilities[capability] = true
		}
	}
	return nil
}

// capabilities are the capabilities that AIGW defines, in display order.
var capabilities = []Capability{
	CapabilityText, CapabilityReasoning, CapabilityStreaming, CapabilityTools,
	CapabilityStructuredOutput, CapabilityContinuation, CapabilityCompaction,
	CapabilityMultimodalInput, CapabilityToolSearch,
}

// Capabilities returns the capabilities that AIGW defines, in display order.
func Capabilities() []Capability { return slices.Clone(capabilities) }

// Known reports whether AIGW defines this capability.
func (capability Capability) Known() bool { return slices.Contains(capabilities, capability) }

// Admits reports whether protocol admits capability: tool search is an
// Anthropic Messages shape, and every other capability may be qualified on any
// protocol.
func (protocol EndpointProtocol) Admits(capability Capability) bool {
	return capability != CapabilityToolSearch || protocol == ProtocolAnthropic
}

// ValidateEndpoint checks a URL that a provider endpoint, a balance probe or a
// forwarding destination may use: HTTPS, or plain HTTP to a loopback host,
// without userinfo or a credential-like query parameter. Its errors name at
// most a query parameter, never a value from the URL.
func ValidateEndpoint(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return errors.New("URL must use http or https and include a host")
	}
	if u.User != nil {
		return errors.New("URL userinfo is forbidden")
	}
	if u.Scheme == "http" && !IsLoopbackHost(u.Hostname()) {
		return errors.New("plain HTTP is allowed only for a loopback endpoint")
	}
	for _, key := range slices.Sorted(maps.Keys(u.Query())) {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "key") || strings.Contains(lower, "auth") || strings.Contains(lower, "password") {
			return fmt.Errorf("credential-like query parameter %q is forbidden", key)
		}
	}
	return nil
}

// IsLoopbackHost classifies a URL hostname without DNS or service discovery.
func IsLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	address, err := netip.ParseAddr(host)
	return err == nil && address.IsLoopback()
}
