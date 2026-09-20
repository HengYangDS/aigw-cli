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
func ValidIdentifier(name string) bool { return profileNamePattern.MatchString(name) }

// Normalize fills deterministic defaults and canonicalizes collection state in place.
func (c *Config) Normalize() {
	if c.Accounts == nil {
		c.Accounts = map[string]Account{}
	}
	if c.Profiles == nil {
		c.Profiles = map[string]Profile{}
	}
	if c.Routes == nil {
		c.Routes = Routes{}
	}
	if c.RecommendedRoutes == nil {
		c.RecommendedRoutes = Routes{}
	}
	if c.Clients == nil {
		c.Clients = map[string]ClientBinding{}
	}
}

// Validate checks schema, accounts, profiles, routes, then client bindings without mutation.
// Within each collection, lexical key order determines the first diagnostic.
func (c Config) Validate() error {
	if err := c.validateCollections(); err != nil {
		return err
	}
	if err := c.validateRoutes(c.Routes, "route"); err != nil {
		return err
	}
	if err := c.validateRoutes(c.RecommendedRoutes, "recommended route"); err != nil {
		return err
	}
	return c.validateClientBindings()
}

func (c Config) validateCollections() error {
	if c.Version != ConfigVersion {
		return &UnsupportedConfigVersionError{Version: c.Version, ExpectedVersion: ConfigVersion}
	}
	if len(c.Profiles) == 0 {
		return errors.New("at least one profile is required")
	}
	if len(c.Accounts) == 0 {
		return errors.New("at least one account is required")
	}
	for _, name := range slices.Sorted(maps.Keys(c.Accounts)) {
		if err := c.Accounts[name].validate(name); err != nil {
			return err
		}
	}
	boundProfiles := make(map[string]bool, len(c.Clients))
	for _, binding := range c.Clients {
		if binding.Profile != "" {
			boundProfiles[binding.Profile] = true
		}
	}
	for _, name := range c.ProfileIDs() {
		if err := c.Profiles[name].validate(name, c.Accounts, boundProfiles[name]); err != nil {
			return err
		}
	}
	return nil
}

func (c Config) validateRoutes(routes Routes, kind string) error {
	for _, client := range slices.Sorted(maps.Keys(routes)) {
		if !IsAdmittedClient(client) {
			return fmt.Errorf("unknown %s %q; supported routes are %s", kind, client, AdmittedClientUsage())
		}
		profile := routes[client]
		selected, ok := c.Profiles[profile]
		if !ok {
			return fmt.Errorf("%s %q references unknown profile %q", kind, client, profile)
		}
		if selected.Client != client {
			return fmt.Errorf("%s %q selects profile %q for %q", kind, client, profile, selected.Client)
		}
	}
	return nil
}

func (c Config) validateClientBindings() error {
	for _, client := range slices.Sorted(maps.Keys(c.Clients)) {
		if !IsAdmittedClient(client) {
			return fmt.Errorf("unknown client binding %q", client)
		}
		binding := c.Clients[client]
		profile := Profile{}
		if binding.Profile != "" {
			var ok bool
			profile, ok = c.Profiles[binding.Profile]
			if !ok {
				return fmt.Errorf("client binding %q references unknown profile %q", client, binding.Profile)
			}
			if profile.Client != "" && profile.Client != client {
				return fmt.Errorf("client binding %q selects profile %q for %q", client, binding.Profile, profile.Client)
			}
			account := c.Accounts[profile.Account]
			account.ID = profile.Account
			spec, _ := ClientSpecFor(client)
			if _, _, err := spec.ResolveEndpoint(account, selectedProtocol(binding, profile)); err != nil {
				return fmt.Errorf("client binding %q: %w", client, err)
			}
		}
		if err := binding.validate(client, profile); err != nil {
			return err
		}
	}
	return nil
}

func (binding ClientBinding) validate(client string, profile Profile) error {
	if binding.ModelProvider != "" && !modelProviderPattern.MatchString(binding.ModelProvider) {
		return fmt.Errorf("client binding %q has invalid model provider %q", client, binding.ModelProvider)
	}
	if binding.ModelProvider != "" && client != ClientCodex {
		return fmt.Errorf("client binding %q model_provider is only supported for codex", client)
	}
	switch authentication := selectedAuthentication(binding, profile); authentication {
	case AuthenticationAccountToken:
	case AuthenticationClientNative:
		if client != ClientCodex {
			return fmt.Errorf("client binding %q client-native authentication is only supported for codex", client)
		}
		if binding.ModelProvider == "" {
			return fmt.Errorf("client binding %q client-native authentication requires model_provider", client)
		}
	default:
		return fmt.Errorf("client binding %q has invalid authentication %q", client, authentication)
	}
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

func (profile Profile) validate(name string, accounts map[string]Account, bound bool) error {
	if !ValidIdentifier(name) {
		return fmt.Errorf("invalid profile name %q; use letters, numbers, dot, dash, or underscore", name)
	}
	if strings.TrimSpace(profile.Label) == "" {
		return fmt.Errorf("profile %q has an empty label", name)
	}
	if profile.Account == "" {
		return fmt.Errorf("profile %q must reference an account", name)
	}
	account, ok := accounts[profile.Account]
	if !ok {
		return fmt.Errorf("profile %q references unknown account %q", name, profile.Account)
	}
	if !bound && !IsAdmittedClient(profile.Client) {
		return fmt.Errorf("profile %q has unknown client %q", name, profile.Client)
	}
	if bound && profile.Client != "" && !IsAdmittedClient(profile.Client) {
		return fmt.Errorf("profile %q has unknown client %q", name, profile.Client)
	}
	if strings.TrimSpace(profile.Model) == "" {
		return fmt.Errorf("profile %q must define a model", name)
	}
	if profile.Client == "" {
		if profile.Protocol != "" || profile.ModelProvider != "" || profile.Authentication != "" {
			return fmt.Errorf("profile %q stores client-specific options without a client binding", name)
		}
		return nil
	}
	account.ID = profile.Account
	spec, _ := ClientSpecFor(profile.Client)
	if _, _, err := spec.ResolveEndpoint(account, profile.Protocol); err != nil {
		return fmt.Errorf("profile %q: %w", name, err)
	}
	if profile.ModelProvider != "" && !modelProviderPattern.MatchString(profile.ModelProvider) {
		return fmt.Errorf("profile %q has invalid model provider %q", name, profile.ModelProvider)
	}
	if profile.ModelProvider != "" && profile.Client != ClientCodex {
		return fmt.Errorf("profile %q model_provider is only supported for codex-scoped profiles", name)
	}
	switch authentication := resolvedAuthentication(profile); authentication {
	case AuthenticationAccountToken:
	case AuthenticationClientNative:
		if profile.Client != ClientCodex {
			return fmt.Errorf("profile %q client-native authentication is only supported for codex-scoped profiles", name)
		}
		if profile.ModelProvider == "" {
			return fmt.Errorf("profile %q client-native authentication requires model_provider", name)
		}
	default:
		return fmt.Errorf("profile %q has invalid authentication %q", name, authentication)
	}
	return nil
}

func resolvedModelProvider(client string, profile Profile) string {
	if client != ClientCodex {
		return ""
	}
	if profile.ModelProvider != "" {
		return profile.ModelProvider
	}
	return ModelProviderAIGW
}

func resolvedAuthentication(profile Profile) Authentication {
	if profile.Authentication == "" {
		return AuthenticationAccountToken
	}
	return profile.Authentication
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
