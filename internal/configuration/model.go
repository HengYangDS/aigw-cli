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
	ConfigVersion                 = 2
	ClientClaude                  = "claude"
	ClientCodex                   = "codex"
	CodexResponsesStorageRequired = "required"
)

var profileNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

type Config struct {
	Version  int                      `toml:"version" json:"version"`
	Accounts map[string]Account       `toml:"accounts,omitempty" json:"accounts,omitempty"`
	Profiles map[string]Profile       `toml:"profiles" json:"profiles"`
	Routes   Routes                   `toml:"routes" json:"routes"`
	Adapters map[string]AdapterConfig `toml:"adapters,omitempty" json:"adapters,omitempty"`
}

type Account struct {
	ID                    string        `toml:"-" json:"id,omitempty"`
	Label                 string        `toml:"label" json:"label"`
	CodexResponsesStorage string        `toml:"codex_responses_storage,omitempty" json:"codex_responses_storage,omitempty"`
	Endpoints             Endpoints     `toml:"endpoints" json:"endpoints"`
	AccountProbe          *AccountProbe `toml:"account_probe,omitempty" json:"account_probe,omitempty"`
}

type Profile struct {
	ID      string `toml:"-" json:"id,omitempty"`
	Label   string `toml:"label" json:"label"`
	Purpose string `toml:"purpose,omitempty" json:"purpose,omitempty"`
	Account string `toml:"account" json:"account"`
	Client  string `toml:"client,omitempty" json:"client,omitempty"`
	Models  Models `toml:"models,omitempty" json:"models,omitempty"`
}

// Models maps an admitted client ID to the upstream model ID it should use.
// The TOML shape remains [profiles.<id>.models] with client IDs as keys.
type Models map[string]string

type Runtime struct {
	ProfileID             string `json:"profile_id"`
	ProfileLabel          string `json:"profile_label"`
	AccountID             string `json:"account_id"`
	AccountLabel          string `json:"account_label"`
	Client                string `json:"client"`
	Endpoint              string `json:"endpoint"`
	Model                 string `json:"model,omitempty"`
	CodexResponsesStorage string `json:"codex_responses_storage,omitempty"`
}

type AccountProbe struct {
	Kind    string `toml:"kind" json:"kind"`
	BaseURL string `toml:"base_url" json:"base_url"`
}

type Endpoints struct {
	OpenAIResponses string `toml:"openai_responses,omitempty" json:"openai_responses,omitempty"`
	Anthropic       string `toml:"anthropic,omitempty" json:"anthropic,omitempty"`
}

type Routes struct {
	Default   string            `toml:"default" json:"default"`
	Overrides map[string]string `toml:"overrides,omitempty" json:"overrides,omitempty"`
}

type AdapterConfig struct {
	Enabled    bool     `toml:"enabled" json:"enabled"`
	Executable string   `toml:"executable,omitempty" json:"executable,omitempty"`
	Targets    []string `toml:"targets,omitempty" json:"targets,omitempty"`
}

