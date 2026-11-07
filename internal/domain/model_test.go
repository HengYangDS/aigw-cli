package domain

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestConfigQueriesOwnProviderAndModelSelectionSemantics(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["gateway"] = Provider{
		Label:     "Gateway",
		Endpoints: Endpoints{Anthropic: "https://gateway.test/anthropic"},
	}
	cfg.Models["zeta"] = testModel("Zeta", "gateway", "claude-zeta", ProtocolAnthropic)
	cfg.Models["alpha"] = testModel("Alpha", "gateway", "claude-test", ProtocolAnthropic)
	cfg.SetSelectedModel(ClientClaude, "alpha", "")

	if got := cfg.ModelIDs(); !reflect.DeepEqual(got, []string{"alpha", "zeta"}) {
		t.Fatalf("RouteIDs() = %#v", got)
	}
	accountID, providerAccount, err := cfg.ResolveProvider("alpha")
	if err != nil || accountID != "gateway" || providerAccount.ID != "gateway" {
		t.Fatalf("ResolveAccount(route) = %q, %#v, %v", accountID, providerAccount, err)
	}
	accountID, _, err = cfg.ResolveProvider("gateway")
	if err != nil || accountID != "gateway" {
		t.Fatalf("ResolveAccount(account) = %q, %v", accountID, err)
	}
	if got := cfg.SelectedProviderIDs(); !reflect.DeepEqual(got, []string{"gateway"}) {
		t.Fatalf("SelectedAccountIDs() = %#v; want the selected Route's Account", got)
	}
	if _, _, err := cfg.ResolveProvider("missing"); err == nil {
		t.Fatal("ResolveAccount() accepted an unknown reference")
	}
}

func TestEnabledClientIDsFollowConfigurationNotSupportedCapabilities(t *testing.T) {
	cfg := NewConfig(testAdmission())
	if got := cfg.EnabledClientIDs(); len(got) != 0 {
		t.Fatalf("empty scope = %v", got)
	}
	cfg.SetClientActivation(ClientCodex, true, "", nil)
	cfg.SetClientActivation(ClientClaude, false, "", nil)
	if got := cfg.EnabledClientIDs(); !reflect.DeepEqual(got, []string{ClientCodex}) {
		t.Fatalf("single-client scope = %v", got)
	}
	cfg.SetClientActivation(ClientClaude, true, "", nil)
	if got := cfg.EnabledClientIDs(); !reflect.DeepEqual(got, []string{ClientClaude, ClientCodex}) {
		t.Fatalf("full scope = %v", got)
	}
}

func TestRequiredProviderTokensFollowEnabledModelsAndAuthentication(t *testing.T) {
	cfg := validConfig()
	cfg.Normalize()
	if got := cfg.RequiredProviderTokenIDs(); len(got) != 0 {
		t.Fatalf("disabled Routes require Tokens: %v", got)
	}
	binding := cfg.Clients[ClientCodex]
	binding.Enabled = true
	cfg.Clients[ClientCodex] = binding
	want := []string{cfg.Models[cfg.SelectedModel(ClientCodex)].Provider}
	if got := cfg.RequiredProviderTokenIDs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("enabled Token scope = %v, want %v", got, want)
	}
	binding.CredentialCommand = filepath.Join(t.TempDir(), "external-reader")
	cfg.Clients[ClientCodex] = binding
	if got := cfg.RequiredProviderTokenIDs(); len(got) != 0 {
		t.Fatalf("external credential ownership requires a duplicate Token: %v", got)
	}
	runtime, err := cfg.ResolveRuntime(ClientCodex, "")
	if err != nil || !runtime.RequiresProviderToken() || runtime.UsesAIGWCredentialStore() {
		t.Fatalf("external credential ownership changed wire authentication: %+v, %v", runtime, err)
	}
	cfg.Providers[want[0]] = cfg.Providers["dmx"]
	cfg.Models["dmx"] = testModel("Shared", want[0], "claude-test", ProtocolAnthropic)
	claude := cfg.Clients[ClientClaude]
	claude.Enabled = true
	cfg.Clients[ClientClaude] = claude
	if got := cfg.RequiredProviderTokenIDs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("shared Account lost its AIGW-owned credential consumer: %v", got)
	}
	claude.CredentialCommand = binding.CredentialCommand
	cfg.Clients[ClientClaude] = claude
	if got := cfg.RequiredProviderTokenIDs(); len(got) != 0 {
		t.Fatalf("shared external credential consumers require a duplicate Token: %v", got)
	}
	delete(cfg.Clients, ClientClaude)
	binding.CredentialCommand = ""
	binding.Authentication = AuthenticationClientNative
	binding.ModelProvider = "native"
	cfg.Clients[ClientCodex] = binding
	if got := cfg.RequiredProviderTokenIDs(); len(got) != 0 {
		t.Fatalf("client-native authentication requires Tokens: %v", got)
	}
}

