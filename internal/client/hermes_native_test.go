//go:build native_hermes

package client

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"aigw-cli/internal/configuration"
	"aigw-cli/internal/platform"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"
)

// TestHermesNativeProjection consumes an explicit candidate and an installed
// Hermes Python runtime. Missing prerequisites fail rather than silently skip.
// It resolves real native configuration and executes the projected credential
// command without requesting inference or touching a user's credential store.
func TestHermesNativeProjection(t *testing.T) {
	candidate := os.Getenv("AIGW_NATIVE_CANDIDATE")
	python := os.Getenv("HERMES_NATIVE_PYTHON")
	for name, path := range map[string]string{"AIGW_NATIVE_CANDIDATE": candidate, "HERMES_NATIVE_PYTHON": python} {
		if !filepath.IsAbs(path) {
			t.Fatalf("%s must identify an absolute installed executable", name)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s prerequisite: %v", name, err)
		}
	}
	for _, protocol := range []configuration.EndpointProtocol{configuration.ProtocolAnthropic, configuration.ProtocolOpenAIResponses, configuration.ProtocolOpenAIChatCompletions} {
		t.Run(string(protocol), func(t *testing.T) {
			home := t.TempDir()
			env := map[string]string{"HOME": home, "USERPROFILE": home, "APPDATA": home, "LOCALAPPDATA": home, "XDG_CONFIG_HOME": home, "XDG_DATA_HOME": home}
			configPath, err := platform.ConfigPathFor(runtime.GOOS, env)
			if err != nil {
				t.Fatal(err)
			}
			hermesHome := filepath.Join(home, "hermes")
			cfg := configuration.NewConfig()
			cfg.Accounts["fixture"] = configuration.Account{Label: "Fixture", Endpoints: configuration.Endpoints{Anthropic: "https://provider.invalid", OpenAIResponses: "https://provider.invalid/v1", OpenAIChatCompletions: "https://provider.invalid/v1"}}
			cfg.Profiles["hermes"] = configuration.Profile{Label: "Hermes", Account: "fixture", Model: "fixture-model"}
			cfg.Clients[configuration.ClientHermes] = configuration.ClientBinding{Profile: "hermes", Enabled: true, Protocol: protocol, Executable: python, Targets: []string{filepath.Join(hermesHome, "config.yaml")}}
			if err := configuration.NewStore(configPath).Save(cfg); err != nil {
				t.Fatal(err)
			}
			deps := Dependencies{AIGWExecutable: candidate}
			if err := DefaultRegistry().Apply(t.Context(), deps, configuration.NewConfig(), cfg, configuration.ClientHermes); err != nil {
				t.Fatal(err)
			}
			env["HERMES_HOME"], env["AIGW_SECRET_BACKEND"], env["AIGW_TOKEN_FIXTURE"] = hermesHome, "env", "public-native-fixture"
			env["PATH"] = os.Getenv("PATH")
			if systemRoot := os.Getenv("SystemRoot"); systemRoot != "" {
				env["SystemRoot"] = systemRoot
			}
			environment := make([]string, 0, len(env))
			for key, value := range env {
				environment = append(environment, key+"="+value)
			}
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			output, err := (process.Runner{}).RunCapture(ctx, process.Plan{Executable: python, Directory: home, Env: environment, Args: []string{"-I", "-c", hermesNativeProbe}})
			if err != nil {
				t.Fatalf("native Hermes contract failed: %v\n%s", err, output)
			}
			var result struct {
				Model      string `json:"model"`
				Protocol   string `json:"protocol"`
				Credential bool   `json:"credential"`
				Source     string `json:"source"`
			}
			if err := json.Unmarshal(output, &result); err != nil {
				t.Fatalf("native result: %v\n%s", err, output)
			}
			want := map[configuration.EndpointProtocol]string{configuration.ProtocolAnthropic: "anthropic_messages", configuration.ProtocolOpenAIResponses: "codex_responses", configuration.ProtocolOpenAIChatCompletions: "chat_completions"}[protocol]
			if result.Model != "fixture-model" || result.Protocol != want || !result.Credential || result.Source == "" {
				t.Fatalf("native projection mismatch: %+v", result)
			}
			t.Logf("native Hermes model=%s protocol=%s credential-command=passed source=%s", result.Model, result.Protocol, result.Source)
			disabled := cfg.Clone()
			(hermesAdapter{}).Withdraw(&disabled)
			if err := DefaultRegistry().Apply(t.Context(), deps, cfg, disabled, configuration.ClientHermes); err != nil {
				t.Fatal(err)
			}
			for _, target := range []string{filepath.Join(hermesHome, "config.yaml"), filepath.Join(hermesHome, "config.yaml.aigw-state.json")} {
				if _, err := os.Stat(target); !os.IsNotExist(err) {
					t.Fatalf("native withdrawal left %s: %v", target, err)
				}
			}
		})
	}
}

