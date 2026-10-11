package client

import (
	"bytes"
	"context"
	"fmt"
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
	if got := DefaultRegistry().ChangedClients(before, after); !slices.Equal(got, []string{configuration.ClientHermes}) {
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
	if got := DefaultRegistry().ChangedClients(before, after); len(got) != 0 {
		t.Fatalf("display-only edit affected clients %v", got)
	}
}

func TestHermesVerificationUsesTheOfficialSingleTurnContract(t *testing.T) {
	t.Setenv("HERMES_DISABLE_LAZY_INSTALLS", "0")
	executable := filepath.Join(t.TempDir(), "hermes")
	if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://gateway.test"}}
	cfg.Routes["hermes"] = qualifiedRoute("", "gateway", "claude-test", configuration.ProtocolAnthropic)
	cfg.Clients[configuration.ClientHermes] = configuration.ClientBinding{Route: "hermes", Enabled: true, Protocol: configuration.ProtocolAnthropic, Executable: executable}
	binding := cfg.Clients[configuration.ClientHermes]
	binding.Targets = []string{filepath.Join(t.TempDir(), "config.yaml")}
	cfg.Clients[configuration.ClientHermes] = binding
	if err := os.WriteFile(binding.Targets[0], []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	clientRuntime, err := cfg.ResolveRuntime(configuration.ClientHermes, "")
	if err != nil {
		t.Fatal(err)
	}
	store := secrets.NewMemoryStore()
	if err := store.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	profiles := filepath.Join(filepath.Dir(binding.Targets[0]), "profiles")
	for _, name := range []string{"installs", "tools"} {
		path := filepath.Join(filepath.Dir(binding.Targets[0]), name)
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "state"), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	originalEnvironment := os.Environ()
	runner := &captureAdapterRunner{outputs: [][]byte{[]byte("Hermes Agent v1\n"), []byte("AIGW_OK\n")}}
	runner.observe = func(plan process.Plan) {
		requireHermesVerificationProfile(t, plan, profiles, originalEnvironment)
	}
	if _, err := (hermesAdapter{}).Verify(context.Background(), Dependencies{Runner: runner, Secrets: store, AIGWExecutable: filepath.Join(t.TempDir(), "aigw")}, cfg, clientRuntime, ""); err != nil {
		t.Fatal(err)
	}
	want := []string{"chat", "--quiet", "--query-file", "-", "--oneshot", "--max-turns", "1", "--run-budget", "45", "--ignore-rules", "--source", "tool"}
	if len(runner.plans) != 2 || !slices.Equal(runner.plans[1].Args, want) {
		t.Fatalf("Hermes verification plans = %#v", runner.plans)
	}
	if _, err := os.Lstat(runner.plans[0].Directory); !os.IsNotExist(err) {
		t.Fatalf("isolated Hermes profile survived verification: %v", err)
	}
	for _, name := range []string{"installs", "tools"} {
		data, err := os.ReadFile(filepath.Join(filepath.Dir(binding.Targets[0]), name, "state"))
		if err != nil || string(data) != name {
			t.Fatalf("verification changed the installed %s state: %q %v", name, data, err)
		}
	}
	for _, test := range []struct {
		name, want string
		cause      error
	}{
		{"deadline", "hermes version probe timed out", context.DeadlineExceeded},
		{"cancellation", "hermes version probe interrupted", context.Canceled},
		{"execution", "hermes version probe failed", os.ErrPermission},
	} {
		t.Run(test.name, func(t *testing.T) {
			failed := &captureAdapterRunner{err: test.cause}
			_, err := (hermesAdapter{}).Verify(context.Background(), Dependencies{
				Runner: failed, Secrets: store, AIGWExecutable: filepath.Join(t.TempDir(), "aigw"),
			}, cfg, clientRuntime, "")
			if err == nil || err.Error() != test.want {
				t.Fatalf("Hermes version probe error = %v, want %q", err, test.want)
			}
		})
	}
	for _, stage := range []int{1, 2} {
		t.Run(fmt.Sprintf("diagnostic stage=%d", stage), func(t *testing.T) {
			probe := &captureAdapterRunner{outputs: [][]byte{[]byte("Hermes Agent v1\n"), []byte("AIGW_OK\n")}}
			probe.observe = func(process.Plan) {
				if probe.calls+1 == stage {
					probe.stderr = []byte("warning: secret=must-not-leak native capability incomplete\n")
				}
			}
			_, err := (hermesAdapter{}).Verify(t.Context(), Dependencies{Runner: probe, Secrets: store, AIGWExecutable: filepath.Join(t.TempDir(), "aigw")}, cfg, clientRuntime, "")
			if err == nil || !strings.Contains(err.Error(), "warning") || strings.Contains(err.Error(), "must-not-leak") {
				t.Fatalf("Hermes diagnostic result = %v", err)
			}
		})
	}
}

func requireHermesVerificationProfile(t *testing.T, plan process.Plan, profiles string, originalEnvironment []string) {
	t.Helper()
	if filepath.Dir(plan.Directory) != profiles {
		t.Fatalf("verification profile %q lost the native installation root %q", plan.Directory, profiles)
	}
	for _, name := range []string{"HOME", "USERPROFILE", "HOMEDRIVE", "HOMEPATH"} {
		matches := func(entry string) bool { return strings.HasPrefix(entry, name+"=") }
		before, after := slices.IndexFunc(originalEnvironment, matches), slices.IndexFunc(plan.Env, matches)
		if (before < 0 && after >= 0) || (before >= 0 && (after < 0 || originalEnvironment[before] != plan.Env[after])) {
			t.Fatalf("verification changed native home variable %s", name)
		}
	}
	if !slices.Contains(plan.Env, "HERMES_HOME="+plan.Directory) ||
		!slices.Contains(plan.Env, "HERMES_DISABLE_LAZY_INSTALLS=1") || slices.Contains(plan.Env, "HERMES_DISABLE_LAZY_INSTALLS=0") {
		t.Fatal("verification must isolate Hermes settings without changing the OS installation home")
	}
	for _, name := range []string{"installs", "tools"} {
		if _, err := os.Lstat(filepath.Join(plan.Directory, name)); !os.IsNotExist(err) {
			t.Fatalf("verification materialized redundant native %s state: %v", name, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(plan.Directory, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var projected struct {
		Security map[string]*bool `yaml:"security"`
		Updates  map[string]*bool `yaml:"updates"`
	}
	if err := yaml.Unmarshal(data, &projected); err != nil {
		t.Fatal(err)
	}
	if value := projected.Security["allow_lazy_installs"]; value == nil || *value {
		t.Fatal("isolated Hermes verification permits runtime dependency installation")
	}
	if value := projected.Updates["check"]; value == nil || *value {
		t.Fatal("isolated Hermes verification permits passive software update checks")
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

func TestHermesVerificationPreservesNativeModelSettings(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "hermes")
	target := filepath.Join(root, "config.yaml")
	original := []byte("agent:\n  reasoning_overrides:\n    mistral-large-3: none\nsecurity:\n  allow_lazy_installs: true\nupdates:\n  check: true\n")
	if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, original, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Endpoints: configuration.Endpoints{OpenAIChatCompletions: "https://gateway.test/v1"}}
	cfg.Routes["selected"] = qualifiedRoute("", "gateway", "mistral-large-3", configuration.ProtocolOpenAIChatCompletions)
	cfg.SetSelectedRoute(configuration.ClientHermes, "selected", "")
	cfg.SetClientActivation(configuration.ClientHermes, true, executable, []string{target})
	runtime, err := cfg.ResolveRuntime(configuration.ClientHermes, "")
	if err != nil {
		t.Fatal(err)
	}
	store := secrets.NewMemoryStore()
	if err := store.Set("gateway", "fixture-only-token"); err != nil {
		t.Fatal(err)
	}
	profiles := filepath.Join(root, "profiles")
	if err := os.Mkdir(profiles, 0o700); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(profiles, "user-profile")
	if err := os.WriteFile(foreign, []byte("user-owned"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &captureAdapterRunner{outputs: [][]byte{[]byte("Hermes fixture"), []byte("AIGW_OK")}}
	deps := Dependencies{Runner: runner, Secrets: store, AIGWExecutable: filepath.Join(root, "aigw")}
	if _, err := (hermesAdapter{}).Apply(t.Context(), deps, configuration.NewConfig(), cfg); err != nil {
		t.Fatal(err)
	}
	original, err = os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	runner.observe = func(plan process.Plan) {
		data, err := os.ReadFile(filepath.Join(plan.Directory, "config.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		var observed struct {
			Agent struct {
				ReasoningOverrides map[string]string `yaml:"reasoning_overrides"`
			} `yaml:"agent"`
			Security struct {
				AllowLazyInstalls bool `yaml:"allow_lazy_installs"`
			} `yaml:"security"`
			Updates struct {
				Check bool `yaml:"check"`
			} `yaml:"updates"`
		}
		if err := yaml.Unmarshal(data, &observed); err != nil {
			t.Fatal(err)
		}
		if got := observed.Agent.ReasoningOverrides["mistral-large-3"]; got != "none" {
			t.Fatalf("native per-model reasoning setting was lost: got %q, want none", got)
		}
		if observed.Security.AllowLazyInstalls || observed.Updates.Check {
			t.Fatal("isolated verification enables dependency or update activity")
		}
	}
	_, err = (hermesAdapter{}).Verify(t.Context(), deps, cfg, runtime, "")
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(after, original) {
		t.Fatal("verification changed the original native configuration")
	}
	runner.err = context.DeadlineExceeded
	if _, err := (hermesAdapter{}).Verify(t.Context(), deps, cfg, runtime, ""); err == nil {
		t.Fatal("failed native version probe was accepted")
	}
	entries, err := os.ReadDir(profiles)
	if err != nil || len(entries) != 1 || entries[0].Name() != filepath.Base(foreign) {
		t.Fatalf("verification leaked a profile or removed foreign state: %v, %v", entries, err)
	}
}

func TestHermesVerificationRequiresOneConfiguredHome(t *testing.T) {
	for _, targets := range [][]string{nil, {"one", "two"}} {
		cfg := configuration.NewConfig()
		cfg.Clients[configuration.ClientHermes] = configuration.ClientBinding{Enabled: true, Targets: targets}
		runner := &captureAdapterRunner{}
		_, err := (hermesAdapter{}).Verify(t.Context(), Dependencies{Runner: runner}, cfg, configuration.Runtime{}, "")
		if err == nil || !strings.Contains(err.Error(), "one configured home") || len(runner.plans) != 0 {
			t.Fatalf("ambiguous configuration home reached native verification: %v", err)
		}
	}
}
