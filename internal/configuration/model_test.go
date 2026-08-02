package configuration

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestConfigQueriesOwnAccountAndProfileSelectionSemantics(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["gateway"] = Account{
		Label:     "Gateway",
		Endpoints: Endpoints{Anthropic: "https://gateway.test/anthropic"},
	}
	cfg.Profiles["zeta"] = Profile{Label: "Zeta", Account: "gateway", Client: ClientClaude}
	cfg.Profiles["alpha"] = Profile{Label: "Alpha", Account: "gateway", Models: Models{ClientClaude: "claude-test"}}
	cfg.Routes.Default = "alpha"

	if got := cfg.ProfileIDs(); !reflect.DeepEqual(got, []string{"alpha", "zeta"}) {
		t.Fatalf("ProfileIDs() = %#v", got)
	}
	accountID, providerAccount, err := cfg.ResolveAccount("alpha")
	if err != nil || accountID != "gateway" || providerAccount.ID != "gateway" {
		t.Fatalf("ResolveAccount(profile) = %q, %#v, %v", accountID, providerAccount, err)
	}
	accountID, _, err = cfg.ResolveAccount("gateway")
	if err != nil || accountID != "gateway" {
		t.Fatalf("ResolveAccount(account) = %q, %v", accountID, err)
	}
	if got := cfg.FirstProfileForClient(ClientClaude); got != "alpha" {
		t.Fatalf("FirstProfileForClient() = %q", got)
	}
	if !cfg.RouteUsesAccount(ClientClaude, "gateway") || cfg.RouteUsesAccount(ClientClaude, "other") {
		t.Fatal("RouteUsesAccount() did not follow the resolved route")
	}
	if _, _, err := cfg.ResolveAccount("missing"); err == nil {
		t.Fatal("ResolveAccount() accepted an unknown reference")
	}
}

func TestRuntimeProfileDoesNotOwnEndpointsOrAccountProbe(t *testing.T) {
	profileType := reflect.TypeOf(Profile{})
	for _, field := range []string{"Endpoints", "AccountProbe"} {
		if _, found := profileType.FieldByName(field); found {
			t.Fatalf("runtimeProfile must not own %s; Account is the only endpoint and probe owner", field)
		}
	}
}

func TestProfileResolutionSurfaceIsAbsent(t *testing.T) {
	if _, found := reflect.TypeOf(Config{}).MethodByName("Resolve"); found {
		t.Fatal("Config.Resolve reintroducedProfile-based resolution; use ResolveRuntime")
	}
	if _, found := reflect.TypeOf(Profile{}).MethodByName("EndpointFor"); found {
		t.Fatal("Profile.EndpointFor reintroduced endpoint ownership outsideAccount")
	}
}

func validConfig() Config {
	return Config{
		Version: ConfigVersion,
		Accounts: map[string]Account{
			"dmx":    {Label: "DMXAPI", Endpoints: Endpoints{OpenAIResponses: "https://example.test/v1", Anthropic: "https://example.test"}},
			"backup": {Label: "Backup", Endpoints: Endpoints{OpenAIResponses: "https://backup.test/v1"}},
		},
		Profiles: map[string]Profile{
			"dmx":    {Label: "DMXAPI", Account: "dmx"},
			"backup": {Label: "Backup", Account: "backup"},
		},
		Routes: Routes{Default: "dmx", Overrides: map[string]string{}},
	}
}

func TestValidateRejectsNonCanonicalSchemaVersion(t *testing.T) {
	cfg := validConfig()
	cfg.Version = 1
	err := cfg.Validate()
	var versionErr *UnsupportedConfigVersionError
	if !errors.As(err, &versionErr) {
		t.Fatalf("version 1 validation error = %v", err)
	}
	if versionErr.Version != 1 || versionErr.ExpectedVersion != ConfigVersion {
		t.Fatalf("version error = %#v", versionErr)
	}
	if !strings.Contains(err.Error(), "unsupported config version 1") {
		t.Fatalf("version error text = %q", err)
	}
}

func TestNewConfigUsesCurrentSchema(t *testing.T) {
	if got := NewConfig().Version; got != ConfigVersion {
		t.Fatalf("new config version = %d, want %d", got, ConfigVersion)
	}
}

