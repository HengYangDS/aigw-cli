package domain_test

import (
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

func validConfig() domain.Config {
	return domain.Config{
		Version: 1,
		Profiles: map[string]domain.Profile{
			"dmx": {Label: "DMXAPI", Endpoints: domain.Endpoints{
				OpenAIResponses: "https://example.test/v1",
				Anthropic:       "https://example.test",
			}},
			"backup": {Label: "Backup", Endpoints: domain.Endpoints{
				OpenAIResponses: "https://backup.test/v1",
			}},
		},
		Routes: domain.Routes{Default: "dmx", Overrides: map[string]string{}},
	}
}

func TestResolveInheritsDefaultAndHonorsOverride(t *testing.T) {
	cfg := validConfig()
	p, inherited, err := cfg.Resolve("claude", "")
	if err != nil || p.ID != "dmx" || !inherited {
		t.Fatalf("default resolution = %#v, %v, %v", p, inherited, err)
	}
	cfg.Routes.Overrides[domain.ClientClaude] = "backup"
	p, inherited, err = cfg.Resolve("claude", "")
	if err != nil || p.ID != "backup" || inherited {
		t.Fatalf("override resolution = %#v, %v, %v", p, inherited, err)
	}
	p, _, err = cfg.Resolve("claude", "dmx")
	if err != nil || p.ID != "dmx" {
		t.Fatalf("explicit resolution = %#v, %v", p, err)
	}
}

func TestValidateRejectsUnsafeOrAmbiguousConfiguration(t *testing.T) {
	tests := []struct {
		name string
		edit func(*domain.Config)
		want string
	}{
		{"invalid profile name", func(c *domain.Config) { c.Profiles["bad name"] = c.Profiles["dmx"] }, "profile name"},
		{"unknown default", func(c *domain.Config) { c.Routes.Default = "missing" }, "unknown profile"},
		{"unknown client", func(c *domain.Config) { c.Routes.Overrides["chat"] = "dmx" }, "unknown route"},
		{"url user info", func(c *domain.Config) {
			p := c.Profiles["dmx"]
			p.Endpoints.Anthropic = "https://user:secret@example.test"
			c.Profiles["dmx"] = p
		}, "userinfo"},
		{"url secret query", func(c *domain.Config) {
			p := c.Profiles["dmx"]
			p.Endpoints.Anthropic = "https://example.test?api_key=secret"
			c.Profiles["dmx"] = p
		}, "credential-like"},
		{"remote plain http", func(c *domain.Config) {
			p := c.Profiles["dmx"]
			p.Endpoints.Anthropic = "http://example.test"
			c.Profiles["dmx"] = p
		}, "loopback"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.edit(&cfg)
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestValidateAllowsLoopbackHTTPForLocalCompatibilityTools(t *testing.T) {
	cfg := validConfig()
	p := cfg.Profiles["dmx"]
	p.Endpoints.OpenAIResponses = "http://127.0.0.1:8791/v1"
	cfg.Profiles["dmx"] = p
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}
