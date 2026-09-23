package configuration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileModelProviderDefaultsAndResolvesExplicitValue(t *testing.T) {
	cfg := modelProviderConfig()
	cfg.Routes["native"] = Route{
		Label:   "Native",
		Account: "gateway",
		Model:   "openai.gpt-5.6-sol",
		Interfaces: map[EndpointProtocol][]Capability{
			ProtocolOpenAIResponses: {},
		},
	}
	cfg.Clients[ClientCodex] = ClientBinding{
		Route:          "native",
		ModelProvider:  "amazon-bedrock",
		Authentication: AuthenticationClientNative,
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
	if runtime.Authentication != AuthenticationClientNative || runtime.RequiresAccountToken() {
		t.Fatalf("authentication = %q, requires Token = %t", runtime.Authentication, runtime.RequiresAccountToken())
	}

	runtime, err = cfg.ResolveRuntime(ClientCodex, "default")
	if err != nil {
		t.Fatal(err)
	}
	if runtime.ModelProvider != ModelProviderAIGW {
		t.Fatalf("default model provider = %q", runtime.ModelProvider)
	}
	if runtime.Authentication != AuthenticationAccountToken || !runtime.RequiresAccountToken() {
		t.Fatalf("default authentication = %q, requires Token = %t", runtime.Authentication, runtime.RequiresAccountToken())
	}

	claude := modelProviderConfig()
	profile := claude.Routes["default"]
	profile.Model = "claude-fable-5"
	claude.Routes["default"] = profile
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
	binding.Authentication = AuthenticationClientNative
	cfg.Clients[ClientCodex] = binding

	store := NewStore(path)
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
	if got := loaded.Clients[ClientCodex].Authentication; got != AuthenticationClientNative {
		t.Fatalf("loaded authentication = %q", got)
	}

	incoming := Manifest{
		Version:  currentVersion,
		Accounts: map[string]Account{"gateway": cfg.Accounts["gateway"]},
		Routes:   map[string]Route{"default": cfg.Routes["default"]},
		Recommendations: map[string]ClientRecommendation{
			ClientCodex: {Primary: ClientSelection{Route: "default", ModelProvider: "amazon-bedrock", Authentication: AuthenticationClientNative}},
		},
	}
	merged, err := MergeWithOptions(modelProviderConfig(), incoming, MergeOptions{ReplaceRoutes: map[string]bool{"default": true}})
	if err != nil {
		t.Fatal(err)
	}
	if got := merged.Recommendations[ClientCodex].Primary.ModelProvider; got != "amazon-bedrock" {
		t.Fatalf("merged model provider = %q", got)
	}
	if got := merged.Recommendations[ClientCodex].Primary.Authentication; got != AuthenticationClientNative {
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
			cfg.Clients = map[string]ClientBinding{
				testCase.client: {Route: "default", ModelProvider: testCase.provider},
			}
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("validation error = %v, want %q", err, testCase.want)
			}
		})
	}
}

func TestClientBindingAuthenticationRejectsInvalidValuesAndClientNativeWithoutProvider(t *testing.T) {
	for name, testCase := range map[string]struct {
		authentication Authentication
		provider       string
		want           string
	}{
		"unknown mode":                 {"magic", "amazon-bedrock", "invalid authentication"},
		"native mode without provider": {AuthenticationClientNative, "", "requires model_provider"},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := modelProviderConfig()
			cfg.Clients[ClientCodex] = ClientBinding{
				Route: "default", ModelProvider: testCase.provider, Authentication: testCase.authentication,
			}
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("validation error = %v, want %q", err, testCase.want)
			}
		})
	}

	claude := modelProviderConfig()
	claude.Clients = map[string]ClientBinding{
		ClientClaude: {Route: "default", Authentication: AuthenticationClientNative},
	}
	if err := claude.Validate(); err == nil || !strings.Contains(err.Error(), "only supported for codex") {
		t.Fatalf("Claude client-native validation error = %v", err)
	}
}

func TestSelectRoutesForConnectedAccountsHonorsRecommendationAuthentication(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["native"] = Account{
		Label:     "Native",
		Endpoints: Endpoints{OpenAIResponses: "https://native.test/v1"},
	}
	cfg.Accounts["token"] = Account{
		Label:     "Token",
		Endpoints: Endpoints{OpenAIResponses: "https://token.test/v1"},
	}
	cfg.Routes["native"] = testRoute("Native", "native", "native-model", ProtocolOpenAIResponses)
	cfg.Routes["token"] = testRoute("Token", "token", "token-model", ProtocolOpenAIResponses)
	cfg.Recommendations[ClientCodex] = ClientRecommendation{Primary: ClientSelection{
		Route:          "native",
		ModelProvider:  "native-provider",
		Authentication: AuthenticationClientNative,
	}}

	withoutTokens, err := cfg.SelectRoutesForConnectedAccounts(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := withoutTokens.SelectedRoute(ClientCodex); got != "native" {
		t.Fatalf("selection without Tokens = %q, want client-native profile", got)
	}

	token := cfg.Clone()
	token.Recommendations[ClientCodex] = ClientRecommendation{Primary: ClientSelection{
		Route:          "token",
		ModelProvider:  "token-provider",
		Authentication: AuthenticationAccountToken,
	}}
	withToken, err := token.SelectRoutesForConnectedAccounts([]string{"token"})
	if err != nil {
		t.Fatal(err)
	}
	if got := withToken.SelectedRoute(ClientCodex); got != "token" {
		t.Fatalf("selection with connected Token = %q, want account-token profile", got)
	}
}

func modelProviderConfig() Config {
	cfg := NewConfig()
	cfg.Accounts["gateway"] = Account{
		Label: "Gateway",
		Endpoints: Endpoints{
			OpenAIResponses: "https://gateway.test/openai/v1",
			Anthropic:       "https://gateway.test/anthropic",
		},
	}
	cfg.Routes["default"] = Route{
		Label:   "Default",
		Account: "gateway",
		Model:   "gpt-5.6-sol",
		Interfaces: map[EndpointProtocol][]Capability{
			ProtocolOpenAIResponses: {},
			ProtocolAnthropic:       {},
		},
	}
	cfg.SetSelectedRoute(ClientCodex, "default")
	return cfg
}
