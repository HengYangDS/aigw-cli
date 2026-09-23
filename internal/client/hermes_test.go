package client

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"

	"go.yaml.in/yaml/v3"
)

func TestHermesLifecycleUsesItsOwnSurfaceAndDefersAbsentClient(t *testing.T) {
	home := t.TempDir()
	source := discovery.System{GOOS: runtime.GOOS, Home: home, Path: home}
	cfg := configuration.NewConfig()
	cfg.Accounts["team"] = configuration.Account{Label: "Team", Endpoints: configuration.Endpoints{Anthropic: "https://provider.test"}}
	cfg.Routes["hermes"] = qualifiedRoute("Hermes", "team", "model-test", configuration.ProtocolAnthropic)
	cfg.Clients[configuration.ClientHermes] = configuration.ClientBinding{Route: "hermes", Enabled: true, Protocol: configuration.ProtocolAnthropic}
	store := secrets.NewMemoryStore()
	if err := store.Set("team", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	deps := Dependencies{Secrets: store, AIGWExecutable: filepath.Join(home, "aigw")}
	registry := DefaultRegistry()
	observed := registry.Discover(source)
	after, err := registry.Converge(deps, cfg, observed, "hermes")
	if err != nil {
		t.Fatal(err)
	}
	if binding := after.Clients["hermes"]; !binding.Enabled || binding.Executable != "" || len(binding.Targets) != 0 {
		t.Fatalf("absent Hermes intent was not deferred: %#v", binding)
	}
	name := "hermes"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(home, name), []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	observed = registry.Discover(source)
	after, err = registry.Converge(deps, cfg, observed, "hermes")
	if err != nil {
		t.Fatal(err)
	}
	if binding := after.Clients["hermes"]; !binding.Enabled || binding.Executable == "" || len(binding.Targets) != 1 {
		t.Fatalf("installed Hermes intent was not materialized: %#v", binding)
	}
	if err := registry.Apply(context.Background(), deps, cfg, after, "hermes"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".hermes", "config.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	for _, target := range observed.AutoManagedCodexTargets() {
		if target == path {
			t.Fatal("Hermes target was admitted as a Codex home")
		}
	}
	clientRuntime, err := after.ResolveRuntime("hermes", "")
	if err != nil {
		t.Fatal(err)
	}
	status := (hermesAdapter{}).Inspect(context.Background(), deps, after, clientRuntime)
	if !status.Ready {
		t.Fatalf("Hermes status = %#v", status)
	}
	disabled := after.Clone()
	(hermesAdapter{}).Withdraw(&disabled)
	if err := registry.Apply(context.Background(), deps, after, disabled, "hermes"); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{path, path + ".aigw-state.json"} {
		if _, err := os.Stat(target); !os.IsNotExist(err) {
			t.Fatalf("withdrawal left %s: %v", target, err)
		}
	}
}

func TestHermesVerificationUsesTheOfficialSingleTurnContract(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "hermes")
	if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://gateway.test"}}
	cfg.Routes["hermes"] = qualifiedRoute("", "gateway", "claude-test", configuration.ProtocolAnthropic)
	cfg.Clients[configuration.ClientHermes] = configuration.ClientBinding{Route: "hermes", Enabled: true, Protocol: configuration.ProtocolAnthropic, Executable: executable}
	clientRuntime, err := cfg.ResolveRuntime(configuration.ClientHermes, "")
	if err != nil {
		t.Fatal(err)
	}
	store := secrets.NewMemoryStore()
	if err := store.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	runner := &captureAdapterRunner{outputs: [][]byte{[]byte("Hermes Agent v1\n"), []byte("AIGW_OK\n")}}
	runner.observe = func(plan process.Plan) {
		if len(plan.Args) == 0 || plan.Args[0] != "chat" {
			return
		}
		data, err := os.ReadFile(filepath.Join(plan.Directory, "config.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		var projected struct {
			Security struct {
				AllowLazyInstalls *bool `yaml:"allow_lazy_installs"`
			} `yaml:"security"`
		}
		if err := yaml.Unmarshal(data, &projected); err != nil {
			t.Fatal(err)
		}
		if projected.Security.AllowLazyInstalls == nil || *projected.Security.AllowLazyInstalls {
			t.Fatal("isolated Hermes verification permits runtime dependency installation")
		}
	}
	if _, err := (hermesAdapter{}).Verify(context.Background(), Dependencies{Runner: runner, Secrets: store, AIGWExecutable: filepath.Join(t.TempDir(), "aigw")}, cfg, clientRuntime, ""); err != nil {
		t.Fatal(err)
	}
	want := []string{"chat", "--quiet", "--query-file", "-", "--oneshot", "--max-turns", "1", "--run-budget", "45", "--ignore-rules", "--source", "tool"}
	if len(runner.plans) != 2 || !slices.Equal(runner.plans[1].Args, want) {
		t.Fatalf("Hermes verification plans = %#v", runner.plans)
	}
}

func TestHermesProjectionGroupsEveryConnectedAccountsCuratedModelsByProtocol(t *testing.T) {
	home := t.TempDir()
	target := filepath.Join(home, ".hermes", "config.yaml")
	cfg := configuration.NewConfig()
	cfg.Accounts["team"] = configuration.Account{Label: "Team", Endpoints: configuration.Endpoints{
		Anthropic:             "https://messages.test",
		OpenAIResponses:       "https://responses.test/v1",
		OpenAIChatCompletions: "https://chat.test/v1",
	}}
	cfg.Accounts["connected"] = configuration.Account{Label: "Connected", Endpoints: configuration.Endpoints{
		OpenAIResponses: "https://connected.test/v1",
	}}
	cfg.Accounts["offline"] = configuration.Account{Label: "Offline", Endpoints: configuration.Endpoints{
		OpenAIResponses: "https://offline.test/v1",
	}}
	cfg.Routes["claude"] = configuration.Route{Label: "Claude", Account: "team", Model: "claude-fable-5-1", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}}}
	cfg.Routes["grok"] = configuration.Route{Label: "Grok", Account: "team", Model: "grok-4.6", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}}}
	cfg.Routes["gemini"] = configuration.Route{Label: "Gemini", Account: "team", Model: "gemini-3.8-flash", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIChatCompletions: {}}}
	cfg.Routes["qwen"] = configuration.Route{Label: "Qwen", Account: "connected", Model: "qwen-max", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}}}
	cfg.Routes["deepseek"] = configuration.Route{Label: "DeepSeek", Account: "offline", Model: "deepseek-v3", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}}}
	cfg.Clients[configuration.ClientHermes] = configuration.ClientBinding{Route: "claude", Enabled: true, Protocol: configuration.ProtocolAnthropic, Targets: []string{target}}
	store := secrets.NewMemoryStore()
	for _, accountID := range []string{"team", "connected"} {
		if err := store.Set(accountID, "fixture-token"); err != nil {
			t.Fatal(err)
		}
	}

	plans, _, err := hermesPlans(Dependencies{Secrets: store, AIGWExecutable: filepath.Join(home, "aigw")}, configuration.NewConfig(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 {
		t.Fatalf("Hermes plans = %d", len(plans))
	}
	if _, err := plans[0].Apply(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"aigw-team-anthropic", "aigw-team-openai-responses", "aigw-team-openai-chat-completions", "aigw-connected-openai-responses", "claude-fable-5-1", "grok-4.6", "gemini-3.8-flash", "qwen-max", "discover_models: false"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("Hermes projection missing %q:\n%s", want, data)
		}
	}
	for _, unwanted := range []string{"aigw-offline-openai-responses", "deepseek-v3"} {
		if strings.Contains(string(data), unwanted) {
			t.Errorf("Hermes projection contains disconnected Account value %q:\n%s", unwanted, data)
		}
	}
}
