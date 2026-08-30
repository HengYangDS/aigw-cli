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
	cfg.Profiles["zeta"] = Profile{Label: "Zeta", Account: "gateway", Client: ClientClaude, Model: "claude-zeta"}
	cfg.Profiles["alpha"] = Profile{Label: "Alpha", Account: "gateway", Client: ClientClaude, Model: "claude-test"}
	cfg.Routes[ClientClaude] = "alpha"

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
			"dmx":    {Label: "DMXAPI", Account: "dmx", Client: ClientClaude, Model: "claude-test"},
			"backup": {Label: "Backup", Account: "backup", Client: ClientCodex, Model: "gpt-test"},
		},
		Routes: Routes{ClientClaude: "dmx", ClientCodex: "backup"},
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
	clone.Routes[ClientClaude] = "changed"
	adapter := clone.Adapters[ClientCodex]
	adapter.Targets[0] = "changed"
	clone.Adapters[ClientCodex] = adapter

	if original.Accounts["dmx"].Label == "Changed" || original.Profiles["default"].Label == "Changed" {
		t.Fatal("clone shares map state with original")
	}
	if original.Routes[ClientClaude] == "changed" {
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
		{"uppercase account name", func(c *Config) { c.Accounts["DMX"] = c.Accounts["dmx"] }, "must be lowercase"},
		{"unknown profile route", func(c *Config) { c.Routes[ClientClaude] = "missing" }, "unknown profile"},
		{"unknown client", func(c *Config) { c.Routes["chat"] = "dmx" }, "unknown route"},
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
	cfg.Profiles["gpt-5.6"] = Profile{Label: "GPT-5.6", Account: "dmx", Client: ClientCodex, Model: "gpt-5.6"}
	cfg.Profiles["claude-opus"] = Profile{Label: "Claude Opus", Account: "dmx", Client: ClientClaude, Model: "claude-opus"}
	cfg.Routes[ClientCodex] = "gpt-5.6"
	cfg.Routes[ClientClaude] = "claude-opus"
	got, err := cfg.ResolveRuntime(ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.ProfileID != "gpt-5.6" || got.AccountID != "dmx" || got.Model != "gpt-5.6" || got.Endpoint != "https://dmx.test/v1" {
		t.Fatalf("runtime = %#v", got)
	}
	got, err = cfg.ResolveRuntime(ClientClaude, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.ProfileID != "claude-opus" || got.Model != "claude-opus" || got.Endpoint != "https://dmx.test" {
		t.Fatalf("runtime = %#v", got)
	}
}

func TestClientForProfileRequiresCanonicalScope(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["dmx"] = Account{Label: "DMXAPI", Endpoints: Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Profiles["codex"] = Profile{Label: "Codex", Account: "dmx", Client: ClientCodex, Model: "gpt-5.6"}
	cfg.Profiles["shared"] = Profile{Label: "Shared", Account: "dmx", Model: "gpt-5.6"}

	client, err := cfg.ClientForProfile("codex")
	if err != nil || client != ClientCodex {
		t.Fatalf("ClientForProfile(codex) = %q, %v", client, err)
	}
	if _, err := cfg.ClientForProfile("shared"); err == nil || !strings.Contains(err.Error(), "does not declare exactly one admitted client") {
		t.Fatalf("unscoped profile error = %v", err)
	}
	if _, err := cfg.ClientForProfile("missing"); err == nil || !strings.Contains(err.Error(), "unknown profile") {
		t.Fatalf("unknown profile error = %v", err)
	}
}

func TestValidateRejectsRuntimeProfileReferencingUnknownAccountOrWrongClient(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["dmx"] = Account{Label: "DMXAPI", Endpoints: Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Profiles["codex"] = Profile{Label: "Codex", Account: "missing", Client: ClientCodex, Model: "gpt-5.6"}
	cfg.Routes[ClientCodex] = "codex"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "unknown account") {
		t.Fatalf("error = %v", err)
	}
	cfg.Profiles["codex"] = Profile{Label: "Codex", Account: "dmx", Client: ClientClaude, Model: "claude-opus", ModelProvider: "amazon-bedrock"}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "only supported for codex") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateRequiresExplicitClientAndModel(t *testing.T) {
	for _, testCase := range []struct {
		name string
		edit func(*Profile)
		want string
	}{
		{name: "missing client", edit: func(profile *Profile) { profile.Client = "" }, want: "unknown client"},
		{name: "unknown client", edit: func(profile *Profile) { profile.Client = "gemini" }, want: "unknown client"},
		{name: "missing model", edit: func(profile *Profile) { profile.Model = " " }, want: "must define a model"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := validConfig()
			profile := cfg.Profiles["dmx"]
			testCase.edit(&profile)
			cfg.Profiles["dmx"] = profile
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("error = %v, want substring %q", err, testCase.want)
			}
		})
	}
}

func TestValidateTreatsProfileIDAsTransparentConfiguration(t *testing.T) {
	const profileID = "gpt-5.6-terra-cdx"
	cfg := validConfig()
	profile := cfg.Profiles["dmx"]
	profile.Client = ClientCodex
	profile.Model = "upstream-model"
	delete(cfg.Profiles, "dmx")
	cfg.Profiles[profileID] = profile
	delete(cfg.Routes, ClientClaude)
	cfg.Routes[ClientCodex] = profileID

	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid profile ID must not be rejected by product-specific naming policy: %v", err)
	}
}

