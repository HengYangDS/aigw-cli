package store

import (
	"aigw-cli/internal/domain"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRouteModelProviderDefaultsAndResolvesExplicitValue(t *testing.T) {
	cfg := modelProviderConfig()
	cfg.Models["native"] = domain.Model{
		Label:    "Native",
		Provider: "gateway",
		Family:   "openai.gpt-5.6-sol",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{
			domain.ProtocolOpenAIResponses: {},
		},
	}
	cfg.Clients[ClientCodex] = domain.ClientBinding{
		Model:          "native",
		ModelProvider:  "amazon-bedrock",
		Authentication: domain.AuthenticationClientNative,
	}

	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	runtime, err := cfg.ResolveRuntime(ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if runtime.ModelProvider != "amazon-bedrock" {
		t.Fatalf("model provider = %q", runtime.ModelProvider)
	}
	if runtime.Authentication != domain.AuthenticationClientNative || runtime.RequiresProviderToken() {
		t.Fatalf("authentication = %q, requires Token = %t", runtime.Authentication, runtime.RequiresProviderToken())
	}

	runtime, err = cfg.ResolveRuntime(ClientCodex, "default")
	if err != nil {
		t.Fatal(err)
	}
	if runtime.ModelProvider != "aigw" {
		t.Fatalf("default model provider = %q", runtime.ModelProvider)
	}
	if runtime.Authentication != domain.AuthenticationProviderToken || !runtime.RequiresProviderToken() {
		t.Fatalf("default authentication = %q, requires Token = %t", runtime.Authentication, runtime.RequiresProviderToken())
	}

	claude := modelProviderConfig()
	route := claude.Models["default"]
	route.Family = "claude-fable-5"
	claude.Models["default"] = route
	runtime, err = claude.ResolveRuntime(ClientClaude, "default")
	if err != nil {
		t.Fatal(err)
	}
	if runtime.ModelProvider != "" {
		t.Fatalf("Claude runtime inherited Codex model provider %q", runtime.ModelProvider)
	}
}

func TestClientModelProviderPersistsAndParticipatesInManifestSelection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := modelProviderConfig()
	binding := cfg.Clients[ClientCodex]
	binding.ModelProvider = "amazon-bedrock"
	binding.Authentication = domain.AuthenticationClientNative
	cfg.Clients[ClientCodex] = binding

	store := NewStore(path, testAdmission())
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `model_provider = 'amazon-bedrock'`) || !strings.Contains(string(data), `authentication = 'client-native'`) {
		t.Fatalf("persisted config lacks native provider authentication:\n%s", data)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Clients[ClientCodex].ModelProvider; got != "amazon-bedrock" {
		t.Fatalf("loaded model provider = %q", got)
	}
	if got := loaded.Clients[ClientCodex].Authentication; got != domain.AuthenticationClientNative {
		t.Fatalf("loaded authentication = %q", got)
	}

	incoming := domain.Manifest{
		Version:   domain.ManifestVersion,
		Providers: map[string]domain.Provider{"gateway": cfg.Providers["gateway"]},
		Models:    map[string]domain.Model{"default": cfg.Models["default"]},
		Recommendations: map[string]domain.ClientRecommendation{
			ClientCodex: {Primary: domain.ClientSelection{Model: "default", ModelProvider: "amazon-bedrock", Authentication: domain.AuthenticationClientNative}},
		},
	}
	merged, err := domain.MergeWithOptions(modelProviderConfig(), incoming, domain.MergeOptions{ReplaceModels: map[string]bool{"default": true}})
	if err != nil {
		t.Fatal(err)
	}
	if got := merged.Recommendations[ClientCodex].Primary.ModelProvider; got != "amazon-bedrock" {
		t.Fatalf("merged model provider = %q", got)
	}
	if got := merged.Recommendations[ClientCodex].Primary.Authentication; got != domain.AuthenticationClientNative {
		t.Fatalf("merged authentication = %q", got)
	}
}

func TestClientBindingModelProviderRejectsUnsafeOrNonCodexValues(t *testing.T) {
	for name, testCase := range map[string]struct {
		client   string
		provider string
		want     string
	}{
		"unsafe punctuation": {ClientCodex, "bad.provider", "invalid model provider"},
		"empty component":    {ClientCodex, "-provider", "invalid model provider"},
		"claude scope":       {ClientClaude, "amazon-bedrock", "only supported for codex"},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := modelProviderConfig()
			cfg.Clients = map[string]domain.ClientBinding{
				testCase.client: {Model: "default", ModelProvider: testCase.provider},
			}
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("validation error = %v, want %q", err, testCase.want)
			}
		})
	}
}