func NewConfig() Config {
	return Config{Version: ConfigVersion, Accounts: map[string]Account{}, Profiles: map[string]Profile{}, Routes: Routes{Overrides: map[string]string{}}, Adapters: map[string]AdapterConfig{}}
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
		if profile.ModelFor(client) != "" {
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
	runtime, _, err := c.ResolveRuntime(client, "")
	return err == nil && runtime.AccountID != "" && runtime.AccountID == accountID
}

func ValidProfileName(name string) bool { return profileNamePattern.MatchString(name) }

func (c *Config) Normalize() {
	if c.Accounts == nil {
		c.Accounts = map[string]Account{}
	}
	if c.Profiles == nil {
		c.Profiles = map[string]Profile{}
	}
	if c.Routes.Overrides == nil {
		c.Routes.Overrides = map[string]string{}
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
		Routes: Routes{
			Default:   c.Routes.Default,
			Overrides: map[string]string{},
		},
		Adapters: map[string]AdapterConfig{},
	}
	for name, account := range c.Accounts {
		out.Accounts[name] = account
	}
	for name, profile := range c.Profiles {
		out.Profiles[name] = profile
	}
	for name, route := range c.Routes.Overrides {
		out.Routes.Overrides[name] = route
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
		if strings.TrimSpace(account.Label) == "" {
			return fmt.Errorf("account %q has an empty label", name)
		}
		if account.Endpoints.OpenAIResponses == "" && account.Endpoints.Anthropic == "" {
			return fmt.Errorf("account %q must define at least one endpoint", name)
		}
		if account.CodexResponsesStorage != "" && account.CodexResponsesStorage != CodexResponsesStorageRequired {
			return fmt.Errorf("account %q codex responses storage must be %q when set", name, CodexResponsesStorageRequired)
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
		if profile.Client != "" && !IsAdmittedClient(profile.Client) {
			return fmt.Errorf("profile %q has unknown client %q", name, profile.Client)
		}
		for client, model := range profile.Models {
			if !IsAdmittedClient(client) {
				return fmt.Errorf("profile %q defines a model for unadmitted client %q", name, client)
			}
			if profile.Client != "" && client != profile.Client && strings.TrimSpace(model) != "" {
				return fmt.Errorf("profile %q is %s-scoped; define a model for %s, not %s", name, profile.Client, profile.Client, client)
			}
		}
	}
	if _, ok := c.Profiles[c.Routes.Default]; !ok {
		return fmt.Errorf("default route references unknown profile %q", c.Routes.Default)
	}
	for client, profile := range c.Routes.Overrides {
		if !IsAdmittedClient(client) {
			return fmt.Errorf("unknown route %q; supported routes are claude and codex", client)
		}
		if _, ok := c.Profiles[profile]; !ok {
			return fmt.Errorf("route %q references unknown profile %q", client, profile)
		}
	}
	for name := range c.Adapters {
		if !IsAdmittedClient(name) {
			return fmt.Errorf("unknown adapter %q", name)
		}
	}
	return nil
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

func (c Config) ResolveRuntime(client, explicitProfile string) (Runtime, bool, error) {
	c = c.normalizedCopy()
	name := explicitProfile
	inherited := false
	if name == "" {
		var overridden bool
		name, overridden = c.Routes.Overrides[client]
		if !overridden {
			name = c.Routes.Default
			inherited = true
		}
	}
	profile, ok := c.Profiles[name]
	if !ok {
		return Runtime{}, inherited, fmt.Errorf("unknown profile %q", name)
	}
	profile.ID = name
	if profile.Client != "" && client != "" && profile.Client != client {
		return Runtime{}, inherited, &RuntimeProfileClientMismatchError{ProfileID: name, ExpectedClient: profile.Client, ActualClient: client}
	}
	account, ok := c.Accounts[profile.Account]
	if !ok {
		return Runtime{}, inherited, &RuntimeProfileUnknownAccountError{ProfileID: name, AccountID: profile.Account}
	}
	account.ID = profile.Account
	endpoint, err := account.EndpointFor(client)
	if err != nil {
		return Runtime{}, inherited, err
	}
	return Runtime{
		ProfileID:             name,
		ProfileLabel:          profile.Label,
		AccountID:             account.ID,
		AccountLabel:          account.Label,
		Client:                client,
		Endpoint:              endpoint,
		Model:                 profile.ModelFor(client),
		CodexResponsesStorage: account.CodexResponsesStorage,
	}, inherited, nil
}

func (p Profile) ModelFor(client string) string {
	return p.Models[client]
}

func (a Account) EndpointFor(client string) (string, error) {
	spec, ok := ClientSpecFor(client)
	if !ok {
		return "", fmt.Errorf("unknown client %q", client)
	}
	switch spec.EndpointProtocol {
	case ProtocolAnthropic:
		if a.Endpoints.Anthropic == "" {
			return "", &RuntimeMissingEndpointError{AccountID: a.ID, Protocol: spec.EndpointProtocol}
		}
		return strings.TrimRight(a.Endpoints.Anthropic, "/"), nil
	case ProtocolOpenAIResponses:
		if a.Endpoints.OpenAIResponses == "" {
			return "", &RuntimeMissingEndpointError{AccountID: a.ID, Protocol: spec.EndpointProtocol}
		}
		return strings.TrimRight(a.Endpoints.OpenAIResponses, "/"), nil
	default:
		return "", fmt.Errorf("client %q has unsupported endpoint protocol %q", client, spec.EndpointProtocol)
	}
}