func validConfig() Config {
	return Config{
		Version: ConfigVersion,
		Providers: map[string]Provider{
			"dmx":    {Label: "DMXAPI", Endpoints: Endpoints{OpenAIResponses: "https://example.test/v1", Anthropic: "https://example.test"}},
			"backup": {Label: "Backup", Endpoints: Endpoints{OpenAIResponses: "https://backup.test/v1"}},
		},
		Models: map[string]Model{
			"dmx":    testModel("DMXAPI", "dmx", "claude-test", ProtocolAnthropic),
			"backup": testModel("Backup", "backup", "gpt-test", ProtocolOpenAIResponses),
		},
		Recommendations: map[string]ClientRecommendation{},
		Clients: map[string]ClientBinding{
			ClientClaude: {Model: "dmx"},
			ClientCodex:  {Model: "backup"},
		},
		admission: testAdmission(),
	}
}

func testModel(label, account, model string, protocols ...EndpointProtocol) Model {
	interfaces := make(map[EndpointProtocol][]Capability, len(protocols))
	for _, protocol := range protocols {
		interfaces[protocol] = []Capability{}
	}
	return Model{Label: label, Provider: account, Family: model, Protocols: interfaces}
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
	if got := NewConfig(testAdmission()).Version; got != ConfigVersion {
		t.Fatalf("new config version = %d, want %d", got, ConfigVersion)
	}
}

func TestConfigCloneDoesNotShareMutableState(t *testing.T) {
	original := validConfig()
	account := original.Providers["dmx"]
	account.BalanceProbe = &BalanceProbe{Kind: "dmxapi", BaseURL: "https://diagnostics.test"}
	original.Providers["dmx"] = account
	original.Clients = map[string]ClientBinding{}
	original.SetClientActivation(ClientCodex, true, "", []string{"one"})
	original.SetRecommendedModel(ClientClaude, "dmx")

	clone := original.Clone()
	clone.Providers["dmx"].BalanceProbe.BaseURL = "https://changed.test"
	if original.Providers["dmx"].BalanceProbe.BaseURL != "https://diagnostics.test" {
		t.Fatal("clone shares account diagnostics with original")
	}
	original.Providers["dmx"].BalanceProbe.Kind = "another"
	if clone.Providers["dmx"].BalanceProbe.Kind != "dmxapi" {
		t.Fatal("original shares account diagnostics with clone")
	}
	clone.Providers["dmx"] = Provider{Label: "Changed"}
	clone.Models["default"] = Model{Label: "Changed"}
	clone.SetSelectedModel(ClientClaude, "changed", "")
	clone.SetRecommendedModel(ClientClaude, "changed")
	adapter := clone.Clients[ClientCodex]
	adapter.Targets[0] = "changed"
	clone.Clients[ClientCodex] = adapter

	if original.Providers["dmx"].Label == "Changed" || original.Models["default"].Label == "Changed" {
		t.Fatal("clone shares map state with original")
	}
	if original.SelectedModel(ClientClaude) == "changed" || original.RecommendedModel(ClientClaude) == "changed" {
		t.Fatal("clone shares selection overrides with original")
	}
	if original.Clients[ClientCodex].Targets[0] == "changed" {
		t.Fatal("clone shares adapter targets with original")
	}
}

