package clients

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/claudedesktop"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
)

type fixedDiscoverer struct{ result discovery.Result }

func (discoverer fixedDiscoverer) Discover() discovery.Result { return discoverer.result }

// hostAt observes home as the user's home directory, so Claude Code's settings
// file is home/.claude/settings.json.
func hostAt(home string) discovery.Host {
	return discovery.Host{GOOS: runtime.GOOS, Home: home, Env: map[string]string{"HOME": home, "USERPROFILE": home}}
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

func codexConfiguration(target string) domain.Config {
	cfg := domain.NewConfig(Admission())
	cfg.Accounts["gateway"] = domain.Account{Endpoints: domain.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	cfg.Routes["codex"] = qualifiedRoute("", "gateway", "gpt-test", domain.ProtocolOpenAIResponses)
	cfg.Clients[codex.ID] = domain.ClientBinding{Route: "codex", Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
	return cfg
}

// TestBuiltInAdmissionIsTheProductClientContract pins each built-in client's
// admission record and the admission order that help, errors and output use.
func TestBuiltInAdmissionIsTheProductClientContract(t *testing.T) {
	anthropic, responses := domain.ProtocolAnthropic, domain.ProtocolOpenAIResponses
	want := []domain.ClientSpec{
		{
			ID: claude.ID, Label: "Claude", EndpointProtocols: []domain.EndpointProtocol{anthropic},
			ProductName: "Claude Code", ModelDiscoveryOptional: true, Doctor: domain.DoctorWording{AdapterStates: true},
			PublishedProfileProtocol: anthropic,
		},
		{
			ID: claudedesktop.ID, Label: "Claude Desktop", EndpointProtocols: []domain.EndpointProtocol{anthropic},
			RestartAfterProjection: true, QualifiedModes: []string{"Chat", "Cowork", "Code"}, QualifiedPlatforms: []string{"macOS"},
		},
		{
			ID: codex.ID, Label: "Codex", EndpointProtocols: []domain.EndpointProtocol{responses},
			TargetFile: "config.toml", TargetsUsage: "repeat for multiple Codex homes", RetryBudgets: true,
			DefaultModelProvider: "aigw", ClientNativeAuthentication: true,
			PublishedProfileProtocol: responses, Doctor: domain.DoctorWording{AdapterStates: true, RouteBinding: true},
		},
		{
			ID: hermes.ID, Label: "Hermes", SessionsKeepRoute: true,
			EndpointProtocols: []domain.EndpointProtocol{responses, anthropic, domain.ProtocolOpenAIChatCompletions},
		},
	}
	if got := Admission().Specs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("built-in client specs = %#v, want %#v", got, want)
	}
	if got := Admission().Usage(); got != "claude, claude-desktop, codex, or hermes" {
		t.Fatalf("built-in client usage = %q", got)
	}
}

func TestChangedClientsRetainsAdmissionOrderAndIndependentResults(t *testing.T) {
	before := codexConfiguration("/target")
	account := before.Accounts["gateway"]
	account.Endpoints.Anthropic = "https://gateway.test"
	before.Accounts["gateway"] = account
	before.Routes["claude"] = qualifiedRoute("", "gateway", "claude-test", domain.ProtocolAnthropic)
	before.SetSelectedRoute(claude.ID, "claude", "")
	before.SetClientActivation(claude.ID, true, "/opt/claude", nil)
	after := before.Clone()
	account.Endpoints = domain.Endpoints{OpenAIResponses: "https://replacement.test/v1", Anthropic: "https://replacement.test"}
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

	before := domain.NewConfig(Admission())
	before.Accounts["gateway"] = domain.Account{Label: "Gateway", Endpoints: domain.Endpoints{
		Anthropic:       "https://gateway.test",
		OpenAIResponses: "https://gateway.test/v1",
	}}
	before.Routes["claude"] = qualifiedRoute("Claude", "gateway", "claude-test", domain.ProtocolAnthropic)
	before.Routes["codex"] = qualifiedRoute("Codex", "gateway", "gpt-test", domain.ProtocolOpenAIResponses)
	before.SetSelectedRoute(claude.ID, "claude", "")
	before.SetSelectedRoute(codex.ID, "codex", "")
	after := before.Clone()
	after.SetClientActivation(claude.ID, true, "/opt/claude", nil)
	after.SetClientActivation(codex.ID, true, "/opt/codex", []string{codexTarget})
	deps := adapter.Dependencies{
		Discovery: fixedDiscoverer{result: discovery.Result{Surfaces: []discovery.Surface{{
			ID: "codex-home-default", Authority: "aigw", ConfigPath: codexTarget, Present: true, AutoManaged: true,
		}}}},
		Host:           hostAt(dir),
		AIGWExecutable: "/opt/aigw",
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
type futureAdapter struct{ adapter.Adapter }

func (futureAdapter) Spec() domain.ClientSpec {
	return domain.ClientSpec{ID: "future", Label: "Future", EndpointProtocols: []domain.EndpointProtocol{domain.ProtocolOpenAIResponses}}
}

func (clientAdapter futureAdapter) Converge(_ adapter.Dependencies, cfg *domain.Config, _ discovery.Result) error {
	cfg.SetClientActivation(clientAdapter.Spec().ID, true, "", nil)
	return nil
}

func TestFutureClientAdmissionPreservesBuiltInClientsAndProviderState(t *testing.T) {
	baselineClientIDs := Registry().IDs()
	future := futureAdapter{}
	registry, err := adapter.NewRegistry(append(adapters(), future)...)
	if err != nil {
		t.Fatal(err)
	}
	before := domain.NewConfig(registry.Admission())
	before.Accounts["direct"] = domain.Account{
		Label: "Direct",
		Endpoints: domain.Endpoints{
			Anthropic:       "https://direct.example.test",
			OpenAIResponses: "https://direct.example.test/v1",
		},
	}
	before.Accounts["gateway"] = domain.Account{
		Label: "External gateway",
		Endpoints: domain.Endpoints{
			Anthropic:       "http://127.0.0.1:9876",
			OpenAIResponses: "http://127.0.0.1:9876/v1",
		},
	}
	before.Routes["claude"] = qualifiedRoute("", "direct", "claude-test", domain.ProtocolAnthropic)
	before.Routes["codex"] = qualifiedRoute("", "gateway", "gpt-test", domain.ProtocolOpenAIResponses)
	before.SetSelectedRoute(claude.ID, "claude", "")
	before.SetSelectedRoute(codex.ID, "codex", "")
	before.SetClientActivation(claude.ID, true, "/clients/claude", nil)
	before.SetClientActivation(codex.ID, true, "/clients/codex", []string{"/clients/codex.toml"})
	wantUnchanged := before.Clone()

	after, err := registry.Converge(adapter.Dependencies{}, before, discovery.Result{}, future.Spec().ID)
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
	cfg := domain.NewConfig(Admission())
	cfg.Accounts["gateway"] = domain.Account{Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
	cfg.Routes["claude"] = domain.Route{
		Account: "gateway", Model: "claude-test",
		Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}},
	}
	store := secret.NewMemoryStore()
	if err := store.Set("gateway", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	deps := adapter.Dependencies{Secrets: store, AIGWExecutable: filepath.Join(t.TempDir(), "aigw")}

	for _, clientID := range []string{claudedesktop.ID, hermes.ID} {
		t.Run(clientID, func(t *testing.T) {
			deferred := cfg.Clone()
			deferred.SetSelectedRoute(clientID, "claude", "")
			deferred.SetClientActivation(clientID, true, "", nil)
			plans, err := Registry().Plan(deps, domain.NewConfig(Admission()), deferred, clientID)
			if err != nil {
				t.Fatalf("Plan(%q) deferred binding: %v", clientID, err)
			}
			if len(plans) != 0 {
				t.Fatalf("Plan(%q) deferred binding = %#v, want no projection", clientID, plans)
			}
		})
	}
}

// AIGW does not start on a host where a built-in client cannot derive its
// native paths, and it reports Claude Code's missing input before Claude
// Desktop's.
func TestBuiltInClientsRefuseAHostWithoutTheirNativePaths(t *testing.T) {
	roaming, local, profile := `C:\Users\alex\AppData\Roaming`, `C:\Users\alex\AppData\Local`, `C:\Users\alex`
	for _, test := range []struct {
		name string
		env  map[string]string
		want string
	}{
		{name: "complete", env: map[string]string{"APPDATA": roaming, "LOCALAPPDATA": local, "USERPROFILE": profile}},
		{name: "without USERPROFILE", env: map[string]string{"APPDATA": roaming, "LOCALAPPDATA": local}, want: "USERPROFILE is not set"},
		{name: "without LOCALAPPDATA", env: map[string]string{"APPDATA": roaming, "USERPROFILE": profile}, want: "LOCALAPPDATA is not set"},
		{name: "without either", env: map[string]string{"APPDATA": roaming}, want: "USERPROFILE is not set"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := Registry().ValidateHost(discovery.Host{GOOS: "windows", Env: test.env})
			if (err == nil) != (test.want == "") || err != nil && err.Error() != test.want {
				t.Fatalf("ValidateHost() = %v, want %q", err, test.want)
			}
		})
	}
}
