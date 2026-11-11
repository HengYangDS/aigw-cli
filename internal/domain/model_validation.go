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

// Normalize fills deterministic defaults and canonicalizes collection state in
// place: every collection exists, and the display order names every entry.
func (c *Config) Normalize() {
	if c.Providers == nil {
		c.Providers = map[string]Provider{}
	}
	if c.Families == nil {
		c.Families = map[string]Family{}
	}
	if c.Models == nil {
		c.Models = map[ModelRef]Model{}
	}
	if c.Recommendations == nil {
		c.Recommendations = map[string]ClientRecommendation{}
	}
	if c.Clients == nil {
		c.Clients = map[string]ClientBinding{}
	}
	c.normalizeOrder()
}

// Validate checks schema, providers, families, models, recommendations, then
// client bindings without mutation. Within each collection, a fixed order
// determines the first diagnostic.
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
			return NewEntryError("provider `"+name+"`", err)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(c.Families)) {
		if err := c.Families[name].validate(name); err != nil {
			return NewEntryError("family `"+name+"`", err)
		}
	}
	for _, ref := range c.ModelRefs() {
		if err := c.Models[ref].validate(ref, c.Providers); err != nil {
			return NewEntryError("model `"+ref.String()+"`", err)
		}
	}
	return nil
}

func (c *Config) validateRecommendations() error {
	for _, client := range slices.Sorted(maps.Keys(c.Recommendations)) {
		entry := "the `" + client + "` recommendation"
		if !c.admission.Admits(client) {
			return NewEntryError(entry, fmt.Errorf("unknown client recommendation %q; supported clients are %s", client, c.admission.Usage()))
		}
		recommendation := c.Recommendations[client]
		if recommendation.Primary.Model.IsZero() {
			return NewEntryError(entry, fmt.Errorf("client recommendation %q has no primary model", client))
		}
		for index, selection := range append([]ClientSelection{recommendation.Primary}, recommendation.Alternatives...) {
			// A recommended protocol that the provider has no endpoint for
			// waits for one: selection skips the choice until then.
			err := c.validateSelection(client, selection)
			if c.waitsForEndpoint(client, selection, err) {
				spec, _ := c.admission.Spec(client)
				err = selection.validate(spec, c.admission)
			}
			if err != nil {
				return NewEntryError(entry, fmt.Errorf("client recommendation %q choice %d: %w", client, index+1, err))
			}
			if c.Models[selection.Model].Deprecated {
				return NewEntryError(entry, fmt.Errorf("client recommendation %q choice %d names the deprecated model %s", client, index+1, selection.Model))
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
		if binding.Model.IsZero() {
			if binding.Enabled || binding.Protocol != "" || binding.ModelProvider != "" || binding.Authentication != "" || binding.ForwardingEndpoint != "" || binding.RetryPolicy != nil {
				return fmt.Errorf("client %q must select a model before it can be enabled or define runtime options", client)
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

// validateForwarding refuses a forwarded binding whose model no longer reaches
// the upstream recorded with its endpoint. Rebinding the destination silently
// would send the client's traffic somewhere the operator did not choose.
func (c *Config) validateForwarding(client string, binding ClientBinding) error {
	if binding.ForwardingEndpoint == "" {
		return nil
	}
	runtime, err := c.ResolveRuntime(client, ModelRef{})
	if err != nil {
		return fmt.Errorf("client binding %q: %w", client, err)
	}
	if runtime.CredentialProjectionFingerprint(client) == binding.ForwardingUpstreamIdentity && runtime.Protocol == binding.ForwardingProtocol {
		return nil
	}
	// The direct connection reaches the same provider, so it keeps the
	// client's retry budgets.
	return fmt.Errorf("client %q forwards to the upstream it was selected with, and this change would move that upstream; run `aigw client edit %s --direct` first", client, client)
}

func (c *Config) validateSelection(client string, selection ClientSelection) error {
	model, ok := c.Models[selection.Model]
	if !ok {
		return fmt.Errorf("names the unknown model %s", selection.Model)
	}
	provider := c.Providers[model.Provider]
	provider.ID = model.Provider
	spec, _ := c.admission.Spec(client)
	if _, _, err := spec.resolveModelEndpoint(provider, model, selection.Protocol); err != nil {
		return fmt.Errorf("model %s: %w", selection.Model, err)
	}
	return selection.validate(spec, c.admission)
}

// waitsForEndpoint reports a recommendation choice that fails only because
// its provider has no endpoint for its protocol, or, without a named
// protocol, for a protocol that both the client and the model speak.
func (c *Config) waitsForEndpoint(client string, selection ClientSelection, err error) bool {
	if _, missing := errors.AsType[*RuntimeMissingEndpointError](err); missing || err == nil {
		return missing
	}
	model, known := c.Models[selection.Model]
	spec, admitted := c.admission.Spec(client)
	if !known || !admitted || selection.Protocol != "" {
		return false
	}
	provider := c.Providers[model.Provider]
	return slices.ContainsFunc(spec.EndpointProtocols, func(protocol EndpointProtocol) bool {
		_, declared := model.Protocols[protocol]
		return declared && provider.Endpoints.For(protocol) == ""
	})
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
	if provider.Endpoints.Responses == "" && provider.Endpoints.Anthropic == "" && provider.Endpoints.Chat == "" {
		return fmt.Errorf("account %q must define at least one endpoint", name)
	}
	for _, endpoint := range []struct {
		protocol EndpointProtocol
		url      string
	}{
		{ProtocolAnthropic, provider.Endpoints.Anthropic},
		{ProtocolResponses, provider.Endpoints.Responses},
		{ProtocolChat, provider.Endpoints.Chat},
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
	return nil
}

// validate checks one model, stored under ref, against the providers. A
// family that the model names needs an entry only for its label: without one,
// its ID labels it. A model may declare a protocol for which its provider has
// no endpoint, as a team file leaves one whose provider keeps its local
// endpoints: the protocol is unusable until the provider has the endpoint,
// because no client selection resolves on it.
func (model Model) validate(ref ModelRef, providers map[string]Provider) error {
	if model.Ref() != ref {
		return fmt.Errorf("model %s is stored as %s", model.Ref(), ref)
	}
	if _, ok := providers[model.Provider]; !ok {
		return fmt.Errorf("model %s names the unknown provider %q", ref, model.Provider)
	}
	if strings.TrimSpace(model.ID) == "" {
		return fmt.Errorf("provider %q has a model with an empty ID", model.Provider)
	}
	if model.Family != "" && !ValidIdentifier(model.Family) {
		return fmt.Errorf("model %s names the invalid family %q", ref, model.Family)
	}
	for _, protocol := range model.AdmittedProtocols() {
		if !slices.Contains(knownProtocols, protocol) {
			return fmt.Errorf("model %s has the unknown protocol %q", ref, protocol)
		}
		capabilities := model.Protocols[protocol]
		seenCapabilities := make(map[Capability]bool, len(capabilities))
		for _, capability := range capabilities {
			if !capability.Known() {
				return fmt.Errorf("model %s has the unknown capability %q on %s", ref, capability, protocol)
			}
			if seenCapabilities[capability] {
				return fmt.Errorf("model %s repeats the capability %q on %s", ref, capability, protocol)
			}
			if !protocol.Admits(capability) {
				return fmt.Errorf("model %s declares %q on %s, which only the %s protocol admits", ref, capability, protocol, ProtocolAnthropic)
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