func TestValidateRejectsUnsafeOrAmbiguousConfiguration(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Config)
		want string
	}{
		{"invalid route name", func(c *Config) { c.Models["bad name"] = c.Models["dmx"] }, "route name"},
		{"uppercase account name", func(c *Config) { c.Providers["DMX"] = c.Providers["dmx"] }, "must be lowercase"},
		{"unknown selected route", func(c *Config) { c.SetSelectedModel(ClientClaude, "missing", "") }, "unknown route"},
		{"unknown client", func(c *Config) { c.Clients["chat"] = ClientBinding{Model: "dmx"} }, "unknown client binding"},
		{"unknown recommended route", func(c *Config) { c.SetRecommendedModel(ClientClaude, "missing") }, "unknown route"},
		{"unknown recommended client", func(c *Config) {
			c.Recommendations["chat"] = ClientRecommendation{Primary: ClientSelection{Model: "dmx"}}
		}, "unknown client recommendation"},
		{"incompatible recommendation", func(c *Config) {
			c.Recommendations[ClientCodex] = ClientRecommendation{Primary: ClientSelection{Model: "dmx", Protocol: ProtocolAnthropic}}
		}, "does not support"},
		{"url user info", func(c *Config) {
			a := c.Providers["dmx"]
			a.Endpoints.Anthropic = "https://user:secret@example.test"
			c.Providers["dmx"] = a
		}, "userinfo"},
		{"url secret query", func(c *Config) {
			a := c.Providers["dmx"]
			a.Endpoints.Anthropic = "https://example.test?api_key=secret"
			c.Providers["dmx"] = a
		}, "credential-like"},
		{"remote plain http", func(c *Config) {
			a := c.Providers["dmx"]
			a.Endpoints.Anthropic = "http://example.test"
			c.Providers["dmx"] = a
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

func TestValidationReportsStableFirstProblemWithoutChangingConfiguration(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*Config)
		want string
	}{
		{"accounts", func(c *Config) {
			c.Providers = map[string]Provider{"alpha": {}, "zeta": {}}
		}, `account "alpha" has an empty label`},
		{"routes", func(c *Config) {
			c.Models = map[string]Model{"alpha": {}, "zeta": {}}
		}, `route "alpha" must reference an account`},
		{"recommendations", func(c *Config) {
			c.Recommendations = map[string]ClientRecommendation{
				ClientClaude: {Primary: ClientSelection{Model: "missing"}},
				ClientCodex:  {Primary: ClientSelection{Model: "missing"}},
			}
		}, `client recommendation "claude" choice 1: references unknown route "missing"`},
		{"client bindings", func(c *Config) {
			c.Clients = map[string]ClientBinding{"alpha": {}, "zeta": {}}
		}, `unknown client binding "alpha"`},
		{"protocol endpoints", func(c *Config) {
			c.Providers = map[string]Provider{"alpha": {Label: "Alpha", Endpoints: Endpoints{
				OpenAIResponses: "invalid", Anthropic: "invalid",
			}}}
		}, `account "alpha" endpoint anthropic: URL must use http or https and include a host`},
		{"credential query parameters", func(c *Config) {
			c.Providers = map[string]Provider{"alpha": {Label: "Alpha", Endpoints: Endpoints{
				Anthropic: "https://example.test?token=value&api_key=value",
			}}}
		}, `account "alpha" endpoint anthropic: credential-like query parameter "api_key" is forbidden`},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := validConfig()
			test.edit(&cfg)
			before := validConfig()
			test.edit(&before)
			for range 64 {
				err := cfg.Validate()
				if err == nil || err.Error() != test.want {
					t.Fatalf("Validate() = %v; want %s", err, test.want)
				}
			}
			if !reflect.DeepEqual(cfg, before) {
				t.Fatal("validation changed configuration")
			}
		})
	}
}

func TestValidateAllowsProviderNeutralExplicitDiagnostics(t *testing.T) {
	cfg := validConfig()
	account := cfg.Providers["dmx"]
	account.BalanceProbe = &BalanceProbe{Kind: "future-provider", BaseURL: "https://diagnostics.example.test"}
	cfg.Providers["dmx"] = account
	if err := cfg.Validate(); err != nil {
		t.Fatalf("explicit provider diagnostics must remain configuration-valid even when this build has no driver: %v", err)
	}
}

