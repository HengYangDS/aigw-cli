package configuration

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
	if c.Accounts == nil {
		c.Accounts = map[string]Account{}
	}
	if c.Models == nil {
		c.Models = map[string]Model{}
	}
	if c.Routes == nil {
		c.Routes = map[string]Route{}
	}
	if c.Recommendations == nil {
		c.Recommendations = map[string]ClientRecommendation{}
	}
	if c.Clients == nil {
		c.Clients = map[string]ClientBinding{}
	}
	for _, id := range c.RouteIDs() {
		route := c.Routes[id]
		if route.UpstreamModel == "" {
			route.UpstreamModel = route.Model
		}
		if route.Model != "" {
			if _, exists := c.Models[route.Model]; !exists {
				c.Models[route.Model] = Model{Label: route.Model}
			}
		}
		c.Routes[id] = route
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

func (c *Config) validateCollections() error {
	if c.Version != ConfigVersion {
		return &UnsupportedConfigVersionError{Version: c.Version, ExpectedVersion: ConfigVersion}
	}
	if len(c.Routes) == 0 {
		return errors.New("at least one route is required")
	}
	if len(c.Accounts) == 0 {
		return errors.New("at least one account is required")
	}
	for _, name := range slices.Sorted(maps.Keys(c.Accounts)) {
		if err := c.Accounts[name].validate(name); err != nil {
			return err
		}
	}
	for _, name := range slices.Sorted(maps.Keys(c.Models)) {
		if err := c.Models[name].validate(name); err != nil {
			return err
		}
	}
	for _, name := range c.RouteIDs() {
		if err := c.Routes[name].validate(name, c.Accounts, c.Models); err != nil {
			return err
		}
	}
	return nil
}

func (c *Config) validateRecommendations() error {
	for _, client := range slices.Sorted(maps.Keys(c.Recommendations)) {
		if !IsAdmittedClient(client) {
			return fmt.Errorf("unknown client recommendation %q; supported clients are %s", client, AdmittedClientUsage())
		}
		recommendation := c.Recommendations[client]
		if recommendation.Primary.Route == "" {
			return fmt.Errorf("client recommendation %q has no primary route", client)
		}
		for index, selection := range append([]ClientSelection{recommendation.Primary}, recommendation.Alternatives...) {
			if err := c.validateSelection(client, selection); err != nil {
				return fmt.Errorf("client recommendation %q choice %d: %w", client, index+1, err)
			}
			if c.Routes[selection.Route].LifecycleState() == RouteDeprecated {
				return fmt.Errorf("client recommendation %q choice %d references deprecated route %q", client, index+1, selection.Route)
			}
		}
	}
	return nil
}

func (c *Config) validateClientBindings() error {
	for _, client := range slices.Sorted(maps.Keys(c.Clients)) {
		if !IsAdmittedClient(client) {
			return fmt.Errorf("unknown client binding %q", client)
		}
		binding := c.clientBinding(client)
		if binding.Route == "" {
			if binding.Enabled || binding.Protocol != "" || binding.ModelProvider != "" || binding.Authentication != "" {
				return fmt.Errorf("client binding %q must select a route before it can be enabled or define runtime options", client)
			}
			if err := binding.validate(client); err != nil {
				return err
			}
			continue
		}
		if err := c.validateSelection(client, binding.selection()); err != nil {
			return fmt.Errorf("client binding %q: %w", client, err)
		}
		if err := binding.validate(client); err != nil {
			return err
		}
	}
	return nil
}

func (c *Config) validateSelection(client string, selection ClientSelection) error {
	route, ok := c.Routes[selection.Route]
	if !ok {
		return fmt.Errorf("references unknown route %q", selection.Route)
	}
	account := c.Accounts[route.Account]
	account.ID = route.Account
	spec, _ := ClientSpecFor(client)
	if _, _, err := spec.ResolveRouteEndpoint(account, route, selection.Protocol); err != nil {
		return fmt.Errorf("route %q: %w", selection.Route, err)
	}
	return selection.validate(client)
}

func (selection ClientSelection) validate(client string) error {
	if selection.ModelProvider != "" && !modelProviderPattern.MatchString(selection.ModelProvider) {
		return fmt.Errorf("invalid model provider %q", selection.ModelProvider)
	}
	if selection.ModelProvider != "" && client != ClientCodex {
		return fmt.Errorf("model_provider is only supported for codex")
	}
	switch authentication := selectedAuthentication(selection); authentication {
	case AuthenticationAccountToken:
	case AuthenticationClientNative:
		if client != ClientCodex {
			return fmt.Errorf("client-native authentication is only supported for codex")
		}
		if selection.ModelProvider == "" {
			return fmt.Errorf("client-native authentication requires model_provider")
		}
	default:
		return fmt.Errorf("invalid authentication %q", authentication)
	}
	return nil
}

func (binding ClientBinding) validate(client string) error {
	command := binding.CredentialCommand
	if command != "" && (!filepath.IsAbs(command) || strings.TrimSpace(command) != command || strings.ContainsFunc(command, unicode.IsControl)) {
		return fmt.Errorf("client binding %q credential_command must be one absolute executable path", client)
	}
	return nil
}

func (account Account) validate(name string) error {
	if !ValidIdentifier(name) {
		return fmt.Errorf("invalid account name %q; use letters, numbers, dot, dash, or underscore", name)
	}
	if name != strings.ToLower(name) {
		return fmt.Errorf("invalid account name %q; environment-backed account IDs must be lowercase", name)
	}
	if strings.TrimSpace(account.Label) == "" {
		return fmt.Errorf("account %q has an empty label", name)
	}
	if account.Endpoints.OpenAIResponses == "" && account.Endpoints.Anthropic == "" && account.Endpoints.OpenAIChatCompletions == "" {
		return fmt.Errorf("account %q must define at least one endpoint", name)
	}
	for _, endpoint := range []struct {
		protocol EndpointProtocol
		url      string
	}{
		{ProtocolAnthropic, account.Endpoints.Anthropic},
		{ProtocolOpenAIResponses, account.Endpoints.OpenAIResponses},
		{ProtocolOpenAIChatCompletions, account.Endpoints.OpenAIChatCompletions},
	} {
		if endpoint.url == "" {
			continue
		}
		if err := validateEndpoint(endpoint.url); err != nil {
			return fmt.Errorf("account %q endpoint %s: %w", name, endpoint.protocol, err)
		}
	}
	if account.AccountProbe == nil {
		return nil
	}
	if !ValidIdentifier(account.AccountProbe.Kind) {
		return fmt.Errorf("account %q has invalid account probe provider %q", name, account.AccountProbe.Kind)
	}
	if err := validateEndpoint(account.AccountProbe.BaseURL); err != nil {
		return fmt.Errorf("account %q account probe: %w", name, err)
	}
	return nil
}

func (model Model) validate(name string) error {
	if !ValidIdentifier(name) {
		return fmt.Errorf("invalid model name %q; use letters, numbers, dot, dash, or underscore", name)
	}
	if strings.TrimSpace(model.Label) == "" {
		return fmt.Errorf("model %q has an empty label", name)
	}
	return nil
}

func (route Route) validate(name string, accounts map[string]Account, models map[string]Model) error {
	if !ValidIdentifier(name) {
		return fmt.Errorf("invalid route name %q; use letters, numbers, dot, dash, or underscore", name)
	}
	if route.Account == "" {
		return fmt.Errorf("route %q must reference an account", name)
	}
	account, ok := accounts[route.Account]
	if !ok {
		return fmt.Errorf("route %q references unknown account %q", name, route.Account)
	}
	if strings.TrimSpace(route.Model) == "" {
		return fmt.Errorf("route %q must reference a model", name)
	}
	if len(models) > 0 {
		if _, ok := models[route.Model]; !ok {
			return fmt.Errorf("route %q references unknown model %q", name, route.Model)
		}
	}
	if route.UpstreamModel != "" && strings.TrimSpace(route.UpstreamModel) == "" {
		return fmt.Errorf("route %q has an empty upstream model", name)
	}
	switch route.LifecycleState() {
	case RouteAdmitted, RouteDeprecated:
	default:
		return fmt.Errorf("route %q has invalid lifecycle %q", name, route.Lifecycle)
	}
	protocols := routeAdmittedProtocols(route)
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
		if account.Endpoints.For(protocol) == "" {
			return fmt.Errorf("route %q: %w", name, &RuntimeMissingEndpointError{AccountID: route.Account, Protocol: protocol})
		}
		seen[protocol] = true
	}
	for protocol, capabilities := range route.Interfaces {
		seenCapabilities := make(map[Capability]bool, len(capabilities))
		for _, capability := range capabilities {
			if !knownCapability(capability) {
				return fmt.Errorf("route %q interface %q has unknown capability %q", name, protocol, capability)
			}
			if seenCapabilities[capability] {
				return fmt.Errorf("route %q interface %q repeats capability %q", name, protocol, capability)
			}
			seenCapabilities[capability] = true
		}
	}
	return nil
}

func knownCapability(capability Capability) bool {
	switch capability {
	case CapabilityText, CapabilityReasoning, CapabilityStreaming, CapabilityTools,
		CapabilityStructuredOutput, CapabilityContinuation, CapabilityCompaction,
		CapabilityMultimodalInput:
		return true
	default:
		return false
	}
}

func validateEndpoint(raw string) error {
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
