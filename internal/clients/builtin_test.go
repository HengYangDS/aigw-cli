package clients

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/client"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/secrets"
)

type fixedDiscoverer struct{ result discovery.Result }

func (discoverer fixedDiscoverer) Discover() discovery.Result { return discoverer.result }

func qualifiedRoute(label, account, model string, protocols ...configuration.EndpointProtocol) configuration.Route {
	interfaces := make(map[configuration.EndpointProtocol][]configuration.Capability, len(protocols))
	for _, protocol := range protocols {
		interfaces[protocol] = []configuration.Capability{}
	}
	return configuration.Route{
		Label: label, Account: account, Model: model, UpstreamModel: model, Interfaces: interfaces,
	}
}

func codexConfiguration(target string) configuration.Config {
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Endpoints: configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	cfg.Routes["codex"] = qualifiedRoute("", "gateway", "gpt-test", configuration.ProtocolOpenAIResponses)
	cfg.Clients[configuration.ClientCodex] = configuration.ClientBinding{Route: "codex", Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
	return cfg
}

func TestChangedClientsRetainsAdmissionOrderAndIndependentResults(t *testing.T) {
	before := codexConfiguration("/target")
	account := before.Accounts["gateway"]
	account.Endpoints.Anthropic = "https://gateway.test"
	before.Accounts["gateway"] = account
	before.Routes["claude"] = qualifiedRoute("", "gateway", "claude-test", configuration.ProtocolAnthropic)
	before.SetSelectedRoute(configuration.ClientClaude, "claude", "")
	before.SetClientActivation(configuration.ClientClaude, true, "/opt/claude", nil)
	after := before.Clone()
	account.Endpoints = configuration.Endpoints{OpenAIResponses: "https://replacement.test/v1", Anthropic: "https://replacement.test"}
	after.Accounts["gateway"] = account
	registry := Registry()
	changed := registry.ChangedClients(before, after)
	want := before.EnabledClientIDs()
	if !slices.Equal(changed, want) {
		t.Fatalf("shared Account change scope = %v, want admission order %v", changed, want)
	}
	changed[0] = "caller edit"
	if got := registry.ChangedClients(before, after); !slices.Equal(got, want) {
		t.Fatalf("caller changed registry scope: %v", got)
	}
}

func TestRegistryPreparesEveryClientBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	codexTarget := filepath.Join(dir, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(codexTarget), 0o700); err != nil {
		t.Fatal(err)
	}
	originalCodex := []byte("model_provider = \"native\"\n")
	if err := os.WriteFile(codexTarget, originalCodex, 0o600); err != nil {
		t.Fatal(err)
	}
	claudeSettings := filepath.Join(dir, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(claudeSettings), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claudeSettings, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}

	before := configuration.NewConfig()
	before.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{
		Anthropic:       "https://gateway.test",
		OpenAIResponses: "https://gateway.test/v1",
	}}
	before.Routes["claude"] = qualifiedRoute("Claude", "gateway", "claude-test", configuration.ProtocolAnthropic)
	before.Routes["codex"] = qualifiedRoute("Codex", "gateway", "gpt-test", configuration.ProtocolOpenAIResponses)
	before.SetSelectedRoute(configuration.ClientClaude, "claude", "")
	before.SetSelectedRoute(configuration.ClientCodex, "codex", "")
	after := before.Clone()
	after.SetClientActivation(configuration.ClientClaude, true, "/opt/claude", nil)
	after.SetClientActivation(configuration.ClientCodex, true, "/opt/codex", []string{codexTarget})
	deps := client.Dependencies{
		Discovery: fixedDiscoverer{result: discovery.Result{Surfaces: []discovery.Surface{{
			ID: "codex-home-default", Authority: "aigw", ConfigPath: codexTarget, Present: true, AutoManaged: true,
		}}}},
		ClaudeSettingsPath: claudeSettings,
		AIGWExecutable:     "/opt/aigw",
	}

	_, _, err := Registry().Apply(t.Context(), deps, before, after)
	if err == nil || !strings.Contains(err.Error(), "parse Claude settings") {
		t.Fatalf("Apply() error = %v", err)
	}
	got, readErr := os.ReadFile(codexTarget)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(got, originalCodex) {
		t.Fatalf("Codex changed before Claude preparation completed:\n%s", got)
	}
	for _, path := range []string{codexTarget + ".aigw-state.json", filepath.Join(filepath.Dir(codexTarget), "models.json")} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("Codex artifact %s exists after preparation failure: %v", path, statErr)
		}
	}
}

