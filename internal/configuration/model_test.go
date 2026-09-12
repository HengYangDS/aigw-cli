package configuration

import (
	"errors"
	"fmt"
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
	account := original.Accounts["dmx"]
	account.AccountProbe = &AccountProbe{Kind: "dmxapi", BaseURL: "https://diagnostics.test"}
	original.Accounts["dmx"] = account
	original.Adapters = map[string]AdapterConfig{}
	original.Adapters[ClientCodex] = AdapterConfig{Enabled: true, Targets: []string{"one"}}

	clone := original.Clone()
	clone.Accounts["dmx"].AccountProbe.BaseURL = "https://changed.test"
	if original.Accounts["dmx"].AccountProbe.BaseURL != "https://diagnostics.test" {
		t.Fatal("clone shares account diagnostics with original")
	}
	original.Accounts["dmx"].AccountProbe.Kind = "another"
	if clone.Accounts["dmx"].AccountProbe.Kind != "dmxapi" {
		t.Fatal("original shares account diagnostics with clone")
	}
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

func TestValidationReportsStableFirstProblemWithoutChangingConfiguration(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*Config)
		want string
	}{
		{"accounts", func(c *Config) {
			c.Accounts = map[string]Account{"alpha": {}, "zeta": {}}
		}, `account "alpha" has an empty label`},
		{"profiles", func(c *Config) {
			c.Profiles = map[string]Profile{"alpha": {}, "zeta": {}}
		}, `profile "alpha" has an empty label`},
		{"routes", func(c *Config) {
			c.Routes = Routes{ClientClaude: "missing", ClientCodex: "missing"}
		}, `route "claude" references unknown profile "missing"`},
		{"adapters", func(c *Config) {
			c.Adapters = map[string]AdapterConfig{"alpha": {}, "zeta": {}}
		}, `unknown adapter "alpha"`},
		{"protocol endpoints", func(c *Config) {
			c.Accounts = map[string]Account{"alpha": {Label: "Alpha", Endpoints: Endpoints{
				OpenAIResponses: "invalid", Anthropic: "invalid",
			}}}
		}, `account "alpha" endpoint anthropic: URL must use http or https and include a host`},
		{"credential query parameters", func(c *Config) {
			c.Accounts = map[string]Account{"alpha": {Label: "Alpha", Endpoints: Endpoints{
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
	account := cfg.Accounts["dmx"]
	account.AccountProbe = &AccountProbe{Kind: "future-provider", BaseURL: "https://diagnostics.example.test"}
	cfg.Accounts["dmx"] = account
	if err := cfg.Validate(); err != nil {
		t.Fatalf("explicit provider diagnostics must remain configuration-valid even when this build has no driver: %v", err)
	}
}

func TestValidateRejectsRuntimeProfileReferencingUnknownAccountOrWrongClient(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["dmx"] = Account{Label: "DMXAPI", Endpoints: Endpoints{OpenAIResponses: "https://dmx.test/v1", Anthropic: "https://dmx.test"}}
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

func TestValidateRequiresEachProfilesClientProtocol(t *testing.T) {
	for _, client := range AdmittedClientSpecs() {
		for _, selected := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/selected=%t", client.ID, selected), func(t *testing.T) {
				cfg := validConfig()
				cfg.Normalize()
				cfg.Profiles = map[string]Profile{"service": {
					Label: "Service", Account: "dmx", Client: client.ID, Model: "model",
				}}
				cfg.Routes = Routes{}
				if selected {
					cfg.Routes[client.ID] = "service"
				}
				if err := cfg.Validate(); err != nil {
					t.Fatalf("compatible profile: %v", err)
				}
				account := cfg.Accounts["dmx"]
				switch client.EndpointProtocol {
				case ProtocolAnthropic:
					account.Endpoints.Anthropic = ""
				case ProtocolOpenAIResponses:
					account.Endpoints.OpenAIResponses = ""
				}
				cfg.Accounts["dmx"] = account
				before := cfg.Clone()
				err := cfg.Validate()
				var missing *RuntimeMissingEndpointError
				if !errors.As(err, &missing) || missing.AccountID != "dmx" || missing.Protocol != client.EndpointProtocol {
					t.Fatalf("incompatible profile error = %v; want Account dmx protocol %s", err, client.EndpointProtocol)
				}
				if !strings.Contains(err.Error(), "service") {
					t.Fatalf("validation error omits Profile: %v", err)
				}
				if !reflect.DeepEqual(cfg, before) {
					t.Fatal("validation changed configuration")
				}
			})
		}
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

func TestValidateAdmitsClientAdapterTargets(t *testing.T) {
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
