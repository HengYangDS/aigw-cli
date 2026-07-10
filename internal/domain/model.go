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
	Profiles map[string]Profile       `toml:"profiles" json:"profiles"`
	Routes   Routes                   `toml:"routes" json:"routes"`
	Adapters map[string]AdapterConfig `toml:"adapters,omitempty" json:"adapters,omitempty"`
}

type Profile struct {
	ID        string    `toml:"-" json:"id,omitempty"`
	Label     string    `toml:"label" json:"label"`
	Endpoints Endpoints `toml:"endpoints" json:"endpoints"`
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
	return Config{
		Version:  1,
		Profiles: map[string]Profile{},
		Routes:   Routes{Overrides: map[string]string{}},
		Adapters: map[string]AdapterConfig{},
	}
}

func ValidProfileName(name string) bool { return profileNamePattern.MatchString(name) }

func (c *Config) Normalize() {
	if c.Version == 0 {
		c.Version = 1
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

func (c Config) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("unsupported config version %d", c.Version)
	}
	if len(c.Profiles) == 0 {
		return errors.New("at least one profile is required")
	}
	for name, profile := range c.Profiles {
		if !ValidProfileName(name) {
			return fmt.Errorf("invalid profile name %q; use letters, numbers, dot, dash, or underscore", name)
		}
		if strings.TrimSpace(profile.Label) == "" {
			return fmt.Errorf("profile %q has an empty label", name)
		}
		if profile.Endpoints.OpenAIResponses == "" && profile.Endpoints.Anthropic == "" {
			return fmt.Errorf("profile %q must define at least one endpoint", name)
		}
		for protocol, raw := range map[string]string{
			"openai_responses": profile.Endpoints.OpenAIResponses,
			"anthropic":        profile.Endpoints.Anthropic,
		} {
			if raw == "" {
				continue
			}
			if err := validateEndpoint(raw); err != nil {
				return fmt.Errorf("profile %q endpoint %s: %w", name, protocol, err)
			}
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
		if strings.Contains(lower, "token") || strings.Contains(lower, "secret") ||
			strings.Contains(lower, "key") || strings.Contains(lower, "auth") ||
			strings.Contains(lower, "password") {
			return fmt.Errorf("credential-like query parameter %q is forbidden", key)
		}
	}
	return nil
}

func (c Config) Resolve(client, explicitProfile string) (Profile, bool, error) {
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
		return Profile{}, inherited, fmt.Errorf("unknown profile %q", name)
	}
	profile.ID = name
	return profile, inherited, nil
}

func (p Profile) EndpointFor(client string) (string, error) {
	switch client {
	case ClientClaude:
		if p.Endpoints.Anthropic == "" {
			return "", fmt.Errorf("profile %q has no Anthropic endpoint", p.ID)
		}
		return strings.TrimRight(p.Endpoints.Anthropic, "/"), nil
	case ClientCodex:
		if p.Endpoints.OpenAIResponses == "" {
			return "", fmt.Errorf("profile %q has no OpenAI Responses endpoint", p.ID)
		}
		return strings.TrimRight(p.Endpoints.OpenAIResponses, "/"), nil
	default:
		return "", fmt.Errorf("unknown client %q", client)
	}
}
