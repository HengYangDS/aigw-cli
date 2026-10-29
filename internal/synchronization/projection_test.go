package synchronization

import (
	"aigw-cli/internal/client/claude"
	"aigw-cli/internal/client/codex"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secrets"
	"aigw-cli/internal/store"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesiredClientConfigurationScopesDiscoveryToRequestedClient(t *testing.T) {
	claudeExecutable := filepath.Join(t.TempDir(), "claude")
	before := domain.NewConfig(clients.Admission())
	before.Accounts["gateway"] = domain.Account{Label: "Gateway", Endpoints: domain.Endpoints{
		Anthropic:       "https://gateway.test",
		OpenAIResponses: "https://gateway.test/v1",
	}}
	before.Routes["claude"] = domain.Route{Label: "Claude", Account: "gateway", Model: "claude-test", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	before.Routes["codex"] = domain.Route{Label: "Codex", Account: "gateway", Model: "gpt-test", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	before.SetSelectedRoute(claude.ID, "claude", "")
	before.SetSelectedRoute(codex.ID, "codex", "")
	before.SetClientActivation(codex.ID, true, "/existing/codex", []string{"/explicit/config.toml"})
	secretStore := secrets.NewMemoryStore()
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	syncer := Synchronizer{Registry: builtinClients, Secrets: secretStore, Discovery: staticDiscovery{result: discovery.Result{Executables: map[string]string{
		claude.ID: claudeExecutable,
	}}}}

	after, _, err := syncer.DesiredClientConfiguration(before, claude.ID)
	if err != nil {
		t.Fatal(err)
	}
	if adapter := after.Clients[claude.ID]; adapter.Enabled || adapter.Route != "claude" || adapter.Executable != "" {
		t.Fatalf("discovery changed disabled Claude intent: %#v", adapter)
	}
	if got := after.Clients[codex.ID]; !got.Enabled || got.Executable != "/existing/codex" || len(got.Targets) != 1 || got.Targets[0] != "/explicit/config.toml" {
		t.Fatalf("unselected Codex adapter changed: %#v", got)
	}
}

func TestDesiredClientConfigurationDoesNotReselectRoutes(t *testing.T) {
	before := domain.NewConfig(clients.Admission())
	before.Accounts["one"] = domain.Account{Label: "One", Endpoints: domain.Endpoints{Anthropic: "https://one.test"}}
	before.Accounts["two"] = domain.Account{Label: "Two", Endpoints: domain.Endpoints{Anthropic: "https://two.test"}}
	before.Routes["one"] = domain.Route{Label: "One", Account: "one", Model: "claude-test", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	before.Routes["two"] = domain.Route{Label: "Two", Account: "two", Model: "claude-test", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	before.SetSelectedRoute(claude.ID, "one", "")
	secretStore := secrets.NewMemoryStore()
	if err := secretStore.Set("two", "token"); err != nil {
		t.Fatal(err)
	}

	after, _, err := (Synchronizer{Registry: builtinClients, Secrets: secretStore, Discovery: staticDiscovery{}}).DesiredClientConfiguration(before)
	if err != nil {
		t.Fatal(err)
	}
	if got := after.SelectedRoute(claude.ID); got != "one" {
		t.Fatalf("client discovery reselected route = %q, want one", got)
	}
}

func TestDesiredSyncConfigurationDoesNotSearchUnselectedWritableCredentials(t *testing.T) {
	before := domain.NewConfig(clients.Admission())
	before.Accounts["team"] = domain.Account{Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	before.Routes["claude"] = domain.Route{Account: "team", Model: "model", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	before.SetRecommendedRoute(claude.ID, "claude")
	want := errors.New("unselected native credential was inspected")
	syncer := Synchronizer{Registry: builtinClients, Secrets: secretReadStub{err: want}, Discovery: staticDiscovery{}}

	after, _, err := syncer.DesiredSyncConfiguration(before)
	if err != nil || len(after.Clients) != 0 {
		t.Fatalf("unselected writable Account was probed or activated: %#v, %v", after.Clients, err)
	}
}

type failingReadOnlyStore struct{ secretReadStub }

func (failingReadOnlyStore) ReadOnly() bool { return true }

func TestDesiredSyncConfigurationFailsClosedOnReadOnlyMetadataError(t *testing.T) {
	before := domain.NewConfig(clients.Admission())
	before.Accounts["team"] = domain.Account{Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	want := errors.New("environment metadata unavailable")
	syncer := Synchronizer{Registry: builtinClients, Secrets: failingReadOnlyStore{secretReadStub{err: want}}, Discovery: staticDiscovery{}}
	after, _, err := syncer.DesiredSyncConfiguration(before)
	if !errors.Is(err, want) || len(after.Accounts) != 0 {
		t.Fatalf("failed credential observation admitted a selected configuration: %#v, %v", after, err)
	}
}

func TestDesiredSyncConfigurationPreservesExplicitDisabledSelection(t *testing.T) {
	before := domain.NewConfig(clients.Admission())
	for _, account := range []string{"manual", "recommended"} {
		before.Accounts[account] = domain.Account{Endpoints: domain.Endpoints{Anthropic: "https://" + account + ".test"}}
		before.Routes[account] = domain.Route{Account: account, Model: "model", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	}
	before.SetRecommendedRoute(claude.ID, "recommended")
	before.SetSelectedRoute(claude.ID, "manual", "")
	store := secrets.NewEnvironmentStore(func(key string) string {
		if key == secrets.EnvironmentKey("recommended") {
			return "token"
		}
		return ""
	})
	after, _, err := (Synchronizer{Registry: builtinClients, Secrets: store, Discovery: staticDiscovery{}}).DesiredSyncConfiguration(before)
	if err != nil || after.SelectedRoute(claude.ID) != "manual" || after.Clients[claude.ID].Enabled {
		t.Fatalf("environment recommendation overrode an explicit disabled selection: %#v, %v", after.Clients, err)
	}
}

func TestDesiredClientConfigurationSurfacesCredentialObservationFailures(t *testing.T) {
	before := domain.NewConfig(clients.Admission())
	before.Accounts["gateway"] = domain.Account{Label: "Gateway", Endpoints: domain.Endpoints{
		Anthropic:       "https://gateway.test",
		OpenAIResponses: "https://gateway.test/v1",
	}}
	before.Routes["claude"] = domain.Route{Label: "Claude", Account: "gateway", Model: "claude-test", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	before.Routes["codex"] = domain.Route{Label: "Codex", Account: "gateway", Model: "gpt-test", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	before.SetSelectedRoute(claude.ID, "claude", "")
	before.SetSelectedRoute(codex.ID, "codex", "")
	want := errors.New("credential observation failed")
	syncer := Synchronizer{Registry: builtinClients, Secrets: secretReadStub{err: want}, Discovery: staticDiscovery{}}

	for _, client := range []string{claude.ID, codex.ID} {
		enabled := before.Clone()
		enabled.SetClientActivation(client, true, "", nil)
		if _, _, err := syncer.DesiredClientConfiguration(enabled, client); !errors.Is(err, want) {
			t.Fatalf("DesiredClientConfiguration(%q) error = %v, want %v", client, err, want)
		}
	}
}

func TestDesiredClientConfigurationDoesNotActivateUnboundClients(t *testing.T) {
	before := domain.NewConfig(clients.Admission())
	before.Accounts["team"] = domain.Account{Label: "Team", Endpoints: domain.Endpoints{
		Anthropic: "https://team.test", OpenAIResponses: "https://team.test/v1",
	}}
	before.Routes["claude"] = domain.Route{Label: "Claude", Account: "team", Model: "claude-test", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	before.Routes["codex"] = domain.Route{Label: "Codex", Account: "team", Model: "gpt-test", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	before.SetRecommendedRoute(claude.ID, "claude")
	store := secrets.NewMemoryStore()
	if err := store.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	after, _, err := (Synchronizer{Registry: builtinClients, Secrets: store, Discovery: staticDiscovery{}}).DesiredClientConfiguration(before, claude.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Clients) != 0 || len(before.Clients) != 0 {
		t.Fatalf("discovery activated an unbound client: before=%#v after=%#v", before.Clients, after.Clients)
	}
}

func TestProjectionPlanningValidatesEnabledClients(t *testing.T) {
	syncer := Synchronizer{Registry: builtinClients}
	cfg := domain.NewConfig(clients.Admission())
	plans, err := syncer.Plan(cfg, cfg)
	if err != nil || len(plans) != 0 {
		t.Fatalf("disabled plan = %#v, %v", plans, err)
	}

	invalid := testConfig("/target")
	delete(invalid.Routes, "gpt")
	if _, err := (Synchronizer{Registry: builtinClients, Discovery: staticDiscovery{}}).Plan(invalid, invalid); err == nil {
		t.Fatal("planning accepted an invalid runtime")
	}
}

func TestPlanIncludesClaudeProjectionAndRestore(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	before := domain.NewConfig(clients.Admission())
	before.Accounts["gateway"] = domain.Account{Label: "Gateway", Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
	before.Routes["claude"] = domain.Route{Label: "Claude", Account: "gateway", Model: "claude-team", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	before.SetSelectedRoute(claude.ID, "claude", "")
	after := before.Clone()
	after.SetClientActivation(claude.ID, true, "/opt/claude", nil)
	configStore := store.NewStore(filepath.Join(dir, "aigw.toml"), clients.Admission())
	if err := configStore.Save(before); err != nil {
		t.Fatal(err)
	}
	syncer := Synchronizer{Registry: builtinClients, Config: configStore, Discovery: staticDiscovery{}, Host: hostAt(dir), AIGWExecutable: filepath.Join(dir, "aigw")}

	plans, err := syncer.Plan(before, after)
	if err != nil || len(plans) != 1 || plans[0].Client != claude.ID || plans[0].Target != settingsPath || plans[0].Action != "project" {
		t.Fatalf("project plans = %#v, %v", plans, err)
	}
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Fatalf("planning wrote Claude settings: %v", err)
	}
	if err := syncer.CommitProjection(t.Context(), before, after, "Claude projection"); err != nil {
		t.Fatal(err)
	}
	plans, err = syncer.Plan(after, before)
	if err != nil || len(plans) != 1 || plans[0].Client != claude.ID || plans[0].Action != "restore" {
		t.Fatalf("restore plans = %#v, %v", plans, err)
	}
}

func TestCommitProjectionPreservesClaudeModelAcrossHelperChange(t *testing.T) {
	root := t.TempDir()
	settings := filepath.Join(root, ".claude", "settings.json")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["gateway"] = domain.Account{Label: "Gateway", Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
	cfg.Routes["claude"] = domain.Route{Label: "Claude", Account: "gateway", Model: "claude-opus-5-5", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.SetSelectedRoute(claude.ID, "claude", "")
	cfg.SetClientActivation(claude.ID, true, "/opt/claude", nil)
	syncer := Synchronizer{Registry: builtinClients,
		Config:    store.NewStore(filepath.Join(root, "aigw.toml"), clients.Admission()),
		Discovery: staticDiscovery{}, Host: hostAt(root),
		AIGWExecutable: filepath.Join(root, "old-aigw"),
	}
	if err := syncer.CommitProjection(t.Context(), domain.NewConfig(clients.Admission()), cfg, "initial Claude projection"); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	edited := bytes.Replace(before, []byte(`"claude-opus-5-5"`), []byte(`"opus[1m]"`), 1)
	if bytes.Equal(before, edited) {
		t.Fatal("native model edit did not change the projection")
	}
	if err := os.WriteFile(settings, edited, 0o600); err != nil {
		t.Fatal(err)
	}
	syncer.AIGWExecutable = filepath.Join(root, "new-aigw")
	plans, err := syncer.Plan(cfg, cfg)
	if err != nil || len(plans) != 1 || plans[0].Action != "project" {
		t.Fatalf("helper migration plan = %#v, %v", plans, err)
	}
	if err := syncer.CommitProjection(t.Context(), cfg, cfg, "Claude helper migration"); err != nil {
		t.Fatal(err)
	}
	var projected struct {
		Model        string `json:"model"`
		APIKeyHelper string `json:"apiKeyHelper"`
	}
	data, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &projected); err != nil {
		t.Fatal(err)
	}
	if projected.Model != "opus[1m]" || !strings.Contains(projected.APIKeyHelper, syncer.AIGWExecutable) {
		t.Fatalf("helper migration lost native model or target helper: %#v", projected)
	}
}

func TestPlanReportsClaudePlanningFailures(t *testing.T) {
	before := domain.NewConfig(clients.Admission())
	before.Accounts["gateway"] = domain.Account{Label: "Gateway", Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
	before.Routes["claude"] = domain.Route{Label: "Claude", Account: "gateway", Model: "claude-team", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	before.SetSelectedRoute(claude.ID, "claude", "")
	after := before.Clone()
	after.SetClientActivation(claude.ID, true, "/opt/claude", nil)

	if _, err := (Synchronizer{Registry: builtinClients, Discovery: staticDiscovery{}}).Plan(before, after); err == nil || !strings.Contains(err.Error(), "settings path") {
		t.Fatalf("missing settings path error = %v", err)
	}
	if _, err := (Synchronizer{Registry: builtinClients, Discovery: staticDiscovery{}, Host: hostAt(t.TempDir())}).Plan(before, after); err == nil || !strings.Contains(err.Error(), "executable path") {
		t.Fatalf("missing AIGW executable error = %v", err)
	}
	invalid := after.Clone()
	delete(invalid.Routes, "claude")
	if _, err := (Synchronizer{Registry: builtinClients, Discovery: staticDiscovery{}, Host: hostAt(t.TempDir()), AIGWExecutable: "/aigw"}).Plan(before, invalid); err == nil {
		t.Fatal("invalid enabled Claude runtime was accepted")
	}
}

func TestCommitReconcilesOnlyClientsWhoseProjectionChanges(t *testing.T) {
	for _, selected := range []string{claude.ID, codex.ID} {
		t.Run(selected, func(t *testing.T) {
			root := t.TempDir()
			targets := map[string]string{
				codex.ID:  filepath.Join(root, "codex.toml"),
				claude.ID: filepath.Join(root, ".claude", "settings.json"),
			}
			before := testConfig(targets[codex.ID])
			account := before.Accounts["gateway"]
			account.Endpoints.Anthropic = "https://gateway.test"
			before.Accounts["gateway"] = account
			before.Routes["claude"] = domain.Route{Label: "Claude", Account: "gateway", Model: "claude-original", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
			before.SetSelectedRoute(claude.ID, "claude", "")
			before.SetClientActivation(claude.ID, true, "/opt/claude", nil)
			configStore := store.NewStore(filepath.Join(root, "aigw.toml"), clients.Admission())
			syncer := Synchronizer{Registry: builtinClients, Config: configStore, Discovery: targetDiscovery(targets[codex.ID]), Host: hostAt(root), AIGWExecutable: filepath.Join(root, "aigw")}
			if err := syncer.CommitProjection(t.Context(), domain.NewConfig(clients.Admission()), before, "initial projection"); err != nil {
				t.Fatal(err)
			}
			unselected := claude.ID
			model := "claude-original"
			if selected == claude.ID {
				unselected = codex.ID
				model = "gpt-test"
			}
			foreignPath := targets[unselected]
			statePath := foreignPath + ".aigw-state.json"
			state, err := os.ReadFile(statePath)
			if err != nil {
				t.Fatal(err)
			}
			original, err := os.ReadFile(foreignPath)
			if err != nil {
				t.Fatal(err)
			}
			foreign := bytes.ReplaceAll(original, []byte(model), []byte("user-selected-model"))
			if bytes.Equal(original, foreign) {
				t.Fatal("fixture did not change the unselected client's owned model")
			}
			if err := os.WriteFile(foreignPath, foreign, 0o600); err != nil {
				t.Fatal(err)
			}
			after := before.Clone()
			account = after.Accounts["gateway"]
			if selected == codex.ID {
				account.Endpoints.OpenAIResponses = "https://replacement.test/v1"
			} else {
				account.Endpoints.Anthropic = "https://replacement.test"
			}
			after.Accounts["gateway"] = account
			if err := syncer.Commit(t.Context(), before, after, "endpoint edit"); err != nil {
				t.Fatalf("unrelated %s edits blocked %s: %v", unselected, selected, err)
			}
			projected, err := os.ReadFile(targets[selected])
			if err != nil || !bytes.Contains(projected, []byte("https://replacement.test")) {
				t.Fatalf("selected projection did not converge: %s, %v", projected, err)
			}
			preserved, err := os.ReadFile(foreignPath)
			if err != nil || !bytes.Equal(preserved, foreign) {
				t.Fatalf("unselected projection changed: %s, %v", preserved, err)
			}
			preservedState, err := os.ReadFile(statePath)
			if err != nil || !bytes.Equal(preservedState, state) {
				t.Fatalf("unselected ownership state changed: %s, %v", preservedState, err)
			}
			stored, err := configStore.Load()
			if err != nil || stored.Accounts["gateway"].Endpoints != account.Endpoints {
				t.Fatalf("configuration did not commit: %#v, %v", stored.Accounts, err)
			}
		})
	}
}

func TestProjectionPlanningRequiresDiscoveryAndValidTargets(t *testing.T) {
	base := testConfig("/target")
	if _, err := (Synchronizer{Registry: builtinClients}).Plan(base, base); err == nil {
		t.Fatal("expected discovery error")
	}
	syncer := Synchronizer{Registry: builtinClients, Discovery: staticDiscovery{}}
	before := base.Clone()
	before.SetClientActivation(codex.ID, true, "", []string{""})
	if _, err := syncer.Plan(before, base); err == nil {
		t.Fatal("expected before-target error")
	}
	after := base.Clone()
	after.SetClientActivation(codex.ID, true, "", []string{""})
	if _, err := syncer.Plan(base, after); err == nil {
		t.Fatal("expected after-target error")
	}
}

func TestReconcileClientHonorsCancellationBeforeObservation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := (Synchronizer{Registry: builtinClients}).ReconcileClient(ctx, domain.NewConfig(clients.Admission()), claude.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled projection = %v", err)
	}
}

func TestCommitProjectionDoesNotBindNativeAuthentication(t *testing.T) {
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := testConfig(target)
	delete(before.Clients, codex.ID)
	after := testConfig(target)
	configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	if err := configStore.Save(before); err != nil {
		t.Fatal(err)
	}
	runner := &recordingRunner{}
	syncer := Synchronizer{Registry: builtinClients,
		Config:         configStore,
		AIGWExecutable: filepath.Join(t.TempDir(), "aigw"),
		Secrets:        secretReadStub{},
		Runner:         runner,
		Discovery:      targetDiscovery(target),
	}

	if err := syncer.CommitProjection(context.Background(), before, after, "sync"); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("projection-only commit started authentication: %#v", runner.plans)
	}
	stored, err := configStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !stored.Clients[codex.ID].Enabled {
		t.Fatal("projection-only commit did not persist the discovered adapter")
	}
	projected, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(projected), "# managed by AIGW") {
		t.Fatalf("projection-only commit did not converge the target:\n%s", projected)
	}
}

func TestCommitProjectionRepairsUnchangedConfiguration(t *testing.T) {
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(target)
	configStore := store.NewStore(filepath.Join(t.TempDir(), "aigw.toml"), clients.Admission())
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	runner := &recordingRunner{}
	syncer := Synchronizer{Registry: builtinClients, Config: configStore, Discovery: targetDiscovery(target), Runner: runner, AIGWExecutable: filepath.Join(t.TempDir(), "aigw")}
	for range 2 {
		if err := syncer.CommitProjection(context.Background(), cfg, cfg, "sync"); err != nil {
			t.Fatal(err)
		}
		projected, err := os.ReadFile(target)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(projected), "# managed by AIGW") {
			t.Fatalf("unchanged configuration left the target unprojected:\n%s", projected)
		}
	}
	if len(runner.plans) != 0 {
		t.Fatalf("projection repair started authentication: %#v", runner.plans)
	}
}

func TestClientNativeModelProviderChangesProjectionWithoutAIGWCredentialHelper(t *testing.T) {
	target := filepath.Join(t.TempDir(), "config.toml")
	credentialCommand := filepath.Join(t.TempDir(), "aigw")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := testConfig(target)
	after := before.Clone()
	route := after.Routes["gpt"]
	route.Model = "openai.gpt-5.6-sol"
	after.Routes["gpt"] = route
	binding := after.Clients[codex.ID]
	binding.ModelProvider = "amazon-bedrock"
	binding.Authentication = domain.AuthenticationClientNative
	after.Clients[codex.ID] = binding

	configStore := store.NewStore(filepath.Join(t.TempDir(), "aigw.toml"), clients.Admission())
	if err := configStore.Save(before); err != nil {
		t.Fatal(err)
	}
	syncer := Synchronizer{Registry: builtinClients, Config: configStore, Discovery: targetDiscovery(target), AIGWExecutable: credentialCommand}
	if err := syncer.Commit(t.Context(), before, after, "native provider"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, `model_provider = "amazon-bedrock"`) {
		t.Fatalf("native projection missing provider:\n%s", text)
	}
	for _, forbidden := range []string{"command = " + credentialCommand, `[model_providers.amazon-bedrock.auth]`, `args = ["credential", "codex"]`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("native projection contains AIGW command authentication %q:\n%s", forbidden, text)
		}
	}
}
