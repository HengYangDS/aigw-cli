// Package configuration owns AIGW's complete configuration domain: its
// schema, validation, persistence, and token-free interchange format.
package configuration

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

const (
	ConfigVersion     = 3
	ClientClaude      = "claude"
	ClientCodex       = "codex"
	ModelProviderAIGW = "aigw"
)

var profileNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
var modelProviderPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

type Config struct {
	Version  int                      `toml:"version" json:"version"`
	Accounts map[string]Account       `toml:"accounts,omitempty" json:"accounts,omitempty"`
	Profiles map[string]Profile       `toml:"profiles" json:"profiles"`
	Routes   Routes                   `toml:"routes" json:"routes"`
	Adapters map[string]AdapterConfig `toml:"adapters,omitempty" json:"adapters,omitempty"`
}

type Account struct {
	ID           string        `toml:"-" json:"id,omitempty"`
	Label        string        `toml:"label" json:"label"`
	Endpoints    Endpoints     `toml:"endpoints" json:"endpoints"`
	AccountProbe *AccountProbe `toml:"account_probe,omitempty" json:"account_probe,omitempty"`
}

type Profile struct {
	ID            string `toml:"-" json:"id,omitempty"`
	Label         string `toml:"label" json:"label"`
	Purpose       string `toml:"purpose,omitempty" json:"purpose,omitempty"`
	Account       string `toml:"account" json:"account"`
	Client        string `toml:"client" json:"client"`
	Model         string `toml:"model" json:"model"`
	ModelProvider string `toml:"model_provider,omitempty" json:"model_provider,omitempty"`
}

type Runtime struct {
	ProfileID         string `json:"profile_id"`
	ProfileLabel      string `json:"profile_label"`
	AccountID         string `json:"account_id"`
	AccountLabel      string `json:"account_label"`
	Client            string `json:"client"`
	Endpoint          string `json:"endpoint"`
	Model             string `json:"model,omitempty"`
	ModelProvider     string `json:"model_provider"`
	CredentialCommand string `json:"-"`
}

type AccountProbe struct {
	Kind    string `toml:"kind" json:"kind"`
	BaseURL string `toml:"base_url" json:"base_url"`
}

type Endpoints struct {
	OpenAIResponses string `toml:"openai_responses,omitempty" json:"openai_responses,omitempty"`
	Anthropic       string `toml:"anthropic,omitempty" json:"anthropic,omitempty"`
}

// Routes maps each admitted client to its selected Profile. There is no global
// fallback because a client-scoped Profile cannot represent another client's
// protocol or model.
type Routes map[string]string

type AdapterConfig struct {
	Enabled    bool     `toml:"enabled" json:"enabled"`
	Executable string   `toml:"executable,omitempty" json:"executable,omitempty"`
	Targets    []string `toml:"targets,omitempty" json:"targets,omitempty"`
}

func NewConfig() Config {
	return Config{Version: ConfigVersion, Accounts: map[string]Account{}, Profiles: map[string]Profile{}, Routes: Routes{}, Adapters: map[string]AdapterConfig{}}
}

// Clone returns an independent configuration value. Config is the semantic
// owner of this copy operation so mutation workflows do not reproduce its
// nested map and slice shape.
func (c Config) Clone() Config {
	return c.normalizedCopy()
}

