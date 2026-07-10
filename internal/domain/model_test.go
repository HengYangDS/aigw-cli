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

func TestResolveRuntimeReturnsAccountEndpointAndModel(t *testing.T) {
	cfg := domain.NewConfig()
	cfg.Accounts["dmx"] = domain.Account{Label: "DMXAPI", Endpoints: domain.Endpoints{OpenAIResponses: "https://dmx.test/v1", Anthropic: "https://dmx.test"}}
	cfg.Profiles["gpt-5.6"] = domain.Profile{Label: "GPT-5.6", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{Codex: "gpt-5.6"}}
	cfg.Profiles["claude-opus"] = domain.Profile{Label: "Claude Opus", Account: "dmx", Client: domain.ClientClaude, Models: domain.Models{Claude: "claude-opus"}}
	cfg.Routes.Default = "gpt-5.6"
	cfg.Routes.Overrides[domain.ClientClaude] = "claude-opus"
	got, inherited, err := cfg.ResolveRuntime(domain.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if !inherited || got.ProfileID != "gpt-5.6" || got.AccountID != "dmx" || got.Model != "gpt-5.6" || got.Endpoint != "https://dmx.test/v1" {
		t.Fatalf("runtime = %#v inherited=%v", got, inherited)
	}
	got, inherited, err = cfg.ResolveRuntime(domain.ClientClaude, "")
	if err != nil {
		t.Fatal(err)
	}
	if inherited || got.ProfileID != "claude-opus" || got.Model != "claude-opus" || got.Endpoint != "https://dmx.test" {
		t.Fatalf("runtime = %#v inherited=%v", got, inherited)
	}
}

func TestValidateRejectsRuntimeProfileReferencingUnknownAccountOrWrongClient(t *testing.T) {
	cfg := domain.NewConfig()
	cfg.Accounts["dmx"] = domain.Account{Label: "DMXAPI", Endpoints: domain.Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Profiles["codex"] = domain.Profile{Label: "Codex", Account: "missing", Client: domain.ClientCodex, Models: domain.Models{Codex: "gpt-5.6"}}
	cfg.Routes.Default = "codex"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "unknown account") {
		t.Fatalf("error = %v", err)
	}
	cfg.Profiles["codex"] = domain.Profile{Label: "Codex", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{Claude: "claude-opus"}}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "codex model") {
		t.Fatalf("error = %v", err)
	}
}

func TestNormalizePromotesLegacyEndpointProfileToAccountBackedRuntimeProfile(t *testing.T) {
	cfg := domain.Config{Version: 1, Profiles: map[string]domain.Profile{"dmx": {Label: "DMXAPI", Endpoints: domain.Endpoints{Anthropic: "https://dmx.test"}}}, Routes: domain.Routes{Default: "dmx"}}
	cfg.Normalize()
	if cfg.Accounts["dmx"].Endpoints.Anthropic != "https://dmx.test" || cfg.Profiles["dmx"].Account != "dmx" {
		t.Fatalf("normalized config = %#v", cfg)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}