// TestHermesNativeCuratedCatalog proves the real Hermes picker and runtime
// consume AIGW's provider-native model catalogue without network discovery.
func TestHermesNativeCuratedCatalog(t *testing.T) {
	candidate := os.Getenv("AIGW_NATIVE_CANDIDATE")
	python := os.Getenv("HERMES_NATIVE_PYTHON")
	for name, path := range map[string]string{"AIGW_NATIVE_CANDIDATE": candidate, "HERMES_NATIVE_PYTHON": python} {
		if !filepath.IsAbs(path) {
			t.Fatalf("%s must identify an absolute installed executable", name)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s prerequisite: %v", name, err)
		}
	}
	home := t.TempDir()
	env := map[string]string{"HOME": home, "USERPROFILE": home, "APPDATA": home, "LOCALAPPDATA": home, "XDG_CONFIG_HOME": home, "XDG_DATA_HOME": home}
	configPath, err := platform.ConfigPathFor(runtime.GOOS, env)
	if err != nil {
		t.Fatal(err)
	}
	manifestData, err := os.ReadFile(filepath.Join("..", "..", "manifests", "team.toml"))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := configuration.Parse(manifestData)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := configuration.Merge(configuration.NewConfig(), manifest)
	if err != nil {
		t.Fatal(err)
	}
	connectedAccounts := []string{"aihubmix", "ucloud"}
	cfg, err = cfg.SelectProfilesForConnectedAccounts(connectedAccounts, configuration.ClientHermes)
	if err != nil {
		t.Fatal(err)
	}
	projectionSecrets := secrets.NewMemoryStore()
	for _, accountID := range connectedAccounts {
		if err := projectionSecrets.Set(accountID, "public-native-fixture"); err != nil {
			t.Fatal(err)
		}
	}
	hermesHome := filepath.Join(home, "hermes")
	binding := cfg.Clients[configuration.ClientHermes]
	binding.Enabled = true
	binding.Executable = python
	binding.Targets = []string{filepath.Join(hermesHome, "config.yaml")}
	cfg.Clients[configuration.ClientHermes] = binding
	if err := configuration.NewStore(configPath).Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := DefaultRegistry().Apply(t.Context(), Dependencies{Secrets: projectionSecrets, AIGWExecutable: candidate}, configuration.NewConfig(), cfg, configuration.ClientHermes); err != nil {
		t.Fatal(err)
	}
	env["HERMES_HOME"], env["AIGW_SECRET_BACKEND"] = hermesHome, "env"
	for _, accountID := range connectedAccounts {
		env[secrets.EnvironmentKey(accountID)] = "public-native-fixture"
	}
	env["PATH"] = os.Getenv("PATH")
	if systemRoot := os.Getenv("SystemRoot"); systemRoot != "" {
		env["SystemRoot"] = systemRoot
	}
	environment := make([]string, 0, len(env))
	for key, value := range env {
		environment = append(environment, key+"="+value)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	output, err := (process.Runner{}).RunCapture(ctx, process.Plan{Executable: python, Directory: home, Env: environment, Args: []string{"-I", "-c", hermesNativeCatalogProbe}})
	if err != nil {
		t.Fatalf("native Hermes catalogue failed: %v\n%s", err, output)
	}
	var result struct {
		Providers  map[string][]string `json:"providers"`
		Model      string              `json:"model"`
		Protocol   string              `json:"protocol"`
		Credential bool                `json:"credential"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("native catalogue result: %v\n%s", err, output)
	}
	connected := map[string]bool{}
	for _, accountID := range connectedAccounts {
		connected[accountID] = true
	}
	want := map[string][]string{}
	for _, profile := range manifest.Profiles {
		if !connected[profile.Account] {
			continue
		}
		for _, protocol := range profile.Protocols {
			providerID := hermesProviderID(profile.Account, protocol)
			want[providerID] = append(want[providerID], profile.Model)
		}
	}
	for providerID, models := range want {
		for _, model := range models {
			if !slices.Contains(result.Providers[providerID], model) {
				t.Errorf("Hermes provider %s lacks %s: %v", providerID, model, result.Providers[providerID])
			}
		}
	}
	if result.Model != "claude-fable-5-1" || result.Protocol != "anthropic_messages" || !result.Credential {
		t.Fatalf("native selected route = %+v", result)
	}
}

const hermesNativeProbe = `
import json
from hermes_cli import runtime_provider
model = runtime_provider._get_model_config()
resolved = runtime_provider.resolve_runtime_provider(target_model=model["default"])
credential = resolved["api_key"]
assert callable(credential), "Hermes did not load the native credential command"
print(json.dumps({"model": model["default"], "protocol": resolved["api_mode"], "credential": credential() == "public-native-fixture", "source": runtime_provider.__file__}))
`

const hermesNativeCatalogProbe = `
import json
from hermes_cli import runtime_provider
from hermes_cli.config import load_config_readonly
from hermes_cli.model_switch_providers import list_authenticated_providers
cfg = load_config_readonly()
model = runtime_provider._get_model_config()
resolved = runtime_provider.resolve_runtime_provider(target_model=model["default"])
credential = resolved["api_key"]
assert callable(credential), "Hermes did not load the native credential command"
rows = list_authenticated_providers(
    current_provider=model["provider"],
    current_base_url=model["base_url"],
    current_model=model["default"],
    user_providers=cfg.get("providers"),
    probe_custom_providers=False,
    probe_current_custom_provider=False,
    for_picker=True,
)
providers = {row["slug"]: row.get("models", []) for row in rows if row.get("slug", "").startswith("aigw-")}
print(json.dumps({"providers": providers, "model": model["default"], "protocol": resolved["api_mode"], "credential": credential() == "public-native-fixture"}))
`