// ProfileIDs returns the stable lexical order used by every CLI projection.
func (c Config) ProfileIDs() []string {
	ids := make([]string, 0, len(c.Profiles))
	for id := range c.Profiles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// ClientForProfile returns the canonical client declared by a named
// profile. Callers must not infer client scope from models, endpoints, routes,
// or profile names when this declaration is absent.
func (c Config) ClientForProfile(name string) (string, error) {
	profile, ok := c.Profiles[name]
	if !ok {
		return "", fmt.Errorf("unknown profile %q", name)
	}
	if !IsAdmittedClient(profile.Client) {
		return "", fmt.Errorf("profile %q does not declare a client; provide --for %s", name, AdmittedClientUsage())
	}
	return profile.Client, nil
}

// ResolveAccount accepts either an Account ID or a Profile ID and returns the
// referenced Account with its map identity populated.
func (c Config) ResolveAccount(reference string) (string, Account, error) {
	if account, ok := c.Accounts[reference]; ok {
		account.ID = reference
		return reference, account, nil
	}
	profile, ok := c.Profiles[reference]
	if !ok {
		return "", Account{}, fmt.Errorf("unknown account or profile %q", reference)
	}
	account, ok := c.Accounts[profile.Account]
	if !ok {
		return "", Account{}, fmt.Errorf("profile %q references unknown account %q", reference, profile.Account)
	}
	account.ID = profile.Account
	return profile.Account, account, nil
}

// FirstProfileForClient returns the first stable profile that can resolve the
// requested client's model or endpoint without changing a route.
func (c Config) FirstProfileForClient(client string) string {
	for _, id := range c.ProfileIDs() {
		profile := c.Profiles[id]
		if profile.Client != "" && profile.Client != client {
			continue
		}
		if profile.Model != "" {
			return id
		}
		if account, ok := c.Accounts[profile.Account]; ok {
			account.ID = profile.Account
			if _, err := account.EndpointFor(client); err == nil {
				return id
			}
		}
	}
	return ""
}

// RouteUsesAccount reports whether the resolved route for client selects the
// given Account.
func (c Config) RouteUsesAccount(client, accountID string) bool {
	runtime, err := c.ResolveRuntime(client, "")
	return err == nil && runtime.AccountID != "" && runtime.AccountID == accountID
}

// RoutedAccountIDs returns the stable set of Accounts selected by resolvable
// admitted-client Routes. Catalogue Accounts that no active Route selects are
// capabilities available for later connection, not current runtime
// dependencies.
func (c Config) RoutedAccountIDs() []string {
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

// SelectRoutesForConnectedAccounts preserves the complete capability catalogue
// while choosing routes that can use one of the locally connected Accounts.
// The existing recommendation wins whenever it is already usable; otherwise
// lexical Profile order makes the replacement deterministic.
func (c Config) SelectRoutesForConnectedAccounts(accountIDs []string) (Config, error) {
	selected := c.Clone()
	connected := make(map[string]bool, len(accountIDs))
	for _, accountID := range accountIDs {
		if _, ok := selected.Accounts[accountID]; !ok {
			return Config{}, fmt.Errorf("unknown account %q", accountID)
		}
		connected[accountID] = true
	}
	if len(connected) == 0 {
		return selected, nil
	}

	for _, client := range AdmittedClientIDs() {
		if selected.routeUsesConnectedAccount(client, connected) {
			continue
		}
		profileID := selected.profileForConnectedClient(client, connected)
		if profileID == "" {
			delete(selected.Routes, client)
			continue
		}
		selected.Routes[client] = profileID
	}
	return selected, nil
}

func (c Config) routeUsesConnectedAccount(client string, connected map[string]bool) bool {
	runtime, err := c.ResolveRuntime(client, "")
	return err == nil && connected[runtime.AccountID]
}

func (c Config) profileForConnectedClient(client string, connected map[string]bool) string {
	preferredModel := ""
	if runtime, err := c.ResolveRuntime(client, ""); err == nil {
		preferredModel = runtime.Model
	}
	replacement := ""
	for _, profileID := range c.ProfileIDs() {
		profile := c.Profiles[profileID]
		if !connected[profile.Account] || profile.Client != client {
			continue
		}
		account := c.Accounts[profile.Account]
		account.ID = profile.Account
		if _, err := account.EndpointFor(client); err != nil {
			continue
		}
		if profile.Model != "" {
			if preferredModel != "" && profile.Model == preferredModel {
				return profileID
			}
			if replacement == "" {
				replacement = profileID
			}
		}
	}
	return replacement
}

func ValidProfileName(name string) bool { return profileNamePattern.MatchString(name) }

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
	if c.Adapters == nil {
		c.Adapters = map[string]AdapterConfig{}
	}
}

func (c Config) normalizedCopy() Config {
	out := Config{
		Version:  c.Version,
		Accounts: map[string]Account{},
		Profiles: map[string]Profile{},
		Routes:   Routes{},
		Adapters: map[string]AdapterConfig{},
	}
	for name, account := range c.Accounts {
		out.Accounts[name] = account
	}
	for name, profile := range c.Profiles {
		out.Profiles[name] = profile
	}
	for name, route := range c.Routes {
		out.Routes[name] = route
	}
	for name, adapter := range c.Adapters {
		adapter.Targets = append([]string(nil), adapter.Targets...)
		out.Adapters[name] = adapter
	}
	out.Normalize()
	return out
}

func (c Config) Validate() error {
	c = c.normalizedCopy()
	if c.Version != ConfigVersion {
		return &UnsupportedConfigVersionError{Version: c.Version, ExpectedVersion: ConfigVersion}
	}
	if len(c.Profiles) == 0 {
		return errors.New("at least one profile is required")
	}
	if len(c.Accounts) == 0 {
		return errors.New("at least one account is required")
	}
	for name, profile := range c.Profiles {
		if !ValidProfileName(name) {
			return fmt.Errorf("invalid profile name %q; use letters, numbers, dot, dash, or underscore", name)
		}
		if strings.TrimSpace(profile.Label) == "" {
			return fmt.Errorf("profile %q has an empty label", name)
		}
	}
	for name, account := range c.Accounts {
		if !ValidProfileName(name) {
			return fmt.Errorf("invalid account name %q; use letters, numbers, dot, dash, or underscore", name)
		}
		if name != strings.ToLower(name) {
			return fmt.Errorf("invalid account name %q; environment-backed account IDs must be lowercase", name)
		}
		if strings.TrimSpace(account.Label) == "" {
			return fmt.Errorf("account %q has an empty label", name)
		}
		if account.Endpoints.OpenAIResponses == "" && account.Endpoints.Anthropic == "" {
			return fmt.Errorf("account %q must define at least one endpoint", name)
		}
		if err := validateEndpoints("account", name, account.Endpoints); err != nil {
			return err
		}
		if account.AccountProbe != nil {
			if !ValidProfileName(account.AccountProbe.Kind) {
				return fmt.Errorf("account %q has invalid account probe provider %q", name, account.AccountProbe.Kind)
			}
			if err := validateEndpoint(account.AccountProbe.BaseURL); err != nil {
				return fmt.Errorf("account %q account probe: %w", name, err)
			}
		}
	}
	for name, profile := range c.Profiles {
		if profile.Account == "" {
			return fmt.Errorf("profile %q must reference an account", name)
		}
		if _, ok := c.Accounts[profile.Account]; !ok {
			return fmt.Errorf("profile %q references unknown account %q", name, profile.Account)
		}
		if !IsAdmittedClient(profile.Client) {
			return fmt.Errorf("profile %q has unknown client %q", name, profile.Client)
		}
		if strings.TrimSpace(profile.Model) == "" {
			return fmt.Errorf("profile %q must define a model", name)
		}
		if profile.ModelProvider != "" {
			if !modelProviderPattern.MatchString(profile.ModelProvider) {
				return fmt.Errorf("profile %q has invalid model provider %q", name, profile.ModelProvider)
			}
			if profile.Client != ClientCodex {
				return fmt.Errorf("profile %q model_provider is only supported for codex-scoped profiles", name)
			}
		}
	}
	for client, profile := range c.Routes {
		if !IsAdmittedClient(client) {
			return fmt.Errorf("unknown route %q; supported routes are %s", client, AdmittedClientUsage())
		}
		selected, ok := c.Profiles[profile]
		if !ok {
			return fmt.Errorf("route %q references unknown profile %q", client, profile)
		}
		if selected.Client != client {
			return fmt.Errorf("route %q selects profile %q for %q", client, profile, selected.Client)
		}
	}
	for name := range c.Adapters {
		if !IsAdmittedClient(name) {
			return fmt.Errorf("unknown adapter %q", name)
		}
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

func validateEndpoints(owner, name string, endpoints Endpoints) error {
	for protocol, raw := range map[string]string{"openai_responses": endpoints.OpenAIResponses, "anthropic": endpoints.Anthropic} {
		if raw == "" {
			continue
		}
		if err := validateEndpoint(raw); err != nil {
			return fmt.Errorf("%s %q endpoint %s: %w", owner, name, protocol, err)
		}
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
	if u.Scheme == "http" && u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" && u.Hostname() != "::1" {
		return errors.New("plain HTTP is allowed only for a loopback endpoint")
	}
	for key := range u.Query() {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "key") || strings.Contains(lower, "auth") || strings.Contains(lower, "password") {
			return fmt.Errorf("credential-like query parameter %q is forbidden", key)
		}
	}
	return nil
}

func (c Config) ResolveRuntime(client, explicitProfile string) (Runtime, error) {
	c = c.normalizedCopy()
	name := explicitProfile
	if name == "" {
		name = c.Routes[client]
		if name == "" {
			return Runtime{}, fmt.Errorf("no route selected for client %q", client)
		}
	}
	profile, ok := c.Profiles[name]
	if !ok {
		return Runtime{}, fmt.Errorf("unknown profile %q", name)
	}
	profile.ID = name
	if profile.Client != client {
		return Runtime{}, &RuntimeProfileClientMismatchError{ProfileID: name, ExpectedClient: profile.Client, ActualClient: client}
	}
	account, ok := c.Accounts[profile.Account]
	if !ok {
		return Runtime{}, &RuntimeProfileUnknownAccountError{ProfileID: name, AccountID: profile.Account}
	}
	account.ID = profile.Account
	endpoint, err := account.EndpointFor(client)
	if err != nil {
		return Runtime{}, err
	}
	return Runtime{
		ProfileID:     name,
		ProfileLabel:  profile.Label,
		AccountID:     account.ID,
		AccountLabel:  account.Label,
		Client:        client,
		Endpoint:      endpoint,
		Model:         profile.Model,
		ModelProvider: resolvedModelProvider(client, profile),
	}, nil
}

func (a Account) EndpointFor(client string) (string, error) {
	spec, ok := ClientSpecFor(client)
	if !ok {
		return "", fmt.Errorf("unknown client %q", client)
	}
	return spec.Endpoint(a)
}
