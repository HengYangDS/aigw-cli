package synchronization

import (
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/secrets"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesiredClientConfigurationScopesDiscoveryToRequestedClient(t *testing.T) {
	claudeExecutable := filepath.Join(t.TempDir(), "claude")
	before := configuration.NewConfig()
	before.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{
		Anthropic:       "https://gateway.test",
		OpenAIResponses: "https://gateway.test/v1",
	}}
	before.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "gateway", Client: configuration.ClientClaude, Model: "claude-test"}
	before.Profiles["codex"] = configuration.Profile{Label: "Codex", Account: "gateway", Client: configuration.ClientCodex, Model: "gpt-test"}
	before.Routes[configuration.ClientClaude] = "claude"
	before.Routes[configuration.ClientCodex] = "codex"
	before.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/existing/codex", Targets: []string{"/explicit/config.toml"}}
	secretStore := secrets.NewMemoryStore()
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	syncer := Synchronizer{Secrets: secretStore, Discovery: staticDiscovery{result: discovery.Result{Executables: map[string]string{
		configuration.ClientClaude: claudeExecutable,
	}}}}

	after, _, err := syncer.DesiredClientConfiguration(before, configuration.ClientClaude)
	if err != nil {
		t.Fatal(err)
	}
	if adapter := after.Adapters[configuration.ClientClaude]; !adapter.Enabled || adapter.Executable != claudeExecutable {
		t.Fatalf("Claude adapter = %#v", adapter)
	}
	if got := after.Adapters[configuration.ClientCodex]; !got.Enabled || got.Executable != "/existing/codex" || len(got.Targets) != 1 || got.Targets[0] != "/explicit/config.toml" {
		t.Fatalf("unselected Codex adapter changed: %#v", got)
	}
}

func TestDesiredClientConfigurationDoesNotReselectRoutes(t *testing.T) {
	before := configuration.NewConfig()
	before.Accounts["one"] = configuration.Account{Label: "One", Endpoints: configuration.Endpoints{Anthropic: "https://one.test"}}
	before.Accounts["two"] = configuration.Account{Label: "Two", Endpoints: configuration.Endpoints{Anthropic: "https://two.test"}}
	before.Profiles["one"] = configuration.Profile{Label: "One", Account: "one", Client: configuration.ClientClaude, Model: "claude-test"}
	before.Profiles["two"] = configuration.Profile{Label: "Two", Account: "two", Client: configuration.ClientClaude, Model: "claude-test"}
	before.Routes[configuration.ClientClaude] = "one"
	secretStore := secrets.NewMemoryStore()
	if err := secretStore.Set("two", "token"); err != nil {
		t.Fatal(err)
	}

	after, _, err := (Synchronizer{Secrets: secretStore, Discovery: staticDiscovery{}}).DesiredClientConfiguration(before)
	if err != nil {
		t.Fatal(err)
	}
	if got := after.Routes[configuration.ClientClaude]; got != "one" {
		t.Fatalf("client discovery reselected route = %q, want one", got)
	}
}

func TestDesiredClientConfigurationSurfacesCredentialObservationFailures(t *testing.T) {
	before := configuration.NewConfig()
	before.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{
		Anthropic:       "https://gateway.test",
		OpenAIResponses: "https://gateway.test/v1",
	}}
	before.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "gateway", Client: configuration.ClientClaude, Model: "claude-test"}
	before.Profiles["codex"] = configuration.Profile{Label: "Codex", Account: "gateway", Client: configuration.ClientCodex, Model: "gpt-test"}
	before.Routes[configuration.ClientClaude] = "claude"
	before.Routes[configuration.ClientCodex] = "codex"
	want := errors.New("credential observation failed")
	syncer := Synchronizer{Secrets: secretReadStub{err: want}, Discovery: staticDiscovery{}}

	for _, client := range []string{configuration.ClientClaude, configuration.ClientCodex} {
		if _, _, err := syncer.DesiredClientConfiguration(before, client); !errors.Is(err, want) {
			t.Fatalf("DesiredClientConfiguration(%q) error = %v, want %v", client, err, want)
		}
	}
}

func TestProjectionPlanningValidatesEnabledClients(t *testing.T) {
	syncer := Synchronizer{}
	cfg := configuration.NewConfig()
	plans, err := syncer.Plan(cfg, cfg)
	if err != nil || len(plans) != 0 {
		t.Fatalf("disabled plan = %#v, %v", plans, err)
	}

	invalid := testConfig("/target")
	delete(invalid.Profiles, "gpt")
	if _, err := (Synchronizer{Discovery: staticDiscovery{}}).Plan(invalid, invalid); err == nil {
		t.Fatal("planning accepted an invalid runtime")
	}
}

func TestPlanIncludesClaudeProjectionAndRestore(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	before := configuration.NewConfig()
	before.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{Anthropic: "https://gateway.test"}}
	before.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "gateway", Client: configuration.ClientClaude, Model: "claude-team"}
	before.Routes[configuration.ClientClaude] = "claude"
	after := before.Clone()
	after.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/claude"}
	syncer := Synchronizer{Config: configuration.NewStore(filepath.Join(dir, "aigw.toml")), Discovery: staticDiscovery{}, ClaudeSettingsPath: settingsPath, AIGWExecutable: filepath.Join(dir, "aigw")}

	plans, err := syncer.Plan(before, after)
	if err != nil || len(plans) != 1 || plans[0].Client != configuration.ClientClaude || plans[0].Target != settingsPath || plans[0].Action != "project" {
		t.Fatalf("project plans = %#v, %v", plans, err)
	}
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Fatalf("planning wrote Claude settings: %v", err)
	}
	if err := syncer.CommitProjection(t.Context(), before, after, "Claude projection"); err != nil {
		t.Fatal(err)
	}
	plans, err = syncer.Plan(after, before)
	if err != nil || len(plans) != 1 || plans[0].Client != configuration.ClientClaude || plans[0].Action != "restore" {
		t.Fatalf("restore plans = %#v, %v", plans, err)
	}
}