// futureAdapter is a client admitted after the built-in set. It embeds the seam,
// so it implements only the two methods that admission reaches.
type futureAdapter struct{ client.Adapter }

func (futureAdapter) Spec() configuration.ClientSpec {
	return configuration.ClientSpec{ID: "future", Label: "Future", EndpointProtocols: []configuration.EndpointProtocol{configuration.ProtocolOpenAIResponses}}
}

func (adapter futureAdapter) Converge(_ client.Dependencies, cfg *configuration.Config, _ discovery.Result) error {
	cfg.SetClientActivation(adapter.Spec().ID, true, "", nil)
	return nil
}

func TestFutureClientAdmissionPreservesBuiltInClientsAndProviderState(t *testing.T) {
	baselineClientIDs := Registry().IDs()
	before := configuration.NewConfig()
	before.Accounts["direct"] = configuration.Account{
		Label: "Direct",
		Endpoints: configuration.Endpoints{
			Anthropic:       "https://direct.example.test",
			OpenAIResponses: "https://direct.example.test/v1",
		},
	}
	before.Accounts["gateway"] = configuration.Account{
		Label: "External gateway",
		Endpoints: configuration.Endpoints{
			Anthropic:       "http://127.0.0.1:9876",
			OpenAIResponses: "http://127.0.0.1:9876/v1",
		},
	}
	before.Routes["claude"] = qualifiedRoute("", "direct", "claude-test", configuration.ProtocolAnthropic)
	before.Routes["codex"] = qualifiedRoute("", "gateway", "gpt-test", configuration.ProtocolOpenAIResponses)
	before.SetSelectedRoute(configuration.ClientClaude, "claude", "")
	before.SetSelectedRoute(configuration.ClientCodex, "codex", "")
	before.SetClientActivation(configuration.ClientClaude, true, "/clients/claude", nil)
	before.SetClientActivation(configuration.ClientCodex, true, "/clients/codex", []string{"/clients/codex.toml"})
	wantUnchanged := before.Clone()

	future := futureAdapter{}
	specs := append(configuration.AdmittedClientSpecs(), future.Spec())
	registry, err := client.NewRegistry(specs, append(adapters(), future)...)
	if err != nil {
		t.Fatal(err)
	}
	after, err := registry.Converge(client.Dependencies{}, before, discovery.Result{}, future.Spec().ID)
	if err != nil {
		t.Fatal(err)
	}
	if !after.Clients[future.Spec().ID].Enabled {
		t.Fatal("future client was not admitted through its adapter")
	}
	delete(after.Clients, future.Spec().ID)
	if !reflect.DeepEqual(after, wantUnchanged) {
		t.Fatalf("future client changed existing client or Provider state:\n got %#v\nwant %#v", after, wantUnchanged)
	}
	if !reflect.DeepEqual(before, wantUnchanged) {
		t.Fatal("future client admission mutated its input configuration")
	}
	if got := Registry().IDs(); !reflect.DeepEqual(got, baselineClientIDs) {
		t.Fatalf("built-in registry changed: got %v, want %v", got, baselineClientIDs)
	}
}

func TestRegistryPlansEnabledUnavailableClientsAsDeferred(t *testing.T) {
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://gateway.test"}}
	cfg.Routes["claude"] = configuration.Route{
		Account: "gateway", Model: "claude-test",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}},
	}
	store := secrets.NewMemoryStore()
	if err := store.Set("gateway", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	deps := client.Dependencies{Secrets: store, AIGWExecutable: filepath.Join(t.TempDir(), "aigw")}

	for _, clientID := range []string{configuration.ClientClaudeDesktop, configuration.ClientHermes} {
		t.Run(clientID, func(t *testing.T) {
			deferred := cfg.Clone()
			deferred.SetSelectedRoute(clientID, "claude", "")
			deferred.SetClientActivation(clientID, true, "", nil)
			plans, err := Registry().Plan(deps, configuration.NewConfig(), deferred, clientID)
			if err != nil {
				t.Fatalf("Plan(%q) deferred binding: %v", clientID, err)
			}
			if len(plans) != 0 {
				t.Fatalf("Plan(%q) deferred binding = %#v, want no projection", clientID, plans)
			}
		})
	}
}
