//go:build native_hermes

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"aigw-cli/internal/configuration"
	hermesconfig "aigw-cli/internal/hermes/configuration"
	"aigw-cli/internal/platform"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"
)

// The native boot-record resolver remains real; only home-maintenance steps are
// stubbed because the synthetic install must not maintain the installed client.
func TestHermesNativeVerificationBootIsolation(t *testing.T) {
	_, python := nativeHermesExecutables(t)
	osHome := t.TempDir()
	owner, project := filepath.Join(osHome, ".hermes"), filepath.Join(osHome, "payload")
	env := map[string]string{"HOME": osHome, "USERPROFILE": osHome, "LOCALAPPDATA": osHome, "HERMES_HOME": owner, "HERMES_BOOT_TEST_ROOT": project}
	output := runHermesNativeScript(t, python, osHome, env, `
import json, os
from pathlib import Path
from hermes_cli import boot_bootstrap
from pm.environments import install_state_dir
root = Path(os.environ["HERMES_BOOT_TEST_ROOT"])
root.mkdir()
(root / "install-stamp.json").write_text(json.dumps({"commit": "abcdef012345", "updateMechanism": "external", "distribution": "nix"}))
state = install_state_dir(root)
state.mkdir(parents=True)
(state / "facts.json").write_text("{}\n")
(state / ".install.lock").touch()
(state / "environments").mkdir()
record = boot_bootstrap.record_path(root)
record.parent.mkdir()
record.write_text(json.dumps({"schemaVersion": 1, "identity": "previous-revision"}))
print(json.dumps({"record": str(record)}))
`)
	var prepared struct {
		Record string `json:"record"`
	}
	if err := json.Unmarshal(output, &prepared); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(prepared.Record)
	if err != nil {
		t.Fatal(err)
	}
	home, cleanup, err := hermesconfig.CreateVerificationHome(filepath.Join(owner, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Error(err)
		}
	})
	env["HERMES_HOME"], env["HERMES_DISABLE_LAZY_INSTALLS"] = home, "1"
	output = runHermesNativeScript(t, python, osHome, env, `
import json, os
from pathlib import Path
from hermes_cli import boot_bootstrap, post_update
root = Path(os.environ["HERMES_BOOT_TEST_ROOT"])
post_update.run_steps = lambda steps: {}
boot_bootstrap.run_boot_bootstrap(root)
record = boot_bootstrap.record_path(root)
print(json.dumps({"record": str(record), "identity": json.loads(record.read_text())["identity"]}))
`)
	var observed struct {
		Record   string `json:"record"`
		Identity string `json:"identity"`
	}
	if err := json.Unmarshal(output, &observed); err != nil {
		t.Fatal(err)
	}
	if observed.Identity != "abcdef012345" || !strings.HasPrefix(observed.Record, home+string(os.PathSeparator)) {
		t.Fatalf("verification wrote boot state outside its home: %+v", observed)
	}
	if actual, err := os.ReadFile(prepared.Record); err != nil || !bytes.Equal(actual, original) {
		t.Fatalf("operator boot record changed: %v", err)
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(observed.Record); !os.IsNotExist(err) {
		t.Fatalf("verification boot record survived cleanup: %v", err)
	}
}

