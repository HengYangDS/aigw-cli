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
	if c.Recommendations == nil {
		c.Recommendations = map[string]ClientSelection{}
	}
	if c.Clients == nil {
		c.Clients = map[string]ClientBinding{}
	}
}

// Validate checks schema, accounts, profiles, recommendations, then client bindings without mutation.
// Within each collection, lexical key order determines the first diagnostic.
func (c *Config) Validate() error {
	if err := c.validateCollections(); err != nil {
		return err
	}
	if err := c.validateSelections(c.Recommendations, "recommendation"); err != nil {
		return err
	}
	return c.validateClientBindings()
}

func (c *Config) validateCollections() error {
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
	for _, name := range c.ProfileIDs() {
		if err := c.Profiles[name].validate(name, c.Accounts); err != nil {
			return err
		}
	}
	return nil
}

func (c *Config) validateSelections(selections map[string]ClientSelection, kind string) error {
	for _, client := range slices.Sorted(maps.Keys(selections)) {
		if !IsAdmittedClient(client) {
			return fmt.Errorf("unknown client %s %q; supported clients are %s", kind, client, AdmittedClientUsage())
		}
		selection := selections[client]
		if err := c.validateSelection(client, selection); err != nil {
			return fmt.Errorf("client %s %q: %w", kind, client, err)
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
		if binding.Profile == "" {
			if binding.Enabled || binding.Protocol != "" || binding.ModelProvider != "" || binding.Authentication != "" {
				return fmt.Errorf("client binding %q must select a profile before it can be enabled or define runtime options", client)
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
	profile, ok := c.Profiles[selection.Profile]
	if !ok {
		return fmt.Errorf("references unknown profile %q", selection.Profile)
	}
	account := c.Accounts[profile.Account]
	account.ID = profile.Account
	spec, _ := ClientSpecFor(client)
	if _, _, err := spec.ResolveProfileEndpoint(account, profile, selection.Protocol); err != nil {
		return fmt.Errorf("profile %q: %w", selection.Profile, err)
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

func (profile Profile) validate(name string, accounts map[string]Account) error {
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
	if strings.TrimSpace(profile.Model) == "" {
		return fmt.Errorf("profile %q must define a model", name)
	}
	switch profile.Tier {
	case "", ModelTierFlagship, ModelTierDaily:
	default:
		return fmt.Errorf("profile %q has unknown tier %q", name, profile.Tier)
	}
	if profile.Protocols != nil && len(profile.Protocols) == 0 {
		return fmt.Errorf("profile %q protocols must be omitted or contain at least one verified protocol", name)
	}
	seen := make(map[EndpointProtocol]bool, len(profile.Protocols))
	for _, protocol := range profile.Protocols {
		switch protocol {
		case ProtocolAnthropic, ProtocolOpenAIResponses, ProtocolOpenAIChatCompletions:
		default:
			return fmt.Errorf("profile %q has unknown protocol %q", name, protocol)
		}
		if seen[protocol] {
			return fmt.Errorf("profile %q repeats protocol %q", name, protocol)
		}
		if account.Endpoints.For(protocol) == "" {
			return fmt.Errorf("profile %q admits protocol %q without an Account endpoint", name, protocol)
		}
		seen[protocol] = true
	}
	return nil
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
