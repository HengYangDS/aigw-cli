package synchronization

import (
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/claudedesktop"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
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
	before.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{
		Anthropic: "https://gateway.test",
		Responses: "https://gateway.test/v1",
	}}
	before.Models[domain.NewModelRef("gateway", "claude-test")] = domain.Model{Label: "Claude", Provider: "gateway", Family: "claude-test", ID: "claude-test", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	before.Models[domain.NewModelRef("gateway", "gpt-test")] = domain.Model{Label: "Codex", Provider: "gateway", Family: "gpt-test", ID: "gpt-test", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolResponses: {}}}
	before.SetSelectedModel(claude.ID, domain.NewModelRef("gateway", "claude-test"), "")
	before.SetSelectedModel(codex.ID, domain.NewModelRef("gateway", "gpt-test"), "")
	before.SetClientActivation(codex.ID, true, "/existing/codex", []string{"/explicit/config.toml"})
	secretStore := secret.NewMemoryStore()
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
	if adapter := after.Clients[claude.ID]; adapter.Enabled || adapter.Model != domain.NewModelRef("gateway", "claude-test") || adapter.Executable != "" {
		t.Fatalf("discovery changed disabled Claude intent: %#v", adapter)
	}
	if got := after.Clients[codex.ID]; !got.Enabled || got.Executable != "/existing/codex" || len(got.Targets) != 1 || got.Targets[0] != "/explicit/config.toml" {
		t.Fatalf("unselected Codex adapter changed: %#v", got)
	}
}