func TestHermesNativeVerificationRetainsDependencyMembers(t *testing.T) {
	_, python := nativeHermesExecutables(t)
	osHome := t.TempDir()
	owner := filepath.Join(osHome, ".hermes")
	plugin := filepath.Join(owner, "plugins", "native-member")
	if err := os.MkdirAll(plugin, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plugin, "pyproject.toml"), []byte("[project]\nname = 'native-member'\nversion = '1.0.0'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(owner, "config.yaml")
	original := []byte("plugins: {enabled: [native-member]}\nproviders: {personal: {api_key: private-selection}}\n")
	if err := os.WriteFile(target, original, 0o600); err != nil {
		t.Fatal(err)
	}
	profiles := filepath.Join(owner, "profiles")
	for _, name := range []string{"work", "retired", ".work.staging-hidden", "Bad Name"} {
		path := filepath.Join(profiles, name)
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
		data := []byte("plugins: [invalid]\n")
		if name == "work" {
			data = []byte("plugins: {enabled: [native-member, disabled], disabled: [disabled]}\nmemory: {provider: memory-member}\n")
			for _, member := range []string{"native-member", "memory-member"} {
				memberRoot := filepath.Join(path, "plugins", member)
				if err := os.MkdirAll(memberRoot, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(memberRoot, "pyproject.toml"), []byte("[project]\nname = '"+member+"'\nversion = '1.0.0'\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
		}
		if err := os.WriteFile(filepath.Join(path, "config.yaml"), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(profiles, ".deleted", "retired"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(owner, filepath.Join(profiles, "default")); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"HOME": osHome, "USERPROFILE": osHome, "LOCALAPPDATA": osHome, "HERMES_HOME": owner}
	probe := `
import json
from pm.workspace import enabled_member_dirs, members_stamp
members = enabled_member_dirs()
print(json.dumps({"members": [str(p.resolve()) for p in members], "stamp": members_stamp(members)}))
`
	var before, after struct {
		Members []string `json:"members"`
		Stamp   string   `json:"stamp"`
	}
	if err := json.Unmarshal(runHermesNativeScript(t, python, osHome, env, probe), &before); err != nil {
		t.Fatal(err)
	}
	if len(before.Members) != 3 {
		t.Fatal("native dependency fixture did not select its complete live-profile union")
	}
	home, cleanup, err := hermesconfig.CreateVerificationHome(target)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Error(err)
		}
	})
	env["HERMES_HOME"] = home
	if err := json.Unmarshal(runHermesNativeScript(t, python, osHome, env, probe), &after); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(before.Members, after.Members) || before.Stamp != after.Stamp {
		t.Fatalf("isolated verification changed native dependency identity: before=%+v after=%+v", before, after)
	}
	if actual, err := os.ReadFile(target); err != nil || !bytes.Equal(actual, original) {
		t.Fatal("dependency borrowing changed the operator configuration")
	}
}

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
			cfg.Routes["hermes"] = configuration.Route{Label: "Hermes", Account: "fixture", Model: "fixture-model"}
			cfg.Clients[configuration.ClientHermes] = configuration.ClientBinding{Route: "hermes", Enabled: true, Protocol: protocol, Executable: python, Targets: []string{filepath.Join(hermesHome, "config.yaml")}}
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
	cfg, err = cfg.SelectRoutesForConnectedAccounts(connectedAccounts, configuration.ClientHermes)
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
	for _, route := range manifest.Routes {
		if !connected[route.Account] {
			continue
		}
		for _, protocol := range route.AdmittedProtocols() {
			providerID := hermesProviderID(route.Account, protocol)
			want[providerID] = append(want[providerID], route.Model)
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

func nativeHermesExecutables(t *testing.T) (candidate, python string) {
	t.Helper()
	candidate, python = os.Getenv("AIGW_NATIVE_CANDIDATE"), os.Getenv("HERMES_NATIVE_PYTHON")
	for name, path := range map[string]string{"AIGW_NATIVE_CANDIDATE": candidate, "HERMES_NATIVE_PYTHON": python} {
		if !filepath.IsAbs(path) {
			t.Fatalf("%s must identify an absolute installed executable", name)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s prerequisite: %v", name, err)
		}
	}
	return candidate, python
}

func runHermesNativeScript(t *testing.T, python, home string, env map[string]string, script string) []byte {
	t.Helper()
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
	output, err := (process.Runner{}).RunCapture(ctx, process.Plan{Executable: python, Directory: home, Env: environment, Args: []string{"-I", "-c", script}})
	if err != nil {
		t.Fatalf("native Hermes script failed: %v\n%s", err, output)
	}
	return output
}
