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

func TestNormalizeFillsEveryNilCollection(t *testing.T) {
	cfg := domain.Config{}
	cfg.Normalize()
	if cfg.Accounts == nil || cfg.Profiles == nil || cfg.Routes.Overrides == nil || cfg.Adapters == nil {
		t.Fatalf("normalized config still has nil collections: %#v", cfg)
	}
}

func TestNormalizedCopyDeepCopiesAdapterTargets(t *testing.T) {
	cfg := validConfig()
	cfg.Adapters = map[string]domain.AdapterConfig{
		domain.ClientCodex: {Enabled: true, Targets: []string{"a", "b"}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("adapters keyed by admitted client must validate: %v", err)
	}
}

func TestValidateRequiresAtLeastOneProfileAndAccount(t *testing.T) {
	cfg := validConfig()
	cfg.Profiles = map[string]domain.Profile{}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "at least one profile") {
		t.Fatalf("empty profiles error = %v", err)
	}

	cfg = validConfig()
	cfg.Accounts = map[string]domain.Account{}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "at least one account") {
		t.Fatalf("empty accounts error = %v", err)
	}
}

func TestValidateRejectsEmptyLabelsAndUnnamedOrUnendpointedAccounts(t *testing.T) {
	tests := []struct {
		name string
		edit func(*domain.Config)
		want string
	}{
		{"empty profile label", func(c *domain.Config) {
			p := c.Profiles["dmx"]
			p.Label = "  "
			c.Profiles["dmx"] = p
		}, "empty label"},
		{"invalid account name", func(c *domain.Config) {
			c.Accounts["bad name"] = c.Accounts["dmx"]
		}, "invalid account name"},
		{"empty account label", func(c *domain.Config) {
			a := c.Accounts["dmx"]
			a.Label = " "
			c.Accounts["dmx"] = a
		}, "empty label"},
		{"account without endpoint", func(c *domain.Config) {
			c.Accounts["dmx"] = domain.Account{Label: "DMXAPI"}
		}, "must define at least one endpoint"},
		{"account probe invalid kind", func(c *domain.Config) {
			a := c.Accounts["dmx"]
			a.AccountProbe = &domain.AccountProbe{Kind: "bad kind", BaseURL: "https://diagnostics.example.test"}
			c.Accounts["dmx"] = a
		}, "invalid account probe provider"},
		{"account probe invalid endpoint", func(c *domain.Config) {
			a := c.Accounts["dmx"]
			a.AccountProbe = &domain.AccountProbe{Kind: "future", BaseURL: "not-a-url"}
			c.Accounts["dmx"] = a
		}, "account probe"},
		{"profile missing account", func(c *domain.Config) {
			p := c.Profiles["dmx"]
			p.Account = ""
			c.Profiles["dmx"] = p
		}, "must reference an account"},
		{"profile unknown client", func(c *domain.Config) {
			p := c.Profiles["dmx"]
			p.Client = "gemini"
			c.Profiles["dmx"] = p
		}, "unknown client"},
		{"override references unknown profile", func(c *domain.Config) {
			c.Routes.Overrides[domain.ClientClaude] = "missing"
		}, "references unknown profile"},
		{"unknown adapter", func(c *domain.Config) {
			c.Adapters = map[string]domain.AdapterConfig{"gemini": {Enabled: true}}
		}, "unknown adapter"},
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

func TestValidateEndpointRejectsMalformedURL(t *testing.T) {
	cfg := validConfig()
	a := cfg.Accounts["dmx"]
	a.Endpoints.Anthropic = "://not-a-valid-url"
	cfg.Accounts["dmx"] = a
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "must use http or https") {
		t.Fatalf("malformed URL error = %v", err)
	}
}

func TestResolveRuntimeRejectsUnknownProfileAccountAndEndpoint(t *testing.T) {
	cfg := domain.NewConfig()
	cfg.Accounts["dmx"] = domain.Account{Label: "DMXAPI", Endpoints: domain.Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Profiles["codex"] = domain.Profile{Label: "Codex", Account: "dmx"}
	cfg.Routes.Default = "codex"

	if _, _, err := cfg.ResolveRuntime(domain.ClientCodex, "missing-profile"); err == nil || !strings.Contains(err.Error(), "unknown profile") {
		t.Fatalf("unknown profile error = %v", err)
	}

	orphan := cfg
	orphan.Profiles = map[string]domain.Profile{"codex": {Label: "Codex", Account: "missing-account"}}
	orphan.Routes.Default = "codex"
	if _, _, err := orphan.ResolveRuntime(domain.ClientCodex, ""); err == nil || !strings.Contains(err.Error(), "unknown account") {
		t.Fatalf("unknown account error = %v", err)
	}

	if _, _, err := cfg.ResolveRuntime(domain.ClientClaude, "codex"); err == nil || !strings.Contains(err.Error(), "no Anthropic endpoint") {
		t.Fatalf("missing endpoint error = %v", err)
	}
}

func TestEndpointForRejectsUnknownClientAndMissingProtocolEndpoint(t *testing.T) {
	account := domain.Account{ID: "dmx", Label: "DMXAPI", Endpoints: domain.Endpoints{Anthropic: "https://dmx.test"}}
	if _, err := account.EndpointFor("gemini"); err == nil || !strings.Contains(err.Error(), "unknown client") {
		t.Fatalf("unknown client error = %v", err)
	}
	if _, err := account.EndpointFor(domain.ClientCodex); err == nil || !strings.Contains(err.Error(), "no OpenAI Responses endpoint") {
		t.Fatalf("missing OpenAI endpoint error = %v", err)
	}
	if endpoint, err := account.EndpointFor(domain.ClientClaude); err != nil || endpoint != "https://dmx.test" {
		t.Fatalf("EndpointFor = %q, %v", endpoint, err)
	}
}