func TestValidateRejectsRuntimeModelReferencingUnknownProviderOrInvalidClientOptions(t *testing.T) {
	cfg := NewConfig(testAdmission())
	cfg.Providers["dmx"] = Provider{Label: "DMXAPI", Endpoints: Endpoints{OpenAIResponses: "https://dmx.test/v1", Anthropic: "https://dmx.test"}}
	cfg.Models["codex"] = testModel("Codex", "missing", "gpt-5.6", ProtocolOpenAIResponses)
	cfg.SetSelectedModel(ClientCodex, "codex", "")
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "unknown account") {
		t.Fatalf("error = %v", err)
	}
	cfg.Models["codex"] = testModel("Codex", "dmx", "claude-opus", ProtocolAnthropic)
	cfg.Clients[ClientClaude] = ClientBinding{Model: "codex", ModelProvider: "amazon-bedrock"}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "only supported for codex") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateRequiresEachSelectedBindingsClientProtocol(t *testing.T) {
	for _, client := range testClientSpecs() {
		t.Run(client.ID, func(t *testing.T) {
			cfg := validConfig()
			cfg.Normalize()
			cfg.Families["model"] = Family{Label: "Model"}
			cfg.Models = map[string]Model{"selected-route": {
				Label: "Selected Route", Provider: "dmx", Family: "model",
				Protocols: map[EndpointProtocol][]Capability{client.EndpointProtocols[0]: {}},
			}}
			cfg.Clients = map[string]ClientBinding{client.ID: {Model: "selected-route", Protocol: client.EndpointProtocols[0]}}
			if err := cfg.Validate(); err != nil {
				t.Fatalf("compatible route: %v", err)
			}
			account := cfg.Providers["dmx"]
			switch client.EndpointProtocols[0] {
			case ProtocolAnthropic:
				account.Endpoints.Anthropic = ""
			case ProtocolOpenAIResponses:
				account.Endpoints.OpenAIResponses = ""
			case ProtocolOpenAIChatCompletions:
				account.Endpoints.OpenAIChatCompletions = ""
			}
			cfg.Providers["dmx"] = account
			before := cfg.Clone()
			err := cfg.Validate()
			var missing *RuntimeMissingEndpointError
			if !errors.As(err, &missing) || missing.ProviderID != "dmx" || missing.Protocol != client.EndpointProtocols[0] {
				t.Fatalf("incompatible route error = %v; want Account dmx protocol %s", err, client.EndpointProtocols[0])
			}
			if !strings.Contains(err.Error(), "selected-route") {
				t.Fatalf("validation error omits Route: %v", err)
			}
			if !reflect.DeepEqual(cfg, before) {
				t.Fatal("validation changed configuration")
			}
		})
	}
}

func TestValidateRequiresAValidFamily(t *testing.T) {
	for _, testCase := range []struct {
		name string
		edit func(*Model)
		want string
	}{
		{name: "missing model", edit: func(route *Model) { route.Family = " " }, want: "must reference a model"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := validConfig()
			route := cfg.Models["dmx"]
			testCase.edit(&route)
			cfg.Models["dmx"] = route
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("error = %v, want substring %q", err, testCase.want)
			}
		})
	}
}

func TestValidateTreatsModelIDAsTransparentConfiguration(t *testing.T) {
	const routeID = "gpt-5.6-terra-cdx"
	cfg := validConfig()
	cfg.Normalize()
	cfg.Families["upstream-model"] = Family{Label: "Upstream Model"}
	route := cfg.Models["dmx"]
	route.Family = "upstream-model"
	route.Protocols = map[EndpointProtocol][]Capability{ProtocolOpenAIResponses: {}}
	delete(cfg.Models, "dmx")
	cfg.Models[routeID] = route
	delete(cfg.Clients, ClientClaude)
	cfg.SetSelectedModel(ClientCodex, routeID, "")

	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid route ID must not be rejected by product-specific naming policy: %v", err)
	}
}

