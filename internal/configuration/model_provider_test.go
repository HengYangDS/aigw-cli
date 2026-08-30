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
		Label:         "Native",
		Account:       "gateway",
		Client:        ClientCodex,
		ModelProvider: "amazon-bedrock",
		Model:         "openai.gpt-5.6-sol",
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
	cfg.Profiles["default"] = profile

	store := NewStore(path)
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `model_provider = 'amazon-bedrock'`) {
		t.Fatalf("persisted config lacks model_provider:\n%s", data)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Profiles["default"].ModelProvider; got != "amazon-bedrock" {
		t.Fatalf("loaded model provider = %q", got)
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
