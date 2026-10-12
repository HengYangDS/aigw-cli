package client

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	hermesconfig "aigw-cli/internal/hermes/configuration"
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

func TestHermesProjectionTracksUnselectedProviderModels(t *testing.T) {
	before := configuration.NewConfig()
	before.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	before.Routes["selected"] = qualifiedRoute("Selected", "gateway", "gpt-6-sol", configuration.ProtocolOpenAIResponses)
	before.Routes["extra"] = qualifiedRoute("Extra", "gateway", "grok-4.6", configuration.ProtocolOpenAIResponses)
	before.SetSelectedRoute(configuration.ClientHermes, "selected")
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

func TestHermesProjectionIgnoresUnselectedDisplayOnlyEdits(t *testing.T) {
	before := configuration.NewConfig()
	before.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	before.Routes["selected"] = qualifiedRoute("Selected", "gateway", "gpt-6-sol", configuration.ProtocolOpenAIResponses)
	before.Routes["extra"] = qualifiedRoute("Extra", "gateway", "grok-4.6", configuration.ProtocolOpenAIResponses)
	before.SetSelectedRoute(configuration.ClientHermes, "selected")
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
	executable := filepath.Join(t.TempDir(), "hermes")
	if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://gateway.test"}}
	cfg.Routes["hermes"] = qualifiedRoute("", "gateway", "claude-test", configuration.ProtocolAnthropic)
	cfg.Clients[configuration.ClientHermes] = configuration.ClientBinding{Route: "hermes", Enabled: true, Protocol: configuration.ProtocolAnthropic, Executable: executable, Targets: []string{filepath.Join(t.TempDir(), "config.yaml")}}
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
		assertHermesVerificationPolicy(t, plan)
	}
	_, err = (hermesAdapter{}).Verify(context.Background(), Dependencies{Runner: runner, Secrets: store, AIGWExecutable: filepath.Join(t.TempDir(), "aigw")}, cfg, clientRuntime, "")
	if runtime.GOOS == "windows" {
		if err == nil || !strings.Contains(err.Error(), "cannot isolate native launcher publication on Windows") || len(runner.plans) != 0 {
			t.Fatalf("unqualified Windows verification = %v, plans = %d", err, len(runner.plans))
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"chat", "--quiet", "--query-file", "-", "--oneshot", "--max-turns", "1", "--run-budget", "45", "--ignore-rules", "--source", "tool"}
	if len(runner.plans) != 2 || !slices.Equal(runner.plans[1].Args, want) {
		t.Fatalf("Hermes verification plans = %#v", runner.plans)
	}
}

func assertHermesVerificationPolicy(t *testing.T, plan process.Plan) {
	t.Helper()
	if !slices.Contains(plan.Env, "HERMES_DISABLE_LAZY_INSTALLS=1") {
		t.Fatal("isolated Hermes verification omitted the official bootstrap no-install guard")
	}
	data, err := os.ReadFile(filepath.Join(plan.Directory, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var projected struct {
		Security struct {
			AllowLazyInstalls *bool `yaml:"allow_lazy_installs"`
		} `yaml:"security"`
		Updates struct {
			Check *bool `yaml:"check"`
		} `yaml:"updates"`
		CLI struct {
			ExposeOnPath *bool `yaml:"expose_on_path"`
		} `yaml:"cli"`
		Plugins struct {
			Enabled []string `yaml:"enabled"`
		} `yaml:"plugins"`
		Memory struct {
			Enabled  *bool  `yaml:"enabled"`
			Provider string `yaml:"provider"`
		} `yaml:"memory"`
	}
	if err := yaml.Unmarshal(data, &projected); err != nil {
		t.Fatal(err)
	}
	if projected.Security.AllowLazyInstalls == nil || *projected.Security.AllowLazyInstalls {
		t.Fatal("isolated Hermes verification permits runtime dependency installation")
	}
	if projected.Updates.Check == nil || *projected.Updates.Check || projected.CLI.ExposeOnPath == nil || *projected.CLI.ExposeOnPath || len(projected.Plugins.Enabled) != 0 || projected.Memory.Enabled == nil || *projected.Memory.Enabled || projected.Memory.Provider != "" {
		t.Fatal("isolated Hermes verification permits operator activity or publication")
	}
}

func prepareHermesVerificationDependencyFixture(t *testing.T) (string, map[string][]byte) {
	t.Helper()
	root := t.TempDir()
	target := filepath.Join(root, "config.yaml")
	installed := filepath.Join(root, "installs", "fixture-install")
	for _, name := range []string{"environments", "pm-runtime"} {
		if err := os.MkdirAll(filepath.Join(installed, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	original := map[string][]byte{
		target:                                 []byte("plugins: {enabled: [member]}\nprivate_operator_setting: preserved\n"),
		filepath.Join(installed, "facts.json"): []byte("{\"generation\":\"native\"}\n"),
		filepath.Join(installed, ".install.lock"):               []byte(""),
		filepath.Join(installed, "boot-record.json"):            []byte("{\"operator\":true}\n"),
		filepath.Join(root, "plugins", "member", "plugin.yaml"): []byte("name: member\n"),
	}
	for path, data := range original {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root, original
}

func assertHermesVerificationFileIdentity(t *testing.T, source, borrowed string) {
	t.Helper()
	operator, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	observed, err := os.Stat(borrowed)
	if err != nil || !os.SameFile(operator, observed) {
		t.Fatalf("verification changed native file identity %s: %v", source, err)
	}
}

func TestHermesVerificationBorrowsInstalledDependenciesAndIsolatesBootState(t *testing.T) {
	if runtime.GOOS == "windows" {
		home, cleanup, err := hermesconfig.CreateVerificationHome(filepath.Join(t.TempDir(), "config.yaml"))
		if err == nil || home != "" || cleanup != nil || !strings.Contains(err.Error(), "no client process was started") {
			t.Fatalf("unqualified Windows verification home = %s, %v", home, err)
		}
		return
	}
	root, original := prepareHermesVerificationDependencyFixture(t)
	target := filepath.Join(root, "config.yaml")
	installed := filepath.Join(root, "installs", "fixture-install")
	executable := filepath.Join(root, "hermes")
	if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://gateway.test"}}
	cfg.Routes["hermes"] = qualifiedRoute("", "gateway", "claude-test", configuration.ProtocolAnthropic)
	cfg.Clients[configuration.ClientHermes] = configuration.ClientBinding{
		Route: "hermes", Enabled: true, Protocol: configuration.ProtocolAnthropic, Executable: executable, Targets: []string{target},
	}
	selected, err := cfg.ResolveRuntime(configuration.ClientHermes, "")
	if err != nil {
		t.Fatal(err)
	}
	store := secrets.NewMemoryStore()
	if err := store.Set("gateway", "public-fixture-token"); err != nil {
		t.Fatal(err)
	}
	var verificationHome string
	runner := &captureAdapterRunner{outputs: [][]byte{[]byte("Hermes Agent v1\n"), []byte("AIGW_OK\n")}}
	runner.observe = func(plan process.Plan) {
		verificationHome = plan.Directory
		borrowed := filepath.Join(plan.Directory, "installs", "fixture-install")
		assertHermesVerificationFileIdentity(t, filepath.Join(installed, "facts.json"), filepath.Join(borrowed, "facts.json"))
		assertHermesVerificationFileIdentity(t, filepath.Join(installed, ".install.lock"), filepath.Join(borrowed, ".install.lock"))
		profiles, err := os.ReadDir(filepath.Join(plan.Directory, "profiles"))
		if err != nil || len(profiles) != 1 {
			t.Fatalf("verification lost dependency-only member profiles: %v, %v", profiles, err)
		}
		profile := filepath.Join(plan.Directory, "profiles", profiles[0].Name())
		selector, err := os.ReadFile(filepath.Join(profile, "config.yaml"))
		if err != nil || !bytes.Contains(selector, []byte("member")) || bytes.Contains(selector, []byte("private_operator_setting")) {
			t.Fatalf("dependency profile copied private configuration or lost membership: %v", err)
		}
		assertHermesVerificationFileIdentity(t, filepath.Join(root, "plugins", "member", "plugin.yaml"), filepath.Join(profile, "plugins", "member", "plugin.yaml"))
		if err := os.WriteFile(filepath.Join(borrowed, "boot-record.json"), []byte("private boot"), 0o600); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(plan.Directory, "config.yaml"))
		if err != nil || bytes.Contains(data, []byte("private_operator_setting")) || bytes.Contains(data, []byte("member")) || !bytes.Contains(data, []byte("expose_on_path: false")) || !bytes.Contains(data, []byte("check: false")) {
			t.Fatalf("inference home retained operator activity or enabled publication: %v", err)
		}
	}
	if _, err := (hermesAdapter{}).Verify(context.Background(), Dependencies{Runner: runner, Secrets: store, AIGWExecutable: filepath.Join(root, "aigw")}, cfg, selected, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(verificationHome); !os.IsNotExist(err) {
		t.Fatalf("verification home survived completion: %v", err)
	}
	for path, want := range original {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("verification changed operator state %s: %v", path, err)
		}
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
