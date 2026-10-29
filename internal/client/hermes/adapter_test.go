package hermes

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/client"
	"aigw-cli/internal/client/codex"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
)

// hermesRegistry registers only this adapter, so it admits only Hermes.
func hermesRegistry() client.Registry {
	registry, err := client.NewRegistry(New())
	if err != nil {
		panic(err)
	}
	return registry
}

func publicHermesReader(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(path, []byte("published predecessor"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestHomeSurfaceIdentityIsStable(t *testing.T) {
	if got := string(HomeSurface); got != "hermes-home-default" {
		t.Fatalf("Hermes home surface = %q", got)
	}
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
	cfg := domain.NewConfig(testAdmission())
	cfg.Accounts["team"] = domain.Account{Label: "Team", Endpoints: domain.Endpoints{Anthropic: "https://provider.test"}}
	cfg.Routes["hermes"] = qualifiedRoute("Hermes", "team", "model-test", domain.ProtocolAnthropic)
	cfg.Clients[ID] = domain.ClientBinding{Route: "hermes", Enabled: true, Protocol: domain.ProtocolAnthropic}
	store := secret.NewMemoryStore()
	if err := store.Set("team", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	deps := client.Dependencies{Secrets: store, AIGWExecutable: filepath.Join(home, "aigw")}
	registry := hermesRegistry()
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
	for _, target := range codex.AutoManagedTargets(observed) {
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
	before := domain.NewConfig(testAdmission())
	before.Accounts["gateway"] = domain.Account{Label: "Gateway", Endpoints: domain.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	before.Routes["selected"] = qualifiedRoute("Selected", "gateway", "gpt-6-sol", domain.ProtocolOpenAIResponses)
	before.Routes["extra"] = qualifiedRoute("Extra", "gateway", "grok-4.6", domain.ProtocolOpenAIResponses)
	before.SetSelectedRoute(ID, "selected", "")
	before.SetClientActivation(ID, true, "/opt/hermes", []string{"/home/test/.hermes/config.yaml"})
	before.Normalize()
	after := before.Clone()
	delete(after.Routes, "extra")
	oldRuntime, oldErr := before.ResolveRuntime(ID, "")
	newRuntime, newErr := after.ResolveRuntime(ID, "")
	if oldErr != nil || newErr != nil || oldRuntime != newRuntime {
		t.Fatalf("selected Hermes Route changed: before=%#v (%v), after=%#v (%v)", oldRuntime, oldErr, newRuntime, newErr)
	}
	if got := hermesRegistry().ChangedClients(before, after); !slices.Equal(got, []string{ID}) {
		t.Fatalf("removing an unselected provider Model affected clients %v, want Hermes", got)
	}
}

func TestHermesCatalogueUsesProviderWireModelIDs(t *testing.T) {
	cfg := domain.NewConfig(testAdmission())
	cfg.Accounts["gateway"] = domain.Account{Endpoints: domain.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	for routeID, wireID := range map[string]string{"ordinary": "gpt-6-astra", "variant": "gpt-6-astra-ssvip"} {
		cfg.Routes[routeID] = domain.Route{
			Account: "gateway", Model: "gpt-6-astra", UpstreamModel: wireID,
			Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}},
		}
	}
	cfg.SetSelectedRoute(ID, "variant", "")
	selected, err := cfg.ResolveRuntime(ID, "")
	if err != nil {
		t.Fatal(err)
	}
	store := secret.NewMemoryStore()
	if err := store.Set("gateway", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	desired, err := hermesDesired(client.Dependencies{Secrets: store, AIGWExecutable: filepath.Join(t.TempDir(), "aigw")}, cfg, selected)
	if err != nil {
		t.Fatal(err)
	}
	if desired.SelectedModel != "gpt-6-astra-ssvip" || len(desired.Providers) != 1 || !slices.Equal(desired.Providers[0].Models, []string{"gpt-6-astra", "gpt-6-astra-ssvip"}) {
		t.Fatalf("Hermes wire catalogue = %#v", desired)
	}
}

func TestHermesCatalogueFollowsDeclaredAccountAndModelOrder(t *testing.T) {
	cfg := domain.NewConfig(testAdmission())
	for id, order := range map[string]int{"alpha": 20, "zulu": 10} {
		cfg.Accounts[id] = domain.Account{Label: id, Order: order, Endpoints: domain.Endpoints{
			Anthropic: "https://messages.test", OpenAIResponses: "https://responses.test/v1",
		}}
		for model, rank := range map[string]int{"first": 10, "second": 20} {
			cfg.Models[model] = domain.Model{Label: model, Order: rank}
			cfg.Routes[id+"-"+model] = domain.Route{Account: id, Model: model, UpstreamModel: map[string]string{"first": "wire-z", "second": "wire-a"}[model], Interfaces: map[domain.EndpointProtocol][]domain.Capability{
				domain.ProtocolAnthropic: {}, domain.ProtocolOpenAIResponses: {},
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
	team, err := os.ReadFile(filepath.Join("..", "..", "..", "manifests", "team.toml"))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := store.Parse(team, builtInAdmission())
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := domain.Merge(domain.NewConfig(builtInAdmission()), manifest)
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
	spec := New().Spec()
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
	before := domain.NewConfig(testAdmission())
	before.Accounts["gateway"] = domain.Account{Label: "Gateway", Endpoints: domain.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	before.Routes["selected"] = qualifiedRoute("Selected", "gateway", "gpt-6-sol", domain.ProtocolOpenAIResponses)
	before.Routes["extra"] = qualifiedRoute("Extra", "gateway", "grok-4.6", domain.ProtocolOpenAIResponses)
	before.SetSelectedRoute(ID, "selected", "")
	before.SetClientActivation(ID, true, "/opt/hermes", []string{"/home/test/.hermes/config.yaml"})
	before.Normalize()
	after := before.Clone()
	route := after.Routes["extra"]
	route.Label = "Edited display label"
	route.Purpose = "Edited purpose"
	after.Routes["extra"] = route
	if got := hermesRegistry().ChangedClients(before, after); len(got) != 0 {
		t.Fatalf("display-only edit affected clients %v", got)
	}
}

func TestHermesProjectionGroupsEveryConnectedAccountsCuratedModelsByProtocol(t *testing.T) {
	home := t.TempDir()
	target := filepath.Join(home, ".hermes", "config.yaml")
	cfg := domain.NewConfig(testAdmission())
	cfg.Accounts["team"] = domain.Account{Label: "Team", Endpoints: domain.Endpoints{
		Anthropic:             "https://messages.test",
		OpenAIResponses:       "https://responses.test/v1",
		OpenAIChatCompletions: "https://chat.test/v1",
	}}
	cfg.Accounts["connected"] = domain.Account{Label: "Connected", Endpoints: domain.Endpoints{
		OpenAIResponses: "https://connected.test/v1",
	}}
	cfg.Accounts["offline"] = domain.Account{Label: "Offline", Endpoints: domain.Endpoints{
		OpenAIResponses: "https://offline.test/v1",
	}}
	cfg.Routes["claude"] = domain.Route{Label: "Claude", Account: "team", Model: "claude-fable-5-1", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.Routes["grok"] = domain.Route{Label: "Grok", Account: "team", Model: "grok-4.6", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	cfg.Routes["gemini"] = domain.Route{Label: "Gemini", Account: "team", Model: "gemini-3.8-flash", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIChatCompletions: {}}}
	cfg.Routes["qwen"] = domain.Route{Label: "Qwen", Account: "connected", Model: "qwen-max", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	cfg.Routes["deepseek"] = domain.Route{Label: "DeepSeek", Account: "offline", Model: "deepseek-v3", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	cfg.Clients[ID] = domain.ClientBinding{Route: "claude", Enabled: true, Protocol: domain.ProtocolAnthropic, Targets: []string{target}}
	store := secret.NewMemoryStore()
	for _, accountID := range []string{"team", "connected"} {
		if err := store.Set(accountID, "fixture-token"); err != nil {
			t.Fatal(err)
		}
	}

	plans, _, err := hermesPlans(client.Dependencies{Secrets: store, AIGWExecutable: filepath.Join(home, "aigw")}, domain.NewConfig(testAdmission()), cfg)
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

func qualifiedRoute(label, account, model string, protocols ...domain.EndpointProtocol) domain.Route {
	interfaces := make(map[domain.EndpointProtocol][]domain.Capability, len(protocols))
	for _, protocol := range protocols {
		interfaces[protocol] = []domain.Capability{}
	}
	return domain.Route{
		Label: label, Account: account, Model: model, UpstreamModel: model, Interfaces: interfaces,
	}
}

type captureAdapterRunner struct {
	err       error
	calls     int
	deadlines []bool
	plans     []process.Plan
	outputs   [][]byte
	stderr    []byte
	observe   func(process.Plan)
}

func (runner *captureAdapterRunner) RunCaptureStreams(ctx context.Context, plan process.Plan) ([]byte, []byte, error) {
	output, err := runner.RunCapture(ctx, plan)
	if err != nil {
		return nil, output, err
	}
	return output, runner.stderr, nil
}

func (runner *captureAdapterRunner) RunCapture(ctx context.Context, plan process.Plan) ([]byte, error) {
	if runner.observe != nil {
		runner.observe(plan)
	}
	runner.plans = append(runner.plans, plan)
	runner.calls++
	_, hasDeadline := ctx.Deadline()
	runner.deadlines = append(runner.deadlines, hasDeadline)
	if runner.calls <= len(runner.outputs) {
		return runner.outputs[runner.calls-1], runner.err
	}
	return nil, runner.err
}