func TestConfigCloneDoesNotShareMutableState(t *testing.T) {
	original := validConfig()
	original.Adapters = map[string]AdapterConfig{}
	original.Adapters[ClientCodex] = AdapterConfig{Enabled: true, Targets: []string{"one"}}

	clone := original.Clone()
	clone.Accounts["dmx"] = Account{Label: "Changed"}
	clone.Profiles["default"] = Profile{Label: "Changed"}
	clone.Routes.Overrides[ClientClaude] = "changed"
	adapter := clone.Adapters[ClientCodex]
	adapter.Targets[0] = "changed"
	clone.Adapters[ClientCodex] = adapter

	if original.Accounts["dmx"].Label == "Changed" || original.Profiles["default"].Label == "Changed" {
		t.Fatal("clone shares map state with original")
	}
	if original.Routes.Overrides[ClientClaude] == "changed" {
		t.Fatal("clone shares route overrides with original")
	}
	if original.Adapters[ClientCodex].Targets[0] == "changed" {
		t.Fatal("clone shares adapter targets with original")
	}
}

func TestValidateRejectsUnsafeOrAmbiguousConfiguration(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Config)
		want string
	}{
		{"invalid profile name", func(c *Config) { c.Profiles["bad name"] = c.Profiles["dmx"] }, "profile name"},
		{"unknown default", func(c *Config) { c.Routes.Default = "missing" }, "unknown profile"},
		{"unknown client", func(c *Config) { c.Routes.Overrides["chat"] = "dmx" }, "unknown route"},
		{"url user info", func(c *Config) {
			a := c.Accounts["dmx"]
			a.Endpoints.Anthropic = "https://user:secret@example.test"
			c.Accounts["dmx"] = a
		}, "userinfo"},
		{"url secret query", func(c *Config) {
			a := c.Accounts["dmx"]
			a.Endpoints.Anthropic = "https://example.test?api_key=secret"
			c.Accounts["dmx"] = a
		}, "credential-like"},
		{"remote plain http", func(c *Config) {
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
	account.AccountProbe = &AccountProbe{Kind: "future-provider", BaseURL: "https://diagnostics.example.test"}
	cfg.Accounts["dmx"] = account
	if err := cfg.Validate(); err != nil {
		t.Fatalf("explicit provider diagnostics must remain configuration-valid even when this build has no driver: %v", err)
	}
}

func TestResolveRuntimeReturnsAccountEndpointAndModel(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["dmx"] = Account{Label: "DMXAPI", Endpoints: Endpoints{OpenAIResponses: "https://dmx.test/v1", Anthropic: "https://dmx.test"}}
	cfg.Profiles["gpt-5.6"] = Profile{Label: "GPT-5.6", Account: "dmx", Client: ClientCodex, Models: Models{ClientCodex: "gpt-5.6"}}
	cfg.Profiles["claude-opus"] = Profile{Label: "Claude Opus", Account: "dmx", Client: ClientClaude, Models: Models{ClientClaude: "claude-opus"}}
	cfg.Routes.Default = "gpt-5.6"
	cfg.Routes.Overrides[ClientClaude] = "claude-opus"
	got, inherited, err := cfg.ResolveRuntime(ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if !inherited || got.ProfileID != "gpt-5.6" || got.AccountID != "dmx" || got.Model != "gpt-5.6" || got.Endpoint != "https://dmx.test/v1" {
		t.Fatalf("runtime = %#v inherited=%v", got, inherited)
	}
	got, inherited, err = cfg.ResolveRuntime(ClientClaude, "")
	if err != nil {
		t.Fatal(err)
	}
	if inherited || got.ProfileID != "claude-opus" || got.Model != "claude-opus" || got.Endpoint != "https://dmx.test" {
		t.Fatalf("runtime = %#v inherited=%v", got, inherited)
	}
}

func TestValidateRejectsRuntimeProfileReferencingUnknownAccountOrWrongClient(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["dmx"] = Account{Label: "DMXAPI", Endpoints: Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Profiles["codex"] = Profile{Label: "Codex", Account: "missing", Client: ClientCodex, Models: Models{ClientCodex: "gpt-5.6"}}
	cfg.Routes.Default = "codex"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "unknown account") {
		t.Fatalf("error = %v", err)
	}
	cfg.Profiles["codex"] = Profile{Label: "Codex", Account: "dmx", Client: ClientCodex, Models: Models{ClientClaude: "claude-opus"}}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "codex-scoped") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateRejectsUnadmittedOrCrossScopedModelKeys(t *testing.T) {
	cfg := validConfig()
	profile := cfg.Profiles["dmx"]
	profile.Client = ClientClaude
	profile.Models = Models{"gemini": "gemini-next"}
	cfg.Profiles["dmx"] = profile
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "unadmitted client") {
		t.Fatalf("unadmitted model key error = %v", err)
	}

	profile.Models = Models{ClientCodex: "gpt-test"}
	cfg.Profiles["dmx"] = profile
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "claude-scoped") {
		t.Fatalf("cross-scoped model key error = %v", err)
	}
}

func TestValidateTreatsProfileIDAsTransparentConfiguration(t *testing.T) {
	const profileID = "gpt-5.6-terra-cdx"
	cfg := validConfig()
	profile := cfg.Profiles["dmx"]
	profile.Client = ClientCodex
	profile.Models = Models{ClientCodex: "upstream-model"}
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
	profile.Client = ClientCodex
	profile.Models = Models{ClientCodex: modelID}
	cfg.Profiles["dmx"] = profile

	if err := cfg.Validate(); err != nil {
		t.Fatalf("upstream model ID must not be rejected by product-specific naming policy: %v", err)
	}
}

func TestNormalizeFillsEveryNilCollection(t *testing.T) {
	cfg := Config{}
	cfg.Normalize()
	if cfg.Accounts == nil || cfg.Profiles == nil || cfg.Routes.Overrides == nil || cfg.Adapters == nil {
		t.Fatalf("normalized config still has nil collections: %#v", cfg)
	}
}

func TestNormalizedCopyDeepCopiesAdapterTargets(t *testing.T) {
	cfg := validConfig()
	cfg.Adapters = map[string]AdapterConfig{
		ClientCodex: {Enabled: true, Targets: []string{"a", "b"}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("adapters keyed by admitted client must validate: %v", err)
	}
}

func TestValidateRequiresAtLeastOneProfileAndAccount(t *testing.T) {
	cfg := validConfig()
	cfg.Profiles = map[string]Profile{}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "at least one profile") {
		t.Fatalf("empty profiles error = %v", err)
	}

	cfg = validConfig()
	cfg.Accounts = map[string]Account{}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "at least one account") {
		t.Fatalf("empty accounts error = %v", err)
	}
}

