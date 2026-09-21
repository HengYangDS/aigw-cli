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
	cfg.Profiles["zeta"] = Profile{Label: "Zeta", Account: "gateway", Model: "claude-zeta"}
	cfg.Profiles["alpha"] = Profile{Label: "Alpha", Account: "gateway", Model: "claude-test"}
	cfg.SetSelectedProfile(ClientClaude, "alpha")

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
	if !cfg.ClientUsesAccount(ClientClaude, "gateway") || cfg.ClientUsesAccount(ClientClaude, "other") {
		t.Fatal("ClientUsesAccount() did not follow the selected binding")
	}
	if _, _, err := cfg.ResolveAccount("missing"); err == nil {
		t.Fatal("ResolveAccount() accepted an unknown reference")
	}
}

func TestEnabledClientIDsFollowConfigurationNotSupportedCapabilities(t *testing.T) {
	cfg := NewConfig()
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

func TestRequiredAccountTokensFollowEnabledRoutesAndAuthentication(t *testing.T) {
	cfg := validConfig()
	cfg.Normalize()
	if got := cfg.RequiredAccountTokenIDs(); len(got) != 0 {
		t.Fatalf("disabled Routes require Tokens: %v", got)
	}
	binding := cfg.Clients[ClientCodex]
	binding.Enabled = true
	cfg.Clients[ClientCodex] = binding
	want := []string{cfg.Profiles[cfg.SelectedProfile(ClientCodex)].Account}
	if got := cfg.RequiredAccountTokenIDs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("enabled Token scope = %v, want %v", got, want)
	}
	binding.Authentication = AuthenticationClientNative
	binding.ModelProvider = "native"
	cfg.Clients[ClientCodex] = binding
	if got := cfg.RequiredAccountTokenIDs(); len(got) != 0 {
		t.Fatalf("client-native authentication requires Tokens: %v", got)
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
			"dmx":    {Label: "DMXAPI", Account: "dmx", Model: "claude-test"},
			"backup": {Label: "Backup", Account: "backup", Model: "gpt-test"},
		},
		Recommendations: map[string]ClientSelection{},
		Clients: map[string]ClientBinding{
			ClientClaude: {Profile: "dmx"},
			ClientCodex:  {Profile: "backup"},
		},
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
	original.Clients = map[string]ClientBinding{}
	original.SetClientActivation(ClientCodex, true, "", []string{"one"})
	original.SetRecommendedProfile(ClientClaude, "dmx")

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
	clone.SetSelectedProfile(ClientClaude, "changed")
	clone.SetRecommendedProfile(ClientClaude, "changed")
	adapter := clone.Clients[ClientCodex]
	adapter.Targets[0] = "changed"
	clone.Clients[ClientCodex] = adapter

	if original.Accounts["dmx"].Label == "Changed" || original.Profiles["default"].Label == "Changed" {
		t.Fatal("clone shares map state with original")
	}
	if original.SelectedProfile(ClientClaude) == "changed" || original.RecommendedProfile(ClientClaude) == "changed" {
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
		{"invalid profile name", func(c *Config) { c.Profiles["bad name"] = c.Profiles["dmx"] }, "profile name"},
		{"uppercase account name", func(c *Config) { c.Accounts["DMX"] = c.Accounts["dmx"] }, "must be lowercase"},
		{"unknown selected profile", func(c *Config) { c.SetSelectedProfile(ClientClaude, "missing") }, "unknown profile"},
		{"unknown client", func(c *Config) { c.Clients["chat"] = ClientBinding{Profile: "dmx"} }, "unknown client binding"},
		{"unknown recommended profile", func(c *Config) { c.SetRecommendedProfile(ClientClaude, "missing") }, "unknown profile"},
		{"unknown recommended client", func(c *Config) { c.Recommendations["chat"] = ClientSelection{Profile: "dmx"} }, "unknown client recommendation"},
		{"incompatible recommendation", func(c *Config) {
			c.Recommendations[ClientCodex] = ClientSelection{Profile: "dmx", Protocol: ProtocolAnthropic}
		}, "does not support"},
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
		{"recommendations", func(c *Config) {
			c.Recommendations = map[string]ClientSelection{ClientClaude: {Profile: "missing"}, ClientCodex: {Profile: "missing"}}
		}, `client recommendation "claude": references unknown profile "missing"`},
		{"client bindings", func(c *Config) {
			c.Clients = map[string]ClientBinding{"alpha": {}, "zeta": {}}
		}, `unknown client binding "alpha"`},
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

func TestValidateRejectsRuntimeProfileReferencingUnknownAccountOrInvalidClientOptions(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["dmx"] = Account{Label: "DMXAPI", Endpoints: Endpoints{OpenAIResponses: "https://dmx.test/v1", Anthropic: "https://dmx.test"}}
	cfg.Profiles["codex"] = Profile{Label: "Codex", Account: "missing", Model: "gpt-5.6"}
	cfg.SetSelectedProfile(ClientCodex, "codex")
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "unknown account") {
		t.Fatalf("error = %v", err)
	}
	cfg.Profiles["codex"] = Profile{Label: "Codex", Account: "dmx", Model: "claude-opus"}
	cfg.Clients[ClientClaude] = ClientBinding{Profile: "codex", ModelProvider: "amazon-bedrock"}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "only supported for codex") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateRequiresEachSelectedBindingsClientProtocol(t *testing.T) {
	for _, client := range AdmittedClientSpecs() {
		t.Run(client.ID, func(t *testing.T) {
			cfg := validConfig()
			cfg.Normalize()
			cfg.Profiles = map[string]Profile{"selected-profile": {
				Label: "Selected Profile", Account: "dmx", Model: "model",
			}}
			cfg.Clients = map[string]ClientBinding{client.ID: {Profile: "selected-profile", Protocol: client.EndpointProtocols[0]}}
			if err := cfg.Validate(); err != nil {
				t.Fatalf("compatible profile: %v", err)
			}
			account := cfg.Accounts["dmx"]
			switch client.EndpointProtocols[0] {
			case ProtocolAnthropic:
				account.Endpoints.Anthropic = ""
			case ProtocolOpenAIResponses:
				account.Endpoints.OpenAIResponses = ""
			case ProtocolOpenAIChatCompletions:
				account.Endpoints.OpenAIChatCompletions = ""
			}
			cfg.Accounts["dmx"] = account
			before := cfg.Clone()
			err := cfg.Validate()
			var missing *RuntimeMissingEndpointError
			if !errors.As(err, &missing) || missing.AccountID != "dmx" || missing.Protocol != client.EndpointProtocols[0] {
				t.Fatalf("incompatible profile error = %v; want Account dmx protocol %s", err, client.EndpointProtocols[0])
			}
			if !strings.Contains(err.Error(), "selected-profile") {
				t.Fatalf("validation error omits Profile: %v", err)
			}
			if !reflect.DeepEqual(cfg, before) {
				t.Fatal("validation changed configuration")
			}
		})
	}
}

func TestValidateRequiresAValidModel(t *testing.T) {
	for _, testCase := range []struct {
		name string
		edit func(*Profile)
		want string
	}{
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
	profile.Model = "upstream-model"
	delete(cfg.Profiles, "dmx")
	cfg.Profiles[profileID] = profile
	delete(cfg.Clients, ClientClaude)
	cfg.SetSelectedProfile(ClientCodex, profileID)

	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid profile ID must not be rejected by product-specific naming policy: %v", err)
	}
}

func TestValidateTreatsUpstreamModelIDAsTransparentConfiguration(t *testing.T) {
	const modelID = "gpt-5.6-terra-cdx"
	cfg := validConfig()
	profile := cfg.Profiles["dmx"]
	profile.Model = modelID
	cfg.Profiles["dmx"] = profile
	delete(cfg.Clients, ClientClaude)
	cfg.SetSelectedProfile(ClientCodex, "dmx")

	if err := cfg.Validate(); err != nil {
		t.Fatalf("upstream model ID must not be rejected by product-specific naming policy: %v", err)
	}
}

func TestNormalizeFillsEveryNilCollection(t *testing.T) {
	cfg := Config{}
	cfg.Normalize()
	if cfg.Accounts == nil || cfg.Profiles == nil || cfg.Recommendations == nil || cfg.Clients == nil {
		t.Fatalf("normalized config still has nil collections: %#v", cfg)
	}
}

func TestValidateAdmitsClientAdapterTargets(t *testing.T) {
	cfg := validConfig()
	cfg.Clients = map[string]ClientBinding{
		ClientCodex: {Profile: "backup", Enabled: true, Targets: []string{"a", "b"}},
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
		{"override references unknown profile", func(c *Config) {
			c.SetSelectedProfile(ClientClaude, "missing")
		}, "references unknown profile"},
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

func TestEndpointHTTPAdmissionUsesLoopbackIdentity(t *testing.T) {
	for _, test := range []struct {
		host    string
		allowed bool
	}{
		{"localhost", true},
		{"LOCALHOST", true},
		{"127.0.0.1", true},
		{"127.1.2.3", true},
		{"[::1]", true},
		{"[::ffff:127.0.0.2]", true},
		{"192.168.1.1", false},
		{"0.0.0.0", false},
		{"[::]", false},
		{"localhost.example.test", false},
		{"[::ffff:192.168.1.1]", false},
	} {
		t.Run(test.host, func(t *testing.T) {
			if err := validateEndpoint("http://" + test.host + ":4567/v1"); (err == nil) != test.allowed {
				t.Errorf("endpoint admission: error=%v, allowed=%t", err, test.allowed)
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
