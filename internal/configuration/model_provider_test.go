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
		Label:          "Native",
		Account:        "gateway",
		Client:         ClientCodex,
		ModelProvider:  "amazon-bedrock",
		Authentication: AuthenticationClientNative,
		Model:          "openai.gpt-5.6-sol",
	}
	cfg.Routes[ClientCodex] = "native"

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

	profile := cfg.Profiles["default"]
	profile.ModelProvider = ""
	cfg.Profiles["default"] = profile
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
	profile = claude.Profiles["default"]
	profile.Client = ClientClaude
	profile.Model = "claude-fable-5"
	profile.ModelProvider = ""
	claude.Profiles["default"] = profile
	runtime, err = claude.ResolveRuntime(ClientClaude, "default")
	if err != nil {
		t.Fatal(err)
	}
	if runtime.ModelProvider != "" {
		t.Fatalf("Claude runtime inherited Codex model provider %q", runtime.ModelProvider)
	}
}

func TestProfileModelProviderPersistsAndParticipatesInManifestEquality(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := modelProviderConfig()
	profile := cfg.Profiles["default"]
	profile.ModelProvider = "amazon-bedrock"
	profile.Authentication = AuthenticationClientNative
	cfg.Profiles["default"] = profile

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
	if got := loaded.Profiles["default"].ModelProvider; got != "amazon-bedrock" {
		t.Fatalf("loaded model provider = %q", got)
	}
	if got := loaded.Profiles["default"].Authentication; got != AuthenticationClientNative {
		t.Fatalf("loaded authentication = %q", got)
	}

	incoming := Manifest{
		Version:  currentVersion,
		Accounts: map[string]Account{"gateway": cfg.Accounts["gateway"]},
		Profiles: map[string]Profile{"default": profile},
	}
	merged, err := MergeWithOptions(modelProviderConfig(), incoming, MergeOptions{ReplaceProfiles: map[string]bool{"default": true}})
	if err != nil {
		t.Fatal(err)
	}
	if got := merged.Profiles["default"].ModelProvider; got != "amazon-bedrock" {
		t.Fatalf("merged model provider = %q", got)
	}
	if got := merged.Profiles["default"].Authentication; got != AuthenticationClientNative {
		t.Fatalf("merged authentication = %q", got)
	}
}

func TestProfileModelProviderRejectsUnsafeOrNonCodexValues(t *testing.T) {
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
			profile := cfg.Profiles["default"]
			profile.Client = testCase.client
			profile.ModelProvider = testCase.provider
			profile.Model = "model"
			cfg.Profiles["default"] = profile
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("validation error = %v, want %q", err, testCase.want)
			}
		})
	}
}

func TestProfileAuthenticationRejectsInvalidValuesAndClientNativeWithoutProvider(t *testing.T) {
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
			profile := cfg.Profiles["default"]
			profile.Authentication = testCase.authentication
			profile.ModelProvider = testCase.provider
			cfg.Profiles["default"] = profile
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("validation error = %v, want %q", err, testCase.want)
			}
		})
	}

	claude := modelProviderConfig()
	profile := claude.Profiles["default"]
	profile.Client = ClientClaude
	profile.ModelProvider = ""
	profile.Authentication = AuthenticationClientNative
	claude.Profiles["default"] = profile
	if err := claude.Validate(); err == nil || !strings.Contains(err.Error(), "only supported for codex") {
		t.Fatalf("Claude client-native validation error = %v", err)
	}
}

func TestSelectRoutesForConnectedAccountsTreatsClientNativeProfilesAsReadyWithoutTokens(t *testing.T) {
	cfg := NewConfig()
	cfg.Accounts["native"] = Account{
		Label:     "Native",
		Endpoints: Endpoints{OpenAIResponses: "https://native.test/v1"},
	}
	cfg.Accounts["token"] = Account{
		Label:     "Token",
		Endpoints: Endpoints{OpenAIResponses: "https://token.test/v1"},
	}
	cfg.Profiles["native"] = Profile{
		Label:          "Native",
		Account:        "native",
		Client:         ClientCodex,
		Model:          "native-model",
		ModelProvider:  "native-provider",
		Authentication: AuthenticationClientNative,
	}
	cfg.Profiles["token"] = Profile{
		Label:          "Token",
		Account:        "token",
		Client:         ClientCodex,
		Model:          "token-model",
		ModelProvider:  "token-provider",
		Authentication: AuthenticationAccountToken,
	}
	cfg.Routes[ClientCodex] = "token"

	withoutTokens, err := cfg.SelectRoutesForConnectedAccounts(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := withoutTokens.Routes[ClientCodex]; got != "native" {
		t.Fatalf("route without Tokens = %q, want client-native profile", got)
	}

	withToken, err := cfg.SelectRoutesForConnectedAccounts([]string{"token"})
	if err != nil {
		t.Fatal(err)
	}
	if got := withToken.Routes[ClientCodex]; got != "token" {
		t.Fatalf("route with connected Token = %q, want selected account-token profile", got)
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
		Client:  ClientCodex,
		Model:   "gpt-5.6-sol",
	}
	cfg.Routes[ClientCodex] = "default"
	return cfg
}
