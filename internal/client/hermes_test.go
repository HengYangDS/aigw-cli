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
	"aigw-cli/internal/secrets"
)

func publicHermesReader(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(path, []byte("published predecessor"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestHermesInspectionKeepsIntactRetainedReader(t *testing.T) {
	for _, name := range []string{"versioned", "public"} {
		t.Run(name, func(t *testing.T) {
			retained, current := versionedReaders(t)
			if name == "public" {
				retained = publicHermesReader(t)
			}
			testHermesRetainedReader(t, retained, current)
		})
	}
}

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
	registry := builtinRegistry()
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
	receipt, _, err := registry.Apply(context.Background(), deps, cfg, after, "hermes")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".hermes", "config.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if err := receipt.Rollback(); err != nil {
		t.Fatalf("Hermes rollback failed: %v", err)
	}
	for _, target := range []string{path, path + ".aigw-state.json"} {
		if _, err := os.Stat(target); !os.IsNotExist(err) {
			t.Fatalf("Hermes rollback left %s: %v", target, err)
		}
	}
	if _, _, err := registry.Apply(context.Background(), deps, cfg, after, "hermes"); err != nil {
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
	if _, _, err := registry.Apply(context.Background(), deps, after, disabled, "hermes"); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{path, path + ".aigw-state.json"} {
		if _, err := os.Stat(target); !os.IsNotExist(err) {
			t.Fatalf("withdrawal left %s: %v", target, err)
		}
	}
}

func TestHermesProjectionTracksUnselectedProviderModels(t *testing.T) {
	before := configuration.NewConfig()
	before.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	before.Routes["selected"] = qualifiedRoute("Selected", "gateway", "gpt-6-sol", configuration.ProtocolOpenAIResponses)
	before.Routes["extra"] = qualifiedRoute("Extra", "gateway", "grok-4.6", configuration.ProtocolOpenAIResponses)
	before.SetSelectedRoute(configuration.ClientHermes, "selected", "")
	before.SetClientActivation(configuration.ClientHermes, true, "/opt/hermes", []string{"/home/test/.hermes/config.yaml"})
	before.Normalize()
	after := before.Clone()
	delete(after.Routes, "extra")
	oldRuntime, oldErr := before.ResolveRuntime(configuration.ClientHermes, "")
	newRuntime, newErr := after.ResolveRuntime(configuration.ClientHermes, "")
	if oldErr != nil || newErr != nil || oldRuntime != newRuntime {
		t.Fatalf("selected Hermes Route changed: before=%#v (%v), after=%#v (%v)", oldRuntime, oldErr, newRuntime, newErr)
	}
	if got := builtinRegistry().ChangedClients(before, after); !slices.Equal(got, []string{configuration.ClientHermes}) {
		t.Fatalf("removing an unselected provider Model affected clients %v, want Hermes", got)
	}
}

func TestHermesCatalogueUsesProviderWireModelIDs(t *testing.T) {
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Endpoints: configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	for routeID, wireID := range map[string]string{"ordinary": "gpt-6-astra", "variant": "gpt-6-astra-ssvip"} {
		cfg.Routes[routeID] = configuration.Route{
			Account: "gateway", Model: "gpt-6-astra", UpstreamModel: wireID,
			Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}},
		}
	}
	cfg.SetSelectedRoute(configuration.ClientHermes, "variant", "")
	selected, err := cfg.ResolveRuntime(configuration.ClientHermes, "")
	if err != nil {
		t.Fatal(err)
	}
	store := secrets.NewMemoryStore()
	if err := store.Set("gateway", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	desired, err := hermesDesired(Dependencies{Secrets: store, AIGWExecutable: filepath.Join(t.TempDir(), "aigw")}, cfg, selected)
	if err != nil {
		t.Fatal(err)
	}
	if desired.SelectedModel != "gpt-6-astra-ssvip" || len(desired.Providers) != 1 || !slices.Equal(desired.Providers[0].Models, []string{"gpt-6-astra", "gpt-6-astra-ssvip"}) {
		t.Fatalf("Hermes wire catalogue = %#v", desired)
	}
}

func TestHermesCatalogueFollowsDeclaredAccountAndModelOrder(t *testing.T) {
	cfg := configuration.NewConfig()
	for id, order := range map[string]int{"alpha": 20, "zulu": 10} {
		cfg.Accounts[id] = configuration.Account{Label: id, Order: order, Endpoints: configuration.Endpoints{
			Anthropic: "https://messages.test", OpenAIResponses: "https://responses.test/v1",
		}}
		for model, rank := range map[string]int{"first": 10, "second": 20} {
			cfg.Models[model] = configuration.Model{Label: model, Order: rank}
			cfg.Routes[id+"-"+model] = configuration.Route{Account: id, Model: model, UpstreamModel: map[string]string{"first": "wire-z", "second": "wire-a"}[model], Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{
				configuration.ProtocolAnthropic: {}, configuration.ProtocolOpenAIResponses: {},
			}}
		}
	}
	catalogue, err := hermesCatalogue(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, provider := range catalogue {
		got = append(got, provider.ID)
		if !slices.Equal(provider.Models, []string{"wire-z", "wire-a"}) {
			t.Fatalf("provider %s reordered exact wires lexically: %v", provider.ID, provider.Models)
		}
	}
	want := []string{"aigw-zulu-anthropic", "aigw-zulu-openai-responses", "aigw-alpha-anthropic", "aigw-alpha-openai-responses"}
	if !slices.Equal(got, want) {
		t.Fatalf("provider Account/protocol order = %v, want %v", got, want)
	}
}

func TestHermesCatalogueCoversShippedRouteWireIDs(t *testing.T) {
	team, err := os.ReadFile(filepath.Join("..", "..", "manifests", "team.toml"))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := configuration.Parse(team)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := configuration.Merge(configuration.NewConfig(), manifest)
	if err != nil {
		t.Fatal(err)
	}
	catalogue, err := hermesCatalogue(cfg)
	if err != nil {
		t.Fatal(err)
	}
	models := make(map[string][]string, len(catalogue))
	for _, provider := range catalogue {
		models[provider.ID] = provider.Models
	}
	spec := mustClientSpec(configuration.ClientHermes)
	for _, routeID := range cfg.RouteIDs() {
		route := cfg.Routes[routeID]
		for _, protocol := range spec.CompatibleRouteProtocols(cfg.Accounts[route.Account], route) {
			providerID := hermesProviderID(route.Account, protocol)
			if !slices.Contains(models[providerID], route.UpstreamModelID()) {
				t.Errorf("Hermes provider %q omits Route %q wire ID %q", providerID, routeID, route.UpstreamModelID())
			}
		}
	}
}

func TestHermesProjectionIgnoresUnselectedDisplayOnlyEdits(t *testing.T) {
	before := configuration.NewConfig()
	before.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	before.Routes["selected"] = qualifiedRoute("Selected", "gateway", "gpt-6-sol", configuration.ProtocolOpenAIResponses)
	before.Routes["extra"] = qualifiedRoute("Extra", "gateway", "grok-4.6", configuration.ProtocolOpenAIResponses)
	before.SetSelectedRoute(configuration.ClientHermes, "selected", "")
	before.SetClientActivation(configuration.ClientHermes, true, "/opt/hermes", []string{"/home/test/.hermes/config.yaml"})
	before.Normalize()
	after := before.Clone()
	route := after.Routes["extra"]
	route.Label = "Edited display label"
	route.Purpose = "Edited purpose"
	after.Routes["extra"] = route
	if got := builtinRegistry().ChangedClients(before, after); len(got) != 0 {
		t.Fatalf("display-only edit affected clients %v", got)
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
