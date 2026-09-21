package client

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	goruntime "runtime"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/claude"
	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"
	"aigw-cli/internal/transaction"

	"github.com/pelletier/go-toml/v2"
)

type fixedDiscoverer struct{ result discovery.Result }

func (discoverer fixedDiscoverer) Discover() discovery.Result { return discoverer.result }

type captureAdapterRunner struct {
	err       error
	calls     int
	deadlines []bool
}

func (runner *captureAdapterRunner) RunCapture(ctx context.Context, _ process.Plan) ([]byte, error) {
	runner.calls++
	_, hasDeadline := ctx.Deadline()
	runner.deadlines = append(runner.deadlines, hasDeadline)
	return nil, runner.err
}

func TestBuiltInAdapterVerificationBoundsClientProcesses(t *testing.T) {
	codexConfig, codexRuntime := codexVerificationFixture(t)
	codexRunner := &captureAdapterRunner{err: errors.New("stop after deadline observation")}
	_, _ = (codexAdapter{}).Verify(context.Background(), Dependencies{Runner: codexRunner}, codexConfig, codexRuntime, "")
	if len(codexRunner.deadlines) == 0 || !codexRunner.deadlines[0] {
		t.Fatalf("Codex verification deadlines = %#v", codexRunner.deadlines)
	}

	claudeExecutable := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(claudeExecutable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	claudeConfig := configuration.NewConfig()
	claudeConfig.Accounts["gateway"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://gateway.test"}}
	claudeConfig.Profiles["claude"] = configuration.Profile{Account: "gateway", Model: "claude-test"}
	claudeConfig.SetSelectedProfile(configuration.ClientClaude, "claude")
	claudeConfig.SetClientActivation(configuration.ClientClaude, true, claudeExecutable, nil)
	claudeRuntime, err := claudeConfig.ResolveRuntime(configuration.ClientClaude, "")
	if err != nil {
		t.Fatal(err)
	}
	secretStore := secrets.NewMemoryStore()
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	claudeRunner := &captureAdapterRunner{err: errors.New("stop after deadline observation")}
	settings := filepath.Join(t.TempDir(), "settings.json")
	aigwExecutable := filepath.Join(t.TempDir(), "aigw")
	if _, err := claude.ReconcileSettings(settings, false, claudeRuntime, aigwExecutable, claudeRuntime.Model); err != nil {
		t.Fatal(err)
	}
	_, _ = (claudeAdapter{}).Verify(context.Background(), Dependencies{Runner: claudeRunner, Secrets: secretStore, ClaudeSettingsPath: settings, AIGWExecutable: aigwExecutable}, claudeConfig, claudeRuntime, "")
	if len(claudeRunner.deadlines) != 1 || !claudeRunner.deadlines[0] {
		t.Fatalf("Claude verification deadlines = %#v", claudeRunner.deadlines)
	}
}

func codexConfiguration(target string) configuration.Config {
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Endpoints: configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	cfg.Profiles["codex"] = configuration.Profile{Account: "gateway", Model: "gpt-test"}
	cfg.Clients[configuration.ClientCodex] = configuration.ClientBinding{Profile: "codex", Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
	return cfg
}

func codexVerificationFixture(t *testing.T) (configuration.Config, configuration.Runtime) {
	t.Helper()
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := codexConfiguration(target)
	configured := cfg.Clients[configuration.ClientCodex]
	configured.Executable = filepath.Join(t.TempDir(), "codex")
	cfg.Clients[configuration.ClientCodex] = configured
	if err := os.WriteFile(cfg.Clients[configuration.ClientCodex].Executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	runtime, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	runtime.CredentialCommand = filepath.Join(t.TempDir(), "aigw")
	if err := codex.SyncConfig(target, runtime); err != nil {
		t.Fatal(err)
	}
	return cfg, runtime
}

func TestCodexConvergencePreservesEnabledIntentWithoutDiscoveredTargets(t *testing.T) {
	cfg := codexConfiguration("/explicit/config.toml")
	before := cfg.Clients[configuration.ClientCodex]

	if err := (codexAdapter{}).Converge(Dependencies{}, &cfg, discovery.Result{}); err != nil {
		t.Fatal(err)
	}
	if got := cfg.Clients[configuration.ClientCodex]; !reflect.DeepEqual(got, before) {
		t.Fatalf("Codex convergence changed explicit intent without a discovered target: got %#v want %#v", got, before)
	}
}

func TestCodexAdapterReportsReadinessStates(t *testing.T) {
	newFixture := func(t *testing.T, provider string) (configuration.Config, configuration.Runtime) {
		t.Helper()
		target := filepath.Join(t.TempDir(), "config.toml")
		if err := os.WriteFile(target, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		cfg := configuration.NewConfig()
		cfg.Accounts["gateway"] = configuration.Account{Endpoints: configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
		cfg.Profiles["codex"] = configuration.Profile{Account: "gateway", Model: "gpt-test"}
		cfg.Clients[configuration.ClientCodex] = configuration.ClientBinding{Profile: "codex", Enabled: true, ModelProvider: provider, Executable: "/usr/bin/codex", Targets: []string{target}}
		runtime, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
		if err != nil {
			t.Fatal(err)
		}
		runtime.CredentialCommand = filepath.Join(t.TempDir(), "aigw")
		if err := codex.SyncConfig(target, runtime); err != nil {
			t.Fatal(err)
		}
		return cfg, runtime
	}

	t.Run("missing executable", func(t *testing.T) {
		cfg, runtime := newFixture(t, configuration.ModelProviderAIGW)
		adapter := cfg.Clients[configuration.ClientCodex]
		adapter.Executable = ""
		cfg.Clients[configuration.ClientCodex] = adapter
		status := (codexAdapter{}).Inspect(context.Background(), Dependencies{}, cfg, runtime)
		if status.Ready || !strings.Contains(status.Issue, "executable") {
			t.Fatalf("status = %#v", status)
		}
	})

	t.Run("missing target", func(t *testing.T) {
		cfg, runtime := newFixture(t, configuration.ModelProviderAIGW)
		adapter := cfg.Clients[configuration.ClientCodex]
		adapter.Targets = nil
		cfg.Clients[configuration.ClientCodex] = adapter
		status := (codexAdapter{}).Inspect(context.Background(), Dependencies{}, cfg, runtime)
		if status.Ready || !strings.Contains(status.Issue, "target") {
			t.Fatalf("status = %#v", status)
		}
	})

	t.Run("projection drift", func(t *testing.T) {
		cfg, runtime := newFixture(t, configuration.ModelProviderAIGW)
		adapter := cfg.Clients[configuration.ClientCodex]
		adapter.Targets = []string{filepath.Join(t.TempDir(), "missing.toml")}
		cfg.Clients[configuration.ClientCodex] = adapter
		status := (codexAdapter{}).Inspect(context.Background(), Dependencies{}, cfg, runtime)
		if status.Ready || !strings.Contains(status.Issue, "projection drift") {
			t.Fatalf("status = %#v", status)
		}
	})

	t.Run("external provider", func(t *testing.T) {
		cfg, runtime := newFixture(t, "amazon-bedrock")
		status := (codexAdapter{}).Inspect(context.Background(), Dependencies{}, cfg, runtime)
		if !status.Ready {
			t.Fatalf("status = %#v", status)
		}
	})

	t.Run("capture unavailable", func(t *testing.T) {
		cfg, runtime := newFixture(t, configuration.ModelProviderAIGW)
		status := (codexAdapter{}).Inspect(context.Background(), Dependencies{}, cfg, runtime)
		if !status.Ready {
			t.Fatalf("status = %#v", status)
		}
	})

	t.Run("inspection does not invoke the client", func(t *testing.T) {
		cfg, runtime := newFixture(t, configuration.ModelProviderAIGW)
		runner := &captureAdapterRunner{err: errors.New("status failed")}
		status := (codexAdapter{}).Inspect(context.Background(), Dependencies{Runner: runner}, cfg, runtime)
		if !status.Ready || runner.calls != 0 {
			t.Fatalf("status = %#v, calls = %d", status, runner.calls)
		}
	})
}

func TestCodexAdapterApplyRequiresDiscovery(t *testing.T) {
	after := configuration.NewConfig()
	after.SetClientActivation(configuration.ClientCodex, true, "", nil)
	if _, err := (codexAdapter{}).Apply(context.Background(), Dependencies{}, configuration.NewConfig(), after); err == nil || !strings.Contains(err.Error(), "surface discovery is unavailable") {
		t.Fatalf("Apply() error = %v", err)
	}
}

func TestCodexAdapterProjectsCredentialHelperOnlyForAccountTokenProviders(t *testing.T) {
	for _, test := range []struct {
		name           string
		provider       string
		authentication configuration.Authentication
		wantCommand    bool
	}{
		{name: "aigw account token", provider: configuration.ModelProviderAIGW, authentication: configuration.AuthenticationAccountToken, wantCommand: true},
		{name: "provider account token", provider: "amazon-bedrock", authentication: configuration.AuthenticationAccountToken, wantCommand: true},
		{name: "client native", provider: "amazon-bedrock", authentication: configuration.AuthenticationClientNative},
	} {
		t.Run(test.name, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "config.toml")
			cfg := configuration.NewConfig()
			cfg.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
			cfg.Profiles["codex"] = configuration.Profile{
				Label:   "Codex",
				Account: "gateway",
				Model:   "gpt-test",
			}
			cfg.Clients[configuration.ClientCodex] = configuration.ClientBinding{
				Profile: "codex", Enabled: true, ModelProvider: test.provider,
				Authentication: test.authentication, Executable: "/usr/bin/codex", Targets: []string{target},
			}
			discovered := discovery.Result{
				Executables: map[string]string{configuration.ClientCodex: "/usr/bin/codex"},
				Surfaces: []discovery.Surface{{
					ID:          "codex-home-default",
					Authority:   "aigw",
					ConfigPath:  target,
					AutoManaged: true,
				}},
			}
			command := filepath.Join(t.TempDir(), "aigw")
			_, err := (codexAdapter{}).Apply(context.Background(), Dependencies{
				Discovery:      fixedDiscoverer{result: discovered},
				AIGWExecutable: command,
			}, configuration.NewConfig(), cfg)
			if err != nil {
				t.Fatal(err)
			}
			projected, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			var document struct {
				ModelProviders map[string]struct {
					Auth struct {
						Command string `toml:"command"`
					} `toml:"auth"`
				} `toml:"model_providers"`
			}
			if err := toml.Unmarshal(projected, &document); err != nil {
				t.Fatalf("decode Codex projection: %v", err)
			}
			hasCommand := document.ModelProviders[test.provider].Auth.Command == command
			if hasCommand != test.wantCommand {
				t.Fatalf("credential command present = %t, want %t:\n%s", hasCommand, test.wantCommand, projected)
			}
		})
	}
}

func TestClaudeAdapterApplyValidatesIntentAndRestoresObservedPreimage(t *testing.T) {
	enabledWithoutRoute := configuration.NewConfig()
	enabledWithoutRoute.SetClientActivation(configuration.ClientClaude, true, "/usr/bin/claude", nil)
	if _, err := (claudeAdapter{}).Apply(context.Background(), Dependencies{}, configuration.NewConfig(), enabledWithoutRoute); err == nil {
		t.Fatal("Apply() accepted an enabled adapter without a route")
	}

	configured := configuration.NewConfig()
	configured.Accounts["gateway"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://gateway.test"}}
	configured.Profiles["claude"] = configuration.Profile{Account: "gateway", Model: "claude-test"}
	configured.SetSelectedProfile(configuration.ClientClaude, "claude")
	configured.SetClientActivation(configuration.ClientClaude, true, "/usr/bin/claude", nil)
	if _, err := (claudeAdapter{}).Apply(context.Background(), Dependencies{}, configuration.NewConfig(), configured); err == nil || !strings.Contains(err.Error(), "settings path is empty") {
		t.Fatalf("Apply() error = %v", err)
	}

	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	receipt, err := (claudeAdapter{}).Apply(context.Background(), Dependencies{
		ClaudeSettingsPath: settingsPath,
		AIGWExecutable:     filepath.Join(t.TempDir(), "aigw"),
	}, enabledWithoutRoute, configured)
	if err != nil {
		t.Fatal(err)
	}
	if err := receipt.Rollback(); err != nil {
		t.Fatalf("Rollback() must restore observed files without reinterpreting prior intent: %v", err)
	}
	for _, path := range []string{settingsPath, settingsPath + ".aigw-state.json"} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("projection artifact remains after rollback: %s: %v", path, err)
		}
	}
}

func TestClaudeVerificationRequiresTheSynchronizedProjection(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "claude")
	if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := configuration.NewConfig()
	cfg.SetClientActivation(configuration.ClientClaude, true, executable, nil)
	store := secrets.NewMemoryStore()
	if err := store.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	runner := &captureAdapterRunner{err: errors.New("client must not start")}
	deps := Dependencies{Runner: runner, Secrets: store, ClaudeSettingsPath: filepath.Join(root, "settings.json"), AIGWExecutable: filepath.Join(root, "aigw")}
	runtime := configuration.Runtime{AccountID: "gateway", ProfileID: "claude", Model: "claude-model", Endpoint: "https://gateway.test"}
	_, err := (claudeAdapter{}).Verify(context.Background(), deps, cfg, runtime, "")
	if err == nil || !strings.Contains(err.Error(), "not synchronized") || runner.calls != 0 {
		t.Fatalf("unsynchronized projection: calls=%d error=%v", runner.calls, err)
	}
}

func TestClaudeInspectionRequiresTheSynchronizedProjection(t *testing.T) {
	tests := []struct {
		name     string
		sync     bool
		model    string
		settings string
		ready    bool
	}{
		{name: "missing", model: "claude-model"},
		{name: "converged", sync: true, model: "claude-model", ready: true},
		{name: "selected model changed", sync: true, model: "another-model"},
		{name: "malformed", sync: true, model: "claude-model", settings: "{"},
		{name: "externally changed", sync: true, model: "claude-model", settings: `{"model":"external-model"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			executable := filepath.Join(root, "claude")
			if goruntime.GOOS == "windows" {
				executable += ".exe"
			}
			if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
				t.Fatal(err)
			}
			cfg := configuration.NewConfig()
			cfg.SetClientActivation(configuration.ClientClaude, true, executable, nil)
			deps := Dependencies{ClaudeSettingsPath: filepath.Join(root, "settings.json"), AIGWExecutable: filepath.Join(root, "aigw")}
			runtime := configuration.Runtime{AccountID: "gateway", ProfileID: "claude", Model: "claude-model", Endpoint: "https://gateway.test"}
			if test.sync {
				if _, err := claude.ReconcileSettings(deps.ClaudeSettingsPath, false, runtime, deps.AIGWExecutable, runtime.Model); err != nil {
					t.Fatal(err)
				}
			}
			runtime.Model = test.model
			if test.settings != "" {
				if err := os.WriteFile(deps.ClaudeSettingsPath, []byte(test.settings), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			files := map[string]transaction.FileSnapshot{}
			for _, path := range []string{deps.ClaudeSettingsPath, deps.ClaudeSettingsPath + ".aigw-state.json"} {
				data, err := transaction.CaptureFileSnapshot(path)
				if err != nil {
					t.Fatal(err)
				}
				files[path] = data
			}
			status := (claudeAdapter{}).Inspect(context.Background(), deps, cfg, runtime)
			if status.Ready != test.ready {
				t.Errorf("inspection = %+v, want ready=%t", status, test.ready)
			}
			if !test.ready && (!strings.Contains(status.Issue, "not synchronized") || status.RepairAction != "aigw sync") {
				t.Errorf("missing synchronization diagnosis: %+v", status)
			}
			for path, before := range files {
				after, err := transaction.CaptureFileSnapshot(path)
				if err != nil || !after.Equal(before) {
					t.Errorf("inspection changed %s: %v", path, err)
				}
			}
		})
	}
}

func TestClaudeAdapterReportsExecutableAndSecretFailures(t *testing.T) {
	runtime := configuration.Runtime{AccountID: "gateway"}
	secretStore := secrets.NewMemoryStore()
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	dependencies := Dependencies{Secrets: secretStore}
	cfg := configuration.NewConfig()
	if _, err := (claudeAdapter{}).Verify(context.Background(), dependencies, cfg, runtime, ""); err == nil || !strings.Contains(err.Error(), "adapter is disabled") {
		t.Fatalf("Verify() disabled error = %v", err)
	}
	cfg.SetClientActivation(configuration.ClientClaude, true, "", nil)
	if _, err := (claudeAdapter{}).Verify(context.Background(), dependencies, cfg, runtime, ""); err == nil || !strings.Contains(err.Error(), "adapter is disabled") {
		t.Fatalf("Verify() unconfigured error = %v", err)
	}

	loop := filepath.Join(t.TempDir(), "loop")
	if err := os.Symlink(loop, loop); err != nil {
		t.Skipf("symbolic link unavailable: %v", err)
	}
	cfg.SetClientActivation(configuration.ClientClaude, true, loop, nil)
	status := (claudeAdapter{}).Inspect(context.Background(), Dependencies{}, cfg, configuration.Runtime{})
	if status.Ready || status.Issue != "Cannot inspect Claude executable" {
		t.Fatalf("status = %#v", status)
	}

	if _, err := (claudeAdapter{}).Verify(context.Background(), Dependencies{}, cfg, runtime, ""); err == nil || !strings.Contains(err.Error(), "secret store is unavailable") {
		t.Fatalf("Verify() error = %v", err)
	}
	if _, err := (claudeAdapter{}).Verify(context.Background(), dependencies, cfg, runtime, ""); err == nil || !strings.Contains(err.Error(), "inspect Claude executable") {
		t.Fatalf("Verify() inspection error = %v", err)
	}
	cfg.SetClientActivation(configuration.ClientClaude, true, filepath.Join(t.TempDir(), "missing"), nil)
	if _, err := (claudeAdapter{}).Verify(context.Background(), dependencies, cfg, runtime, ""); err == nil || !strings.Contains(err.Error(), "executable is unavailable") {
		t.Fatalf("Verify() unavailable error = %v", err)
	}
}

func TestBuiltInRegistryInvariantsPanicOnInvalidDefinitions(t *testing.T) {
	tests := []struct {
		name string
		run  func()
	}{
		{name: "invalid registry", run: func() { mustRegistry(nil, failingProjectionAdapter{id: "unadmitted"}) }},
		{name: "missing client", run: func() { mustClientSpec("missing") }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("invariant violation did not panic")
				}
			}()
			test.run()
		})
	}
}

func TestChangedClientsFollowPersistentCodexSemantics(t *testing.T) {
	target := filepath.Join(t.TempDir(), "configuration.toml")
	before := codexConfiguration(target)

	disabled := before.Clone()
	delete(disabled.Clients, configuration.ClientCodex)
	adapterRegistry := DefaultRegistry()
	if changed := adapterRegistry.ChangedClients(before, disabled); len(changed) != 1 || changed[0] != configuration.ClientCodex {
		t.Fatalf("adapter removal scope = %v, want Codex only", changed)
	}

	purpose := before.Clone()
	profile := purpose.Profiles["codex"]
	profile.Purpose = "display only"
	purpose.Profiles["codex"] = profile
	if len(adapterRegistry.ChangedClients(before, purpose)) != 0 {
		t.Fatal("display-only purpose must not change the projection")
	}
}

func TestChangedClientsRetainsAdmissionOrderAndIndependentResults(t *testing.T) {
	before := codexConfiguration("/target")
	account := before.Accounts["gateway"]
	account.Endpoints.Anthropic = "https://gateway.test"
	before.Accounts["gateway"] = account
	before.Profiles["claude"] = configuration.Profile{Account: "gateway", Model: "claude-test"}
	before.SetSelectedProfile(configuration.ClientClaude, "claude")
	before.SetClientActivation(configuration.ClientClaude, true, "/opt/claude", nil)
	after := before.Clone()
	account.Endpoints = configuration.Endpoints{OpenAIResponses: "https://replacement.test/v1", Anthropic: "https://replacement.test"}
	after.Accounts["gateway"] = account
	registry := DefaultRegistry()
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

func TestProjectionErrorAndInvalidRuntimeBranches(t *testing.T) {
	t.Run("invalid enabled Codex runtime", func(t *testing.T) {
		before := codexConfiguration("/target")
		after := before.Clone()
		delete(before.Profiles, "codex")
		delete(after.Profiles, "codex")
		if changed := DefaultRegistry().ChangedClients(before, after); len(changed) != 1 || changed[0] != configuration.ClientCodex {
			t.Fatal("invalid Codex runtime was treated as unchanged")
		}
	})

	t.Run("Codex targets changed", func(t *testing.T) {
		before := codexConfiguration("/first")
		after := before.Clone()
		after.SetClientActivation(configuration.ClientCodex, true, "/opt/codex", []string{"/second"})
		if changed := DefaultRegistry().ChangedClients(before, after); len(changed) != 1 || changed[0] != configuration.ClientCodex {
			t.Fatal("changed Codex target set was treated as unchanged")
		}
	})

	t.Run("invalid enabled Claude runtime", func(t *testing.T) {
		before := configuration.NewConfig()
		before.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{Anthropic: "https://gateway.test"}}
		before.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "gateway", Model: "claude-test"}
		before.SetSelectedProfile(configuration.ClientClaude, "claude")
		before.SetClientActivation(configuration.ClientClaude, true, "/opt/claude", nil)
		after := before.Clone()
		delete(before.Profiles, "claude")
		delete(after.Profiles, "claude")
		if changed := DefaultRegistry().ChangedClients(before, after); len(changed) != 1 || changed[0] != configuration.ClientClaude {
			t.Fatal("invalid Claude runtime was treated as unchanged")
		}
	})
}

func TestExternalCredentialRunnerPreservesCancellationWithoutDiagnostics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runner := externalCredentialRunner{runner: &captureAdapterRunner{err: errors.New("public-secret-marker")}}
	out, err := runner.RunCapture(ctx, process.Plan{})
	if !errors.Is(err, context.Canceled) || len(out) != 0 {
		t.Fatalf("cancellation=%v, output bytes=%d", err, len(out))
	}
}
