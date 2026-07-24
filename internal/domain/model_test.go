package domain_test

import (
	"reflect"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

func TestRuntimeProfileDoesNotOwnEndpointsOrAccountProbe(t *testing.T) {
	profileType := reflect.TypeOf(domain.Profile{})
	for _, field := range []string{"Endpoints", "AccountProbe"} {
		if _, found := profileType.FieldByName(field); found {
			t.Fatalf("runtime Profile must not own %s; Account is the only endpoint and probe owner", field)
		}
	}
}

func TestProfileResolutionSurfaceIsAbsent(t *testing.T) {
	if _, found := reflect.TypeOf(domain.Config{}).MethodByName("Resolve"); found {
		t.Fatal("Config.Resolve reintroduced Profile-based resolution; use ResolveRuntime")
	}
	if _, found := reflect.TypeOf(domain.Profile{}).MethodByName("EndpointFor"); found {
		t.Fatal("Profile.EndpointFor reintroduced endpoint ownership outside Account")
	}
}

func validConfig() domain.Config {
	return domain.Config{
		Version: domain.ConfigVersion,
		Accounts: map[string]domain.Account{
			"dmx":    {Label: "DMXAPI", Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1", Anthropic: "https://example.test"}},
			"backup": {Label: "Backup", Endpoints: domain.Endpoints{OpenAIResponses: "https://backup.test/v1"}},
		},
		Profiles: map[string]domain.Profile{
			"dmx":    {Label: "DMXAPI", Account: "dmx"},
			"backup": {Label: "Backup", Account: "backup"},
		},
		Routes: domain.Routes{Default: "dmx", Overrides: map[string]string{}},
	}
}

func TestValidateRejectsNonCanonicalSchemaVersion(t *testing.T) {
	cfg := validConfig()
	cfg.Version = 1
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "unsupported config version 1") {
		t.Fatalf("version 1 validation error = %v", err)
	}
}

func TestNewConfigUsesCurrentSchema(t *testing.T) {
	if got := domain.NewConfig().Version; got != domain.ConfigVersion {
		t.Fatalf("new config version = %d, want %d", got, domain.ConfigVersion)
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
			a := c.Accounts["dmx"]
			a.Endpoints.Anthropic = "https://user:secret@example.test"
			c.Accounts["dmx"] = a
		}, "userinfo"},
		{"url secret query", func(c *domain.Config) {
			a := c.Accounts["dmx"]
			a.Endpoints.Anthropic = "https://example.test?api_key=secret"
			c.Accounts["dmx"] = a
		}, "credential-like"},
		{"remote plain http", func(c *domain.Config) {
			a := c.Accounts["dmx"]
			a.Endpoints.Anthropic = "http://example.test"
			c.Accounts["dmx"] = a
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

func TestValidateAllowsExplicitLoopbackDevelopmentAccount(t *testing.T) {
	cfg := validConfig()
	a := cfg.Accounts["dmx"]
	a.Endpoints.OpenAIResponses = "http://127.0.0.1:18765/v1"
	cfg.Accounts["dmx"] = a
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateAllowsProviderNeutralExplicitDiagnostics(t *testing.T) {
	cfg := validConfig()
	account := cfg.Accounts["dmx"]
	account.AccountProbe = &domain.AccountProbe{Kind: "future-provider", BaseURL: "https://diagnostics.example.test"}
	cfg.Accounts["dmx"] = account
	if err := cfg.Validate(); err != nil {
		t.Fatalf("explicit provider diagnostics must remain configuration-valid even when this build has no driver: %v", err)
	}
}

func TestResolveRuntimeReturnsAccountEndpointAndModel(t *testing.T) {
	cfg := domain.NewConfig()
	cfg.Accounts["dmx"] = domain.Account{Label: "DMXAPI", Endpoints: domain.Endpoints{OpenAIResponses: "https://dmx.test/v1", Anthropic: "https://dmx.test"}}
	cfg.Profiles["gpt-5.6"] = domain.Profile{Label: "GPT-5.6", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-5.6"}}
	cfg.Profiles["claude-opus"] = domain.Profile{Label: "Claude Opus", Account: "dmx", Client: domain.ClientClaude, Models: domain.Models{domain.ClientClaude: "claude-opus"}}
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
	cfg.Profiles["codex"] = domain.Profile{Label: "Codex", Account: "missing", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-5.6"}}
	cfg.Routes.Default = "codex"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "unknown account") {
		t.Fatalf("error = %v", err)
	}
	cfg.Profiles["codex"] = domain.Profile{Label: "Codex", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{domain.ClientClaude: "claude-opus"}}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "codex-scoped") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateRejectsUnadmittedOrCrossScopedModelKeys(t *testing.T) {
	cfg := validConfig()
	profile := cfg.Profiles["dmx"]
	profile.Client = domain.ClientClaude
	profile.Models = domain.Models{"gemini": "gemini-next"}
	cfg.Profiles["dmx"] = profile
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "unadmitted client") {
		t.Fatalf("unadmitted model key error = %v", err)
	}

	profile.Models = domain.Models{domain.ClientCodex: "gpt-test"}
	cfg.Profiles["dmx"] = profile
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "claude-scoped") {
		t.Fatalf("cross-scoped model key error = %v", err)
	}
}

func TestValidateTreatsProfileIDAsTransparentConfiguration(t *testing.T) {
	const profileID = "gpt-5.6-terra-cdx"
	cfg := validConfig()
	profile := cfg.Profiles["dmx"]
	profile.Client = domain.ClientCodex
	profile.Models = domain.Models{domain.ClientCodex: "upstream-model"}
	delete(cfg.Profiles, "dmx")
	cfg.Profiles[profileID] = profile
	cfg.Routes.Default = profileID

	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid profile ID must not be rejected by product-specific naming policy: %v", err)
	}
}

func TestValidateTreatsUpstreamModelIDAsTransparentConfiguration(t *testing.T) {
	const modelID = "gpt-5.6-terra-cdx"
	cfg := validConfig()
	profile := cfg.Profiles["dmx"]
	profile.Client = domain.ClientCodex
	profile.Models = domain.Models{domain.ClientCodex: modelID}
	cfg.Profiles["dmx"] = profile

	if err := cfg.Validate(); err != nil {
		t.Fatalf("upstream model ID must not be rejected by product-specific naming policy: %v", err)
	}
}