func TestClientBindingAuthenticationRejectsInvalidValuesAndClientNativeWithoutProvider(t *testing.T) {
	for name, testCase := range map[string]struct {
		authentication domain.Authentication
		provider       string
		want           string
	}{
		"unknown mode":                 {"magic", "amazon-bedrock", "invalid authentication"},
		"native mode without provider": {domain.AuthenticationClientNative, "", "requires model_provider"},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := modelProviderConfig()
			cfg.Clients[ClientCodex] = domain.ClientBinding{
				Model: "default", ModelProvider: testCase.provider, Authentication: testCase.authentication,
			}
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("validation error = %v, want %q", err, testCase.want)
			}
		})
	}

	claude := modelProviderConfig()
	claude.Clients = map[string]domain.ClientBinding{
		ClientClaude: {Model: "default", Authentication: domain.AuthenticationClientNative},
	}
	if err := claude.Validate(); err == nil || !strings.Contains(err.Error(), "only supported for codex") {
		t.Fatalf("Claude client-native validation error = %v", err)
	}
}

func TestSelectModelsForConnectedProvidersHonorsRecommendationAuthentication(t *testing.T) {
	cfg := domain.NewConfig(testAdmission())
	cfg.Providers["native"] = domain.Provider{
		Label:     "Native",
		Endpoints: domain.Endpoints{OpenAIResponses: "https://native.test/v1"},
	}
	cfg.Providers["token"] = domain.Provider{
		Label:     "Token",
		Endpoints: domain.Endpoints{OpenAIResponses: "https://token.test/v1"},
	}
	cfg.Models["native"] = testModel("Native", "native", "native-model", domain.ProtocolOpenAIResponses)
	cfg.Models["token"] = testModel("Token", "token", "token-model", domain.ProtocolOpenAIResponses)
	cfg.Recommendations[ClientCodex] = domain.ClientRecommendation{Primary: domain.ClientSelection{
		Model:          "native",
		ModelProvider:  "native-provider",
		Authentication: domain.AuthenticationClientNative,
	}}

	withoutTokens, err := cfg.SelectModelsForConnectedProviders(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := withoutTokens.SelectedModel(ClientCodex); got != "native" {
		t.Fatalf("selection without Tokens = %q, want client-native route", got)
	}

	token := cfg.Clone()
	token.Recommendations[ClientCodex] = domain.ClientRecommendation{Primary: domain.ClientSelection{
		Model:          "token",
		ModelProvider:  "token-provider",
		Authentication: domain.AuthenticationProviderToken,
	}}
	withToken, err := token.SelectModelsForConnectedProviders([]string{"token"})
	if err != nil {
		t.Fatal(err)
	}
	if got := withToken.SelectedModel(ClientCodex); got != "token" {
		t.Fatalf("selection with connected Token = %q, want account-token route", got)
	}
}

func TestValidateKeepsUnselectedBindingsWithinTheDeclarationContract(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*domain.Config)
		want string
	}{
		{"unselected client", func(c *domain.Config) { c.Clients[ClientCodex] = domain.ClientBinding{} }, ""},
		{"relative reader without route", func(c *domain.Config) { c.Clients[ClientCodex] = domain.ClientBinding{CredentialCommand: "reader"} }, "one absolute executable path"},
		{"empty recommendation", func(c *domain.Config) { c.Recommendations[ClientCodex] = domain.ClientRecommendation{} }, "has no primary route"},
		{"invalid canonical model", func(c *domain.Config) { c.Families["bad model"] = domain.Family{Label: "Invalid"} }, "invalid model name"},
		{"unknown canonical model", func(c *domain.Config) {
			route := c.Models["default"]
			route.Family = "unknown"
			c.Models["default"] = route
		}, "references unknown model"},
		{"blank wire model", func(c *domain.Config) {
			route := c.Models["default"]
			route.UpstreamModel = " "
			c.Models["default"] = route
		}, "empty upstream model"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := modelProviderConfig()
			cfg.Normalize()
			test.edit(&cfg)
			before := cfg.Clone()
			err := cfg.Validate()
			if test.want == "" && err != nil || test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)) {
				t.Fatalf("declaration error = %v; want %q", err, test.want)
			}
			if !reflect.DeepEqual(cfg, before) {
				t.Fatal("declaration validation changed the input")
			}
		})
	}
}

func modelProviderConfig() domain.Config {
	cfg := domain.NewConfig(testAdmission())
	cfg.Providers["gateway"] = domain.Provider{
		Label: "Gateway",
		Endpoints: domain.Endpoints{
			OpenAIResponses: "https://gateway.test/openai/v1",
			Anthropic:       "https://gateway.test/anthropic",
		},
	}
	cfg.Models["default"] = domain.Model{
		Label:    "Default",
		Provider: "gateway",
		Family:   "gpt-5.6-sol",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{
			domain.ProtocolOpenAIResponses: {},
			domain.ProtocolAnthropic:       {},
		},
	}
	cfg.SetSelectedModel(ClientCodex, "default", "")
	return cfg
}