func TestPlanReportsClaudePlanningFailures(t *testing.T) {
	before := configuration.NewConfig()
	before.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{Anthropic: "https://gateway.test"}}
	before.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "gateway", Client: configuration.ClientClaude, Model: "claude-team"}
	before.Routes[configuration.ClientClaude] = "claude"
	after := before.Clone()
	after.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/claude"}

	if _, err := (Synchronizer{Discovery: staticDiscovery{}}).Plan(before, after); err == nil || !strings.Contains(err.Error(), "settings path") {
		t.Fatalf("missing settings path error = %v", err)
	}
	if _, err := (Synchronizer{Discovery: staticDiscovery{}, ClaudeSettingsPath: "/settings"}).Plan(before, after); err == nil || !strings.Contains(err.Error(), "executable path") {
		t.Fatalf("missing AIGW executable error = %v", err)
	}
	invalid := after.Clone()
	delete(invalid.Profiles, "claude")
	if _, err := (Synchronizer{Discovery: staticDiscovery{}, ClaudeSettingsPath: "/settings", AIGWExecutable: "/aigw"}).Plan(before, invalid); err == nil {
		t.Fatal("invalid enabled Claude runtime was accepted")
	}
}

func TestCommitReconcilesOnlyClientsWhoseProjectionChanges(t *testing.T) {
	for _, selected := range configuration.AdmittedClientIDs() {
		t.Run(selected, func(t *testing.T) {
			root := t.TempDir()
			targets := map[string]string{
				configuration.ClientCodex:  filepath.Join(root, "codex.toml"),
				configuration.ClientClaude: filepath.Join(root, "claude.json"),
			}
			before := testConfig(targets[configuration.ClientCodex])
			account := before.Accounts["gateway"]
			account.Endpoints.Anthropic = "https://gateway.test"
			before.Accounts["gateway"] = account
			before.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "gateway", Client: configuration.ClientClaude, Model: "claude-original"}
			before.Routes[configuration.ClientClaude] = "claude"
			before.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/claude"}
			store := configuration.NewStore(filepath.Join(root, "aigw.toml"))
			syncer := Synchronizer{Config: store, Discovery: targetDiscovery(targets[configuration.ClientCodex]), ClaudeSettingsPath: targets[configuration.ClientClaude], AIGWExecutable: filepath.Join(root, "aigw")}
			if err := syncer.CommitProjection(t.Context(), configuration.NewConfig(), before, "initial projection"); err != nil {
				t.Fatal(err)
			}
			unselected := configuration.ClientClaude
			model := "claude-original"
			if selected == configuration.ClientClaude {
				unselected = configuration.ClientCodex
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
			if selected == configuration.ClientCodex {
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
			stored, err := store.Load()
			if err != nil || stored.Accounts["gateway"].Endpoints != account.Endpoints {
				t.Fatalf("configuration did not commit: %#v, %v", stored.Accounts, err)
			}
		})
	}
}

func TestProjectionPlanningRequiresDiscoveryAndValidTargets(t *testing.T) {
	base := testConfig("/target")
	if _, err := (Synchronizer{}).Plan(base, base); err == nil {
		t.Fatal("expected discovery error")
	}
	syncer := Synchronizer{Discovery: staticDiscovery{}}
	before := base.Clone()
	before.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Targets: []string{""}}
	if _, err := syncer.Plan(before, base); err == nil {
		t.Fatal("expected before-target error")
	}
	after := base.Clone()
	after.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Targets: []string{""}}
	if _, err := syncer.Plan(base, after); err == nil {
		t.Fatal("expected after-target error")
	}
}

func TestReconcileClientHonorsCancellationBeforeObservation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := (Synchronizer{}).ReconcileClient(ctx, configuration.NewConfig(), configuration.ClientClaude); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled projection = %v", err)
	}
}

func TestCommitProjectionDoesNotBindNativeAuthentication(t *testing.T) {
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := testConfig(target)
	delete(before.Adapters, configuration.ClientCodex)
	after := testConfig(target)
	store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
	if err := store.Save(before); err != nil {
		t.Fatal(err)
	}
	runner := &recordingRunner{}
	syncer := Synchronizer{
		Config:         store,
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
	stored, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !stored.Adapters[configuration.ClientCodex].Enabled {
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
	store := configuration.NewStore(filepath.Join(t.TempDir(), "aigw.toml"))
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	runner := &recordingRunner{}
	syncer := Synchronizer{Config: store, Discovery: targetDiscovery(target), Runner: runner, AIGWExecutable: filepath.Join(t.TempDir(), "aigw")}
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
	profile := after.Profiles["gpt"]
	profile.ModelProvider = "amazon-bedrock"
	profile.Authentication = configuration.AuthenticationClientNative
	profile.Model = "openai.gpt-5.6-sol"
	after.Profiles["gpt"] = profile

	syncer := Synchronizer{
		Config:    configuration.NewStore(filepath.Join(t.TempDir(), "aigw.toml")),
		Discovery: targetDiscovery(target), AIGWExecutable: credentialCommand,
	}
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
