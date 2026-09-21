package configuration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileModelProviderDefaultsAndResolvesExplicitValue(t *testing.T) {
	cfg := modelProviderConfig()
	cfg.Profiles["native"] = Profile{
		Label:   "Native",
		Account: "gateway",
		Model:   "openai.gpt-5.6-sol",
	}
	cfg.Clients[ClientCodex] = ClientBinding{
		Profile:        "native",
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
	profile := claude.Profiles["default"]
	profile.Model = "claude-fable-5"
	claude.Profiles["default"] = profile
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
		Profiles: map[string]Profile{"default": cfg.Profiles["default"]},
		Recommendations: map[string]ClientSelection{
			ClientCodex: {Profile: "default", ModelProvider: "amazon-bedrock", Authentication: AuthenticationClientNative},
		},
	}
	merged, err := MergeWithOptions(modelProviderConfig(), incoming, MergeOptions{ReplaceProfiles: map[string]bool{"default": true}})
	if err != nil {
		t.Fatal(err)
	}
	if got := merged.Recommendations[ClientCodex].ModelProvider; got != "amazon-bedrock" {
		t.Fatalf("merged model provider = %q", got)
	}
	if got := merged.Recommendations[ClientCodex].Authentication; got != AuthenticationClientNative {
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
				testCase.client: {Profile: "default", ModelProvider: testCase.provider},
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
				Profile: "default", ModelProvider: testCase.provider, Authentication: testCase.authentication,
			}
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("validation error = %v, want %q", err, testCase.want)
			}
		})
	}

	claude := modelProviderConfig()
	claude.Clients = map[string]ClientBinding{
		ClientClaude: {Profile: "default", Authentication: AuthenticationClientNative},
	}
	if err := claude.Validate(); err == nil || !strings.Contains(err.Error(), "only supported for codex") {
		t.Fatalf("Claude client-native validation error = %v", err)
	}
}

func TestSelectProfilesForConnectedAccountsHonorsRecommendationAuthentication(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["native"] = Account{
		Label:     "Native",
		Endpoints: Endpoints{OpenAIResponses: "https://native.test/v1"},
	}
	cfg.Accounts["token"] = Account{
		Label:     "Token",
		Endpoints: Endpoints{OpenAIResponses: "https://token.test/v1"},
	}
	cfg.Profiles["native"] = Profile{Label: "Native", Account: "native", Model: "native-model"}
	cfg.Profiles["token"] = Profile{Label: "Token", Account: "token", Model: "token-model"}
	cfg.Recommendations[ClientCodex] = ClientSelection{
		Profile:        "native",
		ModelProvider:  "native-provider",
		Authentication: AuthenticationClientNative,
	}

	withoutTokens, err := cfg.SelectProfilesForConnectedAccounts(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := withoutTokens.SelectedProfile(ClientCodex); got != "native" {
		t.Fatalf("selection without Tokens = %q, want client-native profile", got)
	}

	token := cfg.Clone()
	token.Recommendations[ClientCodex] = ClientSelection{
		Profile:        "token",
		ModelProvider:  "token-provider",
		Authentication: AuthenticationAccountToken,
	}
	withToken, err := token.SelectProfilesForConnectedAccounts([]string{"token"})
	if err != nil {
		t.Fatal(err)
	}
	if got := withToken.SelectedProfile(ClientCodex); got != "token" {
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
	cfg.Profiles["default"] = Profile{
		Label:   "Default",
		Account: "gateway",
		Model:   "gpt-5.6-sol",
	}
	cfg.SetSelectedProfile(ClientCodex, "default")
	return cfg
}