func TestValidateRejectsEmptyLabelsAndUnnamedOrUnendpointedAccounts(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Config)
		want string
	}{
		{"empty profile label", func(c *Config) {
			p := c.Profiles["dmx"]
			p.Label = "  "
			c.Profiles["dmx"] = p
		}, "empty label"},
		{"invalid account name", func(c *Config) {
			c.Accounts["bad name"] = c.Accounts["dmx"]
		}, "invalid account name"},
		{"empty account label", func(c *Config) {
			a := c.Accounts["dmx"]
			a.Label = " "
			c.Accounts["dmx"] = a
		}, "empty label"},
		{"account without endpoint", func(c *Config) {
			c.Accounts["dmx"] = Account{Label: "DMXAPI"}
		}, "must define at least one endpoint"},
		{"account probe invalid kind", func(c *Config) {
			a := c.Accounts["dmx"]
			a.AccountProbe = &AccountProbe{Kind: "bad kind", BaseURL: "https://diagnostics.example.test"}
			c.Accounts["dmx"] = a
		}, "invalid account probe provider"},
		{"account probe invalid endpoint", func(c *Config) {
			a := c.Accounts["dmx"]
			a.AccountProbe = &AccountProbe{Kind: "future", BaseURL: "not-a-url"}
			c.Accounts["dmx"] = a
		}, "account probe"},
		{"profile missing account", func(c *Config) {
			p := c.Profiles["dmx"]
			p.Account = ""
			c.Profiles["dmx"] = p
		}, "must reference an account"},
		{"profile unknown client", func(c *Config) {
			p := c.Profiles["dmx"]
			p.Client = "gemini"
			c.Profiles["dmx"] = p
		}, "unknown client"},
		{"override references unknown profile", func(c *Config) {
			c.Routes.Overrides[ClientClaude] = "missing"
		}, "references unknown profile"},
		{"unknown adapter", func(c *Config) {
			c.Adapters = map[string]AdapterConfig{"gemini": {Enabled: true}}
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
	cfg := NewConfig()
	cfg.Accounts["dmx"] = Account{Label: "DMXAPI", Endpoints: Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Profiles["codex"] = Profile{Label: "Codex", Account: "dmx"}
	cfg.Routes.Default = "codex"

	if _, _, err := cfg.ResolveRuntime(ClientCodex, "missing-profile"); err == nil || !strings.Contains(err.Error(), "unknown profile") {
		t.Fatalf("unknown profile error = %v", err)
	}

	orphan := cfg
	orphan.Profiles = map[string]Profile{"codex": {Label: "Codex", Account: "missing-account"}}
	orphan.Routes.Default = "codex"
	_, _, err := orphan.ResolveRuntime(ClientCodex, "")
	var accountErr *RuntimeProfileUnknownAccountError
	if !errors.As(err, &accountErr) || accountErr.ProfileID != "codex" || accountErr.AccountID != "missing-account" {
		t.Fatalf("unknown account error = %v", err)
	}

	_, _, err = cfg.ResolveRuntime(ClientClaude, "codex")
	var endpointErr *RuntimeMissingEndpointError
	if !errors.As(err, &endpointErr) || endpointErr.AccountID != "dmx" || endpointErr.Protocol != ProtocolAnthropic {
		t.Fatalf("missing endpoint error = %v", err)
	}

	scoped := cfg
	profile := scoped.Profiles["codex"]
	profile.Client = ClientCodex
	scoped.Profiles["codex"] = profile
	_, _, err = scoped.ResolveRuntime(ClientClaude, "codex")
	var mismatchErr *RuntimeProfileClientMismatchError
	if !errors.As(err, &mismatchErr) || mismatchErr.ProfileID != "codex" || mismatchErr.ExpectedClient != ClientCodex || mismatchErr.ActualClient != ClientClaude {
		t.Fatalf("profile client mismatch error = %v", err)
	}
}

func TestEndpointForRejectsUnknownClientAndMissingProtocolEndpoint(t *testing.T) {
	account := Account{ID: "dmx", Label: "DMXAPI", Endpoints: Endpoints{Anthropic: "https://dmx.test"}}
	if _, err := account.EndpointFor("gemini"); err == nil || !strings.Contains(err.Error(), "unknown client") {
		t.Fatalf("unknown client error = %v", err)
	}
	if _, err := account.EndpointFor(ClientCodex); err == nil || !strings.Contains(err.Error(), "no OpenAI Responses endpoint") {
		t.Fatalf("missing OpenAI endpoint error = %v", err)
	}
	if endpoint, err := account.EndpointFor(ClientClaude); err != nil || endpoint != "https://dmx.test" {
		t.Fatalf("EndpointFor = %q, %v", endpoint, err)
	}
}

func TestDomainErrorTextAndProfileSelectionBranches(t *testing.T) {
	if got := (&RuntimeProfileClientMismatchError{ProfileID: "one", ExpectedClient: "codex", ActualClient: "claude"}).Error(); !strings.Contains(got, "for codex") {
		t.Fatalf("mismatch error = %q", got)
	}
	if got := (&RuntimeProfileUnknownAccountError{ProfileID: "one", AccountID: "missing"}).Error(); !strings.Contains(got, "unknown account") {
		t.Fatalf("account error = %q", got)
	}
	if got := (&RuntimeMissingEndpointError{AccountID: "one", Protocol: EndpointProtocol("future")}).Error(); !strings.Contains(got, "future endpoint") {
		t.Fatalf("endpoint error = %q", got)
	}

	cfg := NewConfig()
	cfg.Accounts["generic"] = Account{Label: "Generic", Endpoints: Endpoints{Anthropic: "https://one.test"}}
	cfg.Profiles["a-skip"] = Profile{Label: "Skip", Account: "generic", Client: ClientClaude}
	cfg.Profiles["b-endpoint"] = Profile{Label: "Endpoint", Account: "generic"}
	if got := cfg.FirstProfileForClient(ClientCodex); got != "" {
		t.Fatalf("Codex profile = %q", got)
	}
	if got := cfg.FirstProfileForClient(ClientClaude); got != "a-skip" {
		t.Fatalf("Claude profile = %q", got)
	}
}