func TestDesiredClientConfigurationDoesNotReselectModels(t *testing.T) {
	before := domain.NewConfig(clients.Admission())
	before.Providers["one"] = domain.Provider{Label: "One", Endpoints: domain.Endpoints{Anthropic: "https://one.test"}}
	before.Providers["two"] = domain.Provider{Label: "Two", Endpoints: domain.Endpoints{Anthropic: "https://two.test"}}
	before.Models[domain.NewModelRef("one", "claude-test")] = domain.Model{Label: "One", Provider: "one", Family: "claude-test", ID: "claude-test", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	before.Models[domain.NewModelRef("two", "claude-test")] = domain.Model{Label: "Two", Provider: "two", Family: "claude-test", ID: "claude-test", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	before.SetSelectedModel(claude.ID, domain.NewModelRef("one", "claude-test"), "")
	secretStore := secret.NewMemoryStore()
	if err := secretStore.Set("two", "token"); err != nil {
		t.Fatal(err)
	}

	after, _, err := (Synchronizer{Registry: builtinClients, Secrets: secretStore, Discovery: staticDiscovery{}}).DesiredClientConfiguration(before)
	if err != nil {
		t.Fatal(err)
	}
	if got := after.SelectedModel(claude.ID); got != domain.NewModelRef("one", "claude-test") {
		t.Fatalf("client discovery reselected route = %q, want one", got)
	}
}

func TestDesiredSyncConfigurationDoesNotSearchUnselectedWritableCredentials(t *testing.T) {
	before := domain.NewConfig(clients.Admission())
	before.Providers["team"] = domain.Provider{Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	before.Models[domain.NewModelRef("team", "model")] = domain.Model{Provider: "team", Family: "model", ID: "model", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	before.SetRecommendedModel(claude.ID, domain.NewModelRef("team", "model"))
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
	before.Providers["team"] = domain.Provider{Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	want := errors.New("environment metadata unavailable")
	syncer := Synchronizer{Registry: builtinClients, Secrets: failingReadOnlyStore{secretReadStub{err: want}}, Discovery: staticDiscovery{}}
	after, _, err := syncer.DesiredSyncConfiguration(before)
	if !errors.Is(err, want) || len(after.Providers) != 0 {
		t.Fatalf("failed credential observation admitted a selected configuration: %#v, %v", after, err)
	}
}

func TestDesiredSyncConfigurationPreservesExplicitDisabledSelection(t *testing.T) {
	before := domain.NewConfig(clients.Admission())
	for _, account := range []string{"manual", "recommended"} {
		before.Providers[account] = domain.Provider{Endpoints: domain.Endpoints{Anthropic: "https://" + account + ".test"}}
		before.Models[domain.NewModelRef(account, "model")] = domain.Model{Provider: account, Family: "model", ID: "model", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	}
	before.SetRecommendedModel(claude.ID, domain.NewModelRef("recommended", "model"))
	before.SetSelectedModel(claude.ID, domain.NewModelRef("manual", "model"), "")
	store := secret.NewEnvironmentStore(func(key string) string {
		if key == secret.EnvironmentKey("recommended") {
			return "token"
		}
		return ""
	})
	after, _, err := (Synchronizer{Registry: builtinClients, Secrets: store, Discovery: staticDiscovery{}}).DesiredSyncConfiguration(before)
	if err != nil || after.SelectedModel(claude.ID) != domain.NewModelRef("manual", "model") || after.Clients[claude.ID].Enabled {
		t.Fatalf("environment recommendation overrode an explicit disabled selection: %#v, %v", after.Clients, err)
	}
}

func TestDesiredClientConfigurationSurfacesCredentialObservationFailures(t *testing.T) {
	before := domain.NewConfig(clients.Admission())
	before.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{
		Anthropic: "https://gateway.test",
		Responses: "https://gateway.test/v1",
	}}
	before.Models[domain.NewModelRef("gateway", "claude-test")] = domain.Model{Label: "Claude", Provider: "gateway", Family: "claude-test", ID: "claude-test", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	before.Models[domain.NewModelRef("gateway", "gpt-test")] = domain.Model{Label: "Codex", Provider: "gateway", Family: "gpt-test", ID: "gpt-test", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolResponses: {}}}
	before.SetSelectedModel(claude.ID, domain.NewModelRef("gateway", "claude-test"), "")
	before.SetSelectedModel(codex.ID, domain.NewModelRef("gateway", "gpt-test"), "")
	before.SetSelectedModel(claudedesktop.ID, domain.NewModelRef("gateway", "claude-test"), "")
	before.SetSelectedModel(hermes.ID, domain.NewModelRef("gateway", "claude-test"), "")
	want := errors.New("credential observation failed")
	syncer := Synchronizer{Registry: builtinClients, Secrets: secretReadStub{err: want}, Discovery: staticDiscovery{}}

	// These clients read the credential store before they check their
	// executable, so a store failure surfaces even without one.
	for _, client := range []string{claude.ID, codex.ID, claudedesktop.ID} {
		enabled := before.Clone()
		enabled.SetClientActivation(client, true, "", nil)
		if _, _, err := syncer.DesiredClientConfiguration(enabled, client); !errors.Is(err, want) {
			t.Fatalf("DesiredClientConfiguration(%q) error = %v, want %v", client, err, want)
		}
	}
	// Hermes activates only a runnable executable and checks it first, so
	// without one it neither reads the store nor activates.
	enabled := before.Clone()
	enabled.SetClientActivation(hermes.ID, true, "", nil)
	after, _, err := syncer.DesiredClientConfiguration(enabled, hermes.ID)
	if err != nil {
		t.Fatalf("DesiredClientConfiguration(hermes) error = %v, want no store read", err)
	}
	if binding := after.Clients[hermes.ID]; binding.Executable != "" || len(binding.Targets) != 0 {
		t.Fatalf("Hermes without a runnable executable was activated: %+v", binding)
	}
}

func TestDesiredClientConfigurationDoesNotActivateUnboundClients(t *testing.T) {
	before := domain.NewConfig(clients.Admission())
	before.Providers["team"] = domain.Provider{Label: "Team", Endpoints: domain.Endpoints{
		Anthropic: "https://team.test", Responses: "https://team.test/v1",
	}}
	before.Models[domain.NewModelRef("team", "claude-test")] = domain.Model{Label: "Claude", Provider: "team", Family: "claude-test", ID: "claude-test", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	before.Models[domain.NewModelRef("team", "gpt-test")] = domain.Model{Label: "Codex", Provider: "team", Family: "gpt-test", ID: "gpt-test", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolResponses: {}}}
	before.SetRecommendedModel(claude.ID, domain.NewModelRef("team", "claude-test"))
	store := secret.NewMemoryStore()
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
	projection, err := syncer.PreviewProjection(cfg, cfg)
	plans := projection.Targets
	if err != nil || len(plans) != 0 {
		t.Fatalf("disabled plan = %#v, %v", plans, err)
	}

	invalid := testConfig("/target")
	delete(invalid.Models, domain.NewModelRef("gateway", "gpt-test"))
	if _, err := (Synchronizer{Registry: builtinClients, Discovery: staticDiscovery{}}).PreviewProjection(invalid, invalid); err == nil {
		t.Fatal("planning accepted an invalid runtime")
	}
}

func TestPlanIncludesClaudeProjectionAndRestore(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	before := domain.NewConfig(clients.Admission())
	before.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
	before.Models[domain.NewModelRef("gateway", "claude-team")] = domain.Model{Label: "Claude", Provider: "gateway", Family: "claude-team", ID: "claude-team", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	before.SetSelectedModel(claude.ID, domain.NewModelRef("gateway", "claude-team"), "")
	after := before.Clone()
	after.SetClientActivation(claude.ID, true, "/opt/claude", nil)
	configStore := store.NewStore(filepath.Join(dir, "aigw.toml"), clients.Admission())
	if err := configStore.Save(before); err != nil {
		t.Fatal(err)
	}
	syncer := Synchronizer{Registry: builtinClients, Config: configStore, Discovery: staticDiscovery{}, Host: hostAt(dir), AIGWExecutable: filepath.Join(dir, "aigw")}

	projection, err := syncer.PreviewProjection(before, after)
	plans := projection.Targets
	if err != nil || len(plans) != 1 || plans[0].Client != claude.ID || plans[0].Target != settingsPath || plans[0].Action != "project" {
		t.Fatalf("project plans = %#v, %v", plans, err)
	}
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Fatalf("planning wrote Claude settings: %v", err)
	}
	if err := syncer.CommitProjection(t.Context(), before, after, "Claude projection"); err != nil {
		t.Fatal(err)
	}
	projection, err = syncer.PreviewProjection(after, before)
	plans = projection.Targets
	if err != nil || len(plans) != 1 || plans[0].Client != claude.ID || plans[0].Action != "restore" {
		t.Fatalf("restore plans = %#v, %v", plans, err)
	}
}

func TestCommitProjectionPreservesClaudeModelAcrossHelperChange(t *testing.T) {
	root := t.TempDir()
	settings := filepath.Join(root, ".claude", "settings.json")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
	cfg.Models[domain.NewModelRef("gateway", "claude-opus-5-5")] = domain.Model{Label: "Claude", Provider: "gateway", Family: "claude-opus-5-5", ID: "claude-opus-5-5", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.SetSelectedModel(claude.ID, domain.NewModelRef("gateway", "claude-opus-5-5"), "")
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
	projection, err := syncer.PreviewProjection(cfg, cfg)
	plans := projection.Targets
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
	before.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{Anthropic: "https://gateway.test"}}
	before.Models[domain.NewModelRef("gateway", "claude-team")] = domain.Model{Label: "Claude", Provider: "gateway", Family: "claude-team", ID: "claude-team", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	before.SetSelectedModel(claude.ID, domain.NewModelRef("gateway", "claude-team"), "")
	after := before.Clone()
	after.SetClientActivation(claude.ID, true, "/opt/claude", nil)

	if _, err := (Synchronizer{Registry: builtinClients, Discovery: staticDiscovery{}}).PreviewProjection(before, after); err == nil || !strings.Contains(err.Error(), "settings path") {
		t.Fatalf("missing settings path error = %v", err)
	}
	if _, err := (Synchronizer{Registry: builtinClients, Discovery: staticDiscovery{}, Host: hostAt(t.TempDir())}).PreviewProjection(before, after); err == nil || !strings.Contains(err.Error(), "executable path") {
		t.Fatalf("missing AIGW executable error = %v", err)
	}
	invalid := after.Clone()
	delete(invalid.Models, domain.NewModelRef("gateway", "claude-team"))
	if _, err := (Synchronizer{Registry: builtinClients, Discovery: staticDiscovery{}, Host: hostAt(t.TempDir()), AIGWExecutable: "/aigw"}).PreviewProjection(before, invalid); err == nil {
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
			account := before.Providers["gateway"]
			account.Endpoints.Anthropic = "https://gateway.test"
			before.Providers["gateway"] = account
			before.Models[domain.NewModelRef("gateway", "claude-original")] = domain.Model{Label: "Claude", Provider: "gateway", Family: "claude-original", ID: "claude-original", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
			before.SetSelectedModel(claude.ID, domain.NewModelRef("gateway", "claude-original"), "")
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
			account = after.Providers["gateway"]
			if selected == codex.ID {
				account.Endpoints.Responses = "https://replacement.test/v1"
			} else {
				account.Endpoints.Anthropic = "https://replacement.test"
			}
			after.Providers["gateway"] = account
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
			if err != nil || stored.Providers["gateway"].Endpoints != account.Endpoints {
				t.Fatalf("configuration did not commit: %#v, %v", stored.Providers, err)
			}
		})
	}
}

func TestProjectionPlanningRequiresDiscoveryAndValidTargets(t *testing.T) {
	base := testConfig("/target")
	if _, err := (Synchronizer{Registry: builtinClients}).PreviewProjection(base, base); err == nil {
		t.Fatal("expected discovery error")
	}
	syncer := Synchronizer{Registry: builtinClients, Discovery: staticDiscovery{}}
	before := base.Clone()
	before.SetClientActivation(codex.ID, true, "", []string{""})
	if _, err := syncer.PreviewProjection(before, base); err == nil {
		t.Fatal("expected before-target error")
	}
	after := base.Clone()
	after.SetClientActivation(codex.ID, true, "", []string{""})
	if _, err := syncer.PreviewProjection(base, after); err == nil {
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
	route := after.Models[domain.NewModelRef("gateway", "gpt-test")]
	route.Family = "openai.gpt-5.6-sol"
	after.Models[domain.NewModelRef("gateway", "gpt-test")] = route
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