func TestValidateTreatsUpstreamModelIDAsTransparentConfiguration(t *testing.T) {
	const modelID = "gpt-5.6-terra-cdx"
	cfg := validConfig()
	route := cfg.Models["dmx"]
	route.UpstreamModel = modelID
	route.Protocols = map[EndpointProtocol][]Capability{ProtocolOpenAIResponses: {}}
	cfg.Models["dmx"] = route
	delete(cfg.Clients, ClientClaude)
	cfg.SetSelectedModel(ClientCodex, "dmx", "")

	if err := cfg.Validate(); err != nil {
		t.Fatalf("upstream model ID must not be rejected by product-specific naming policy: %v", err)
	}
}

func TestModelUpstreamModelIDPreservesExactProviderIdentity(t *testing.T) {
	if got := (Model{Family: "canonical"}).UpstreamModelID(); got != "canonical" {
		t.Fatalf("fallback upstream model = %q", got)
	}
	if got := (Model{Family: "canonical", UpstreamModel: "provider/exact"}).UpstreamModelID(); got != "provider/exact" {
		t.Fatalf("explicit upstream model = %q", got)
	}
}

func TestNormalizeFillsEveryNilCollection(t *testing.T) {
	cfg := Config{}
	cfg.Normalize()
	if cfg.Providers == nil || cfg.Models == nil || cfg.Recommendations == nil || cfg.Clients == nil {
		t.Fatalf("normalized config still has nil collections: %#v", cfg)
	}
}

func TestValidateAdmitsClientAdapterTargets(t *testing.T) {
	cfg := validConfig()
	cfg.Clients = map[string]ClientBinding{
		ClientCodex: {Model: "backup", Enabled: true, Targets: []string{"a", "b"}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("adapters keyed by admitted client must validate: %v", err)
	}
}

// TestValidateAcceptsAConfigurationWithoutModels shows that providers without
// models, and a configuration without anything, are valid, while a model
// still needs its provider.
func TestValidateAcceptsAConfigurationWithoutModels(t *testing.T) {
	cfg := validConfig()
	cfg.Models = map[string]Model{}
	cfg.Clients = map[string]ClientBinding{}
	cfg.Recommendations = map[string]ClientRecommendation{}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("providers without models: %v", err)
	}
	empty := NewConfig(testAdmission())
	if err := empty.Validate(); err != nil {
		t.Fatalf("an empty configuration: %v", err)
	}
	cfg = validConfig()
	cfg.Providers = map[string]Provider{}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "references unknown account") {
		t.Fatalf("models without their provider: %v", err)
	}
}

func TestValidateRejectsEmptyLabelsAndUnnamedOrUnendpointedProviders(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Config)
		want string
	}{
		{"empty model label", func(c *Config) {
			c.Families = map[string]Family{"claude-test": {Label: "  "}}
		}, "empty label"},
		{"invalid account name", func(c *Config) {
			c.Providers["bad name"] = c.Providers["dmx"]
		}, "invalid account name"},
		{"empty account label", func(c *Config) {
			a := c.Providers["dmx"]
			a.Label = " "
			c.Providers["dmx"] = a
		}, "empty label"},
		{"account without endpoint", func(c *Config) {
			c.Providers["dmx"] = Provider{Label: "DMXAPI"}
		}, "must define at least one endpoint"},
		{"account probe invalid kind", func(c *Config) {
			a := c.Providers["dmx"]
			a.BalanceProbe = &BalanceProbe{Kind: "bad kind", BaseURL: "https://diagnostics.example.test"}
			c.Providers["dmx"] = a
		}, "invalid account probe provider"},
		{"account probe invalid endpoint", func(c *Config) {
			a := c.Providers["dmx"]
			a.BalanceProbe = &BalanceProbe{Kind: "future", BaseURL: "not-a-url"}
			c.Providers["dmx"] = a
		}, "account probe"},
		{"route missing account", func(c *Config) {
			p := c.Models["dmx"]
			p.Provider = ""
			c.Models["dmx"] = p
		}, "must reference an account"},
		{"binding references unknown route", func(c *Config) {
			c.SetSelectedModel(ClientClaude, "missing", "")
		}, "references unknown route"},
		{"unknown client binding", func(c *Config) {
			c.Clients = map[string]ClientBinding{"gemini": {Enabled: true}}
		}, "unknown client binding"},
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