func TestValidateTreatsUpstreamModelIDAsTransparentConfiguration(t *testing.T) {
	const modelID = "gpt-5.6-terra-cdx"
	cfg := validConfig()
	profile := cfg.Profiles["dmx"]
	profile.Client = ClientCodex
	profile.Model = modelID
	cfg.Profiles["dmx"] = profile
	delete(cfg.Routes, ClientClaude)
	cfg.Routes[ClientCodex] = "dmx"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("upstream model ID must not be rejected by product-specific naming policy: %v", err)
	}
}

func TestNormalizeFillsEveryNilCollection(t *testing.T) {
	cfg := Config{}
	cfg.Normalize()
	if cfg.Accounts == nil || cfg.Profiles == nil || cfg.Routes == nil || cfg.Adapters == nil {
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
			c.Routes[ClientClaude] = "missing"
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
	cfg.Profiles["codex"] = Profile{Label: "Codex", Account: "dmx", Client: ClientCodex, Model: "gpt-test"}
	cfg.Routes[ClientCodex] = "codex"

	if _, err := cfg.ResolveRuntime(ClientCodex, "missing-profile"); err == nil || !strings.Contains(err.Error(), "unknown profile") {
		t.Fatalf("unknown profile error = %v", err)
	}

	orphan := cfg
	orphan.Profiles = map[string]Profile{"codex": {Label: "Codex", Account: "missing-account", Client: ClientCodex, Model: "gpt-test"}}
	orphan.Routes[ClientCodex] = "codex"
	_, err := orphan.ResolveRuntime(ClientCodex, "")
	var accountErr *RuntimeProfileUnknownAccountError
	if !errors.As(err, &accountErr) || accountErr.ProfileID != "codex" || accountErr.AccountID != "missing-account" {
		t.Fatalf("unknown account error = %v", err)
	}

	claudeProfile := cfg.Profiles["codex"]
	claudeProfile.Client = ClientClaude
	claudeProfile.Model = "claude-test"
	cfg.Profiles["claude"] = claudeProfile
	_, err = cfg.ResolveRuntime(ClientClaude, "claude")
	var endpointErr *RuntimeMissingEndpointError
	if !errors.As(err, &endpointErr) || endpointErr.AccountID != "dmx" || endpointErr.Protocol != ProtocolAnthropic {
		t.Fatalf("missing endpoint error = %v", err)
	}

	scoped := cfg
	profile := scoped.Profiles["codex"]
	profile.Client = ClientCodex
	scoped.Profiles["codex"] = profile
	_, err = scoped.ResolveRuntime(ClientClaude, "codex")
	var mismatchErr *RuntimeProfileClientMismatchError
	if !errors.As(err, &mismatchErr) || mismatchErr.ProfileID != "codex" || mismatchErr.ExpectedClient != ClientCodex || mismatchErr.ActualClient != ClientClaude {
		t.Fatalf("profile client mismatch error = %v", err)
	}
}

func TestSelectRoutesForConnectedAccountsKeepsCapabilityAndChoosesUsableProfiles(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["alpha"] = Account{Label: "Alpha", Endpoints: Endpoints{Anthropic: "https://alpha.test", OpenAIResponses: "https://alpha.test/v1"}}
	cfg.Accounts["beta"] = Account{Label: "Beta", Endpoints: Endpoints{Anthropic: "https://beta.test", OpenAIResponses: "https://beta.test/v1"}}
	cfg.Profiles["alpha-claude"] = Profile{Label: "Alpha Claude", Account: "alpha", Client: ClientClaude, Model: "claude-test"}
	cfg.Profiles["alpha-codex"] = Profile{Label: "Alpha Codex", Account: "alpha", Client: ClientCodex, Model: "gpt-test"}
	cfg.Profiles["beta-claude"] = Profile{Label: "Beta Claude", Account: "beta", Client: ClientClaude, Model: "claude-test"}
	cfg.Profiles["beta-codex"] = Profile{Label: "Beta Codex", Account: "beta", Client: ClientCodex, Model: "gpt-test"}
	cfg.Routes[ClientCodex] = "alpha-codex"
	cfg.Routes[ClientClaude] = "alpha-claude"
	cfg.Routes[ClientCodex] = "alpha-codex"

	one, err := cfg.SelectRoutesForConnectedAccounts([]string{"beta"})
	if err != nil {
		t.Fatal(err)
	}
	if one.Routes[ClientClaude] != "beta-claude" || one.Routes[ClientCodex] != "beta-codex" {
		t.Fatalf("one connected Account routes = %#v", one.Routes)
	}
	if len(one.Accounts) != 2 || len(one.Profiles) != 4 {
		t.Fatalf("route selection discarded catalogue capability: %#v", one)
	}

	both, err := cfg.SelectRoutesForConnectedAccounts([]string{"alpha", "beta"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(both.Routes, cfg.Routes) {
		t.Fatalf("usable recommended routes changed: got %#v want %#v", both.Routes, cfg.Routes)
	}

	if _, err := cfg.SelectRoutesForConnectedAccounts([]string{"missing"}); err == nil || !strings.Contains(err.Error(), "unknown account") {
		t.Fatalf("unknown connected Account error = %v", err)
	}
}

func TestSelectRoutesForConnectedAccountsPreservesRecommendedModels(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["preferred"] = Account{Label: "Preferred", Endpoints: Endpoints{Anthropic: "https://preferred.test", OpenAIResponses: "https://preferred.test/v1"}}
	cfg.Accounts["connected"] = Account{Label: "Connected", Endpoints: Endpoints{Anthropic: "https://connected.test", OpenAIResponses: "https://connected.test/v1"}}
	cfg.Profiles["preferred-claude"] = Profile{Label: "Preferred Claude", Account: "preferred", Client: ClientClaude, Model: "claude-fable-5"}
	cfg.Profiles["preferred-codex"] = Profile{Label: "Preferred Codex", Account: "preferred", Client: ClientCodex, Model: "gpt-5.6-sol"}
	cfg.Profiles["connected-claude"] = Profile{Label: "Connected Claude", Account: "connected", Client: ClientClaude, Model: "claude-fable-5"}
	cfg.Profiles["connected-codex-luna"] = Profile{Label: "Connected Codex Luna", Account: "connected", Client: ClientCodex, Model: "gpt-5.6-luna"}
	cfg.Profiles["connected-codex-sol"] = Profile{Label: "Connected Codex Sol", Account: "connected", Client: ClientCodex, Model: "gpt-5.6-sol"}
	cfg.Routes[ClientCodex] = "preferred-codex"
	cfg.Routes[ClientClaude] = "preferred-claude"
	cfg.Routes[ClientCodex] = "preferred-codex"

	selected, err := cfg.SelectRoutesForConnectedAccounts([]string{"connected"})
	if err != nil {
		t.Fatal(err)
	}
	if selected.Routes[ClientCodex] != "connected-codex-sol" {
		t.Fatalf("Codex route lost the recommended model: %#v", selected.Routes)
	}
	if selected.Routes[ClientClaude] != "connected-claude" {
		t.Fatalf("Claude route = %q", selected.Routes[ClientClaude])
	}
}

func TestRoutedAccountIDsReturnsUniqueStableActiveAccounts(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["alpha"] = Account{Label: "Alpha", Endpoints: Endpoints{Anthropic: "https://alpha.test", OpenAIResponses: "https://alpha.test/v1"}}
	cfg.Accounts["optional"] = Account{Label: "Optional", Endpoints: Endpoints{OpenAIResponses: "https://optional.test/v1"}}
	cfg.Profiles["alpha-claude"] = Profile{Label: "Alpha Claude", Account: "alpha", Client: ClientClaude, Model: "claude-test"}
	cfg.Profiles["alpha-codex"] = Profile{Label: "Alpha Codex", Account: "alpha", Client: ClientCodex, Model: "gpt-test"}
	cfg.Profiles["optional"] = Profile{Label: "Optional", Account: "optional", Client: ClientCodex, Model: "gpt-optional"}
	cfg.Routes[ClientClaude] = "alpha-claude"
	cfg.Routes[ClientCodex] = "alpha-codex"

	if got := cfg.RoutedAccountIDs(); !reflect.DeepEqual(got, []string{"alpha"}) {
		t.Fatalf("RoutedAccountIDs() = %#v", got)
	}
}

func TestSelectRoutesForConnectedAccountFallsBackToAnyUsableClient(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["claude-only"] = Account{Label: "Claude only", Endpoints: Endpoints{Anthropic: "https://claude.test"}}
	cfg.Accounts["codex"] = Account{Label: "Codex", Endpoints: Endpoints{OpenAIResponses: "https://codex.test/v1"}}
	cfg.Profiles["claude"] = Profile{Label: "Claude", Account: "claude-only", Client: ClientClaude, Model: "claude-test"}
	cfg.Profiles["codex"] = Profile{Label: "Codex", Account: "codex", Client: ClientCodex, Model: "gpt-test"}
	cfg.Routes[ClientCodex] = "codex"
	cfg.Routes[ClientCodex] = "codex"

	got, err := cfg.SelectRoutesForConnectedAccounts([]string{"claude-only"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Routes[ClientClaude] != "claude" {
		t.Fatalf("fallback routes = %#v", got.Routes)
	}
}

func TestSelectRoutesForConnectedAccountsSkipsProfilesWithoutTheClientEndpoint(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["team"] = Account{Label: "Team", Endpoints: Endpoints{Anthropic: "https://team.test"}}
	cfg.Profiles["generic"] = Profile{Label: "Generic", Account: "team", Client: ClientClaude, Model: "claude-test"}

	selected, err := cfg.SelectRoutesForConnectedAccounts([]string{"team"})
	if err != nil {
		t.Fatal(err)
	}
	if selected.Routes[ClientCodex] != "" {
		t.Fatalf("profile without Responses endpoint selected for Codex: %#v", selected.Routes)
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

func TestResolveAccountRejectsProfileWithUnknownAccount(t *testing.T) {
	cfg := NewConfig()
	cfg.Profiles["orphan"] = Profile{Label: "Orphan", Account: "missing"}
	_, _, err := cfg.ResolveAccount("orphan")
	if err == nil || !strings.Contains(err.Error(), "references unknown account") {
		t.Fatalf("orphan profile error = %v", err)
	}
}

func TestDomainErrorTextAndProfileSelectionBranches(t *testing.T) {
	if got := endpointProtocolName(ProtocolAnthropic); got != "Anthropic" {
		t.Fatalf("Anthropic protocol name = %q", got)
	}
	if got := endpointProtocolName(ProtocolOpenAIResponses); got != "OpenAI Responses" {
		t.Fatalf("Responses protocol name = %q", got)
	}
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
	cfg.Profiles["b-endpoint"] = Profile{Label: "Endpoint", Account: "generic", Client: ClientClaude, Model: "claude-endpoint"}
	if got := cfg.FirstProfileForClient(ClientCodex); got != "" {
		t.Fatalf("Codex profile = %q", got)
	}
	if got := cfg.FirstProfileForClient(ClientClaude); got != "a-skip" {
		t.Fatalf("Claude profile = %q", got)
	}
}
