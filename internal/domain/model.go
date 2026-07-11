package domain

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const (
	ClientClaude = "claude"
	ClientCodex  = "codex"
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
	ID           string        `toml:"-" json:"id,omitempty"`
	Label        string        `toml:"label" json:"label"`
	Endpoints    Endpoints     `toml:"endpoints" json:"endpoints"`
	AccountProbe *AccountProbe `toml:"account_probe,omitempty" json:"account_probe,omitempty"`
}

type Profile struct {
	ID      string `toml:"-" json:"id,omitempty"`
	Label   string `toml:"label" json:"label"`
	Account string `toml:"account" json:"account"`
	Client  string `toml:"client,omitempty" json:"client,omitempty"`
	Models  Models `toml:"models,omitempty" json:"models,omitempty"`
}

type Models struct {
	Claude string `toml:"claude,omitempty" json:"claude,omitempty"`
	Codex  string `toml:"codex,omitempty" json:"codex,omitempty"`
}

type Runtime struct {
	ProfileID    string `json:"profile_id"`
	ProfileLabel string `json:"profile_label"`
	AccountID    string `json:"account_id"`
	AccountLabel string `json:"account_label"`
	Client       string `json:"client"`
	Endpoint     string `json:"endpoint"`
	Model        string `json:"model,omitempty"`
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
	return Config{Version: 1, Accounts: map[string]Account{}, Profiles: map[string]Profile{}, Routes: Routes{Overrides: map[string]string{}}, Adapters: map[string]AdapterConfig{}}
}

func ValidProfileName(name string) bool { return profileNamePattern.MatchString(name) }

func (c *Config) Normalize() {
	if c.Version == 0 {
		c.Version = 1
	}
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
	if c.Version != 1 {
		return fmt.Errorf("unsupported config version %d", c.Version)
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
		if err := validateEndpoints("account", name, account.Endpoints); err != nil {
			return err
		}
		if account.AccountProbe != nil {
			if account.AccountProbe.Kind != "dmxapi" {
				return fmt.Errorf("account %q has unsupported account probe %q", name, account.AccountProbe.Kind)
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
		if profile.Client != "" && profile.Client != ClientClaude && profile.Client != ClientCodex {
			return fmt.Errorf("profile %q has unknown client %q", name, profile.Client)
		}
		if profile.Client == ClientCodex && profile.Models.Claude != "" {
			return fmt.Errorf("profile %q is codex-scoped; define a codex model, not a claude model", name)
		}
		if profile.Client == ClientClaude && profile.Models.Codex != "" {
			return fmt.Errorf("profile %q is claude-scoped; define a claude model, not a codex model", name)
		}
	}
	if _, ok := c.Profiles[c.Routes.Default]; !ok {
		return fmt.Errorf("default route references unknown profile %q", c.Routes.Default)
	}
	for client, profile := range c.Routes.Overrides {
		if client != ClientClaude && client != ClientCodex {
			return fmt.Errorf("unknown route %q; supported routes are claude and codex", client)
		}
		if _, ok := c.Profiles[profile]; !ok {
			return fmt.Errorf("route %q references unknown profile %q", client, profile)
		}
	}
	for name := range c.Adapters {
		if name != ClientClaude && name != ClientCodex {
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
		return Runtime{}, inherited, fmt.Errorf("profile %q is for %s, not %s", name, profile.Client, client)
	}
	account, ok := c.Accounts[profile.Account]
	if !ok {
		return Runtime{}, inherited, fmt.Errorf("profile %q references unknown account %q", name, profile.Account)
	}
	account.ID = profile.Account
	endpoint, err := account.EndpointFor(client)
	if err != nil {
		return Runtime{}, inherited, err
	}
	return Runtime{ProfileID: name, ProfileLabel: profile.Label, AccountID: account.ID, AccountLabel: account.Label, Client: client, Endpoint: endpoint, Model: profile.ModelFor(client)}, inherited, nil
}

func (p Profile) ModelFor(client string) string {
	switch client {
	case ClientClaude:
		return p.Models.Claude
	case ClientCodex:
		return p.Models.Codex
	default:
		return ""
	}
}

func (a Account) EndpointFor(client string) (string, error) {
	switch client {
	case ClientClaude:
		if a.Endpoints.Anthropic == "" {
			return "", fmt.Errorf("account %q has no Anthropic endpoint", a.ID)
		}
		return strings.TrimRight(a.Endpoints.Anthropic, "/"), nil
	case ClientCodex:
		if a.Endpoints.OpenAIResponses == "" {
			return "", fmt.Errorf("account %q has no OpenAI Responses endpoint", a.ID)
		}
		return strings.TrimRight(a.Endpoints.OpenAIResponses, "/"), nil
	default:
		return "", fmt.Errorf("unknown client %q", client)
	}
}
