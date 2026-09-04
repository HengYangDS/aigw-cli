package client

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"
)

type runOnlyAdapterRunner struct{}

func (runOnlyAdapterRunner) Run(context.Context, process.Plan) error { return nil }

type captureAdapterRunner struct {
	err       error
	calls     int
	deadlines []bool
}

func (*captureAdapterRunner) Run(context.Context, process.Plan) error { return nil }

func (runner *captureAdapterRunner) RunCapture(ctx context.Context, _ process.Plan) ([]byte, error) {
	runner.calls++
	_, hasDeadline := ctx.Deadline()
	runner.deadlines = append(runner.deadlines, hasDeadline)
	return nil, runner.err
}

func TestBuiltInAdapterVerificationBoundsClientProcesses(t *testing.T) {
	codexConfig, codexRuntime := codexVerificationFixture(t)
	codexRunner := &captureAdapterRunner{err: errors.New("stop after deadline observation")}
	_, _ = (codexAdapter{}).Verify(context.Background(), Dependencies{Runner: codexRunner}, codexConfig, codexRuntime)
	if len(codexRunner.deadlines) == 0 || !codexRunner.deadlines[0] {
		t.Fatalf("Codex verification deadlines = %#v", codexRunner.deadlines)
	}

	claudeExecutable := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(claudeExecutable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	claudeConfig := configuration.NewConfig()
	claudeConfig.Accounts["gateway"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://gateway.test"}}
	claudeConfig.Profiles["claude"] = configuration.Profile{Account: "gateway", Client: configuration.ClientClaude, Model: "claude-test"}
	claudeConfig.Routes[configuration.ClientClaude] = "claude"
	claudeConfig.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: claudeExecutable}
	claudeRuntime, err := claudeConfig.ResolveRuntime(configuration.ClientClaude, "")
	if err != nil {
		t.Fatal(err)
	}
	secretStore := secrets.NewMemoryStore()
	if err := secretStore.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	claudeRunner := &captureAdapterRunner{err: errors.New("stop after deadline observation")}
	_, _ = (claudeAdapter{}).Verify(context.Background(), Dependencies{Runner: claudeRunner, Secrets: secretStore}, claudeConfig, claudeRuntime)
	if len(claudeRunner.deadlines) != 1 || !claudeRunner.deadlines[0] {
		t.Fatalf("Claude verification deadlines = %#v", claudeRunner.deadlines)
	}
}

func codexVerificationFixture(t *testing.T) (configuration.Config, configuration.Runtime) {
	t.Helper()
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Endpoints: configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	cfg.Profiles["codex"] = configuration.Profile{Account: "gateway", Client: configuration.ClientCodex, Model: "gpt-test"}
	cfg.Routes[configuration.ClientCodex] = "codex"
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: filepath.Join(t.TempDir(), "codex"), Targets: []string{target}}
	if err := os.WriteFile(cfg.Adapters[configuration.ClientCodex].Executable, []byte("fixture"), 0o700); err != nil {
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

func TestCodexAdapterReportsNativeAuthenticationStates(t *testing.T) {
	newFixture := func(t *testing.T, provider string) (configuration.Config, configuration.Runtime) {
		t.Helper()
		target := filepath.Join(t.TempDir(), "config.toml")
		if err := os.WriteFile(target, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		cfg := configuration.NewConfig()
		cfg.Accounts["gateway"] = configuration.Account{Endpoints: configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
		cfg.Profiles["codex"] = configuration.Profile{Account: "gateway", Client: configuration.ClientCodex, Model: "gpt-test", ModelProvider: provider}
		cfg.Routes[configuration.ClientCodex] = "codex"
		cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/usr/bin/codex", Targets: []string{target}}
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

	t.Run("external provider", func(t *testing.T) {
		cfg, runtime := newFixture(t, "amazon-bedrock")
		status := (codexAdapter{}).Inspect(context.Background(), Dependencies{}, cfg, runtime, InspectionOptions{NativeAuthentication: true})
		if !status.Ready || status.NativeAuthentication != "not_required" {
			t.Fatalf("status = %#v", status)
		}
	})

	t.Run("capture unavailable", func(t *testing.T) {
		cfg, runtime := newFixture(t, configuration.ModelProviderAIGW)
		status := (codexAdapter{}).Inspect(context.Background(), Dependencies{Runner: runOnlyAdapterRunner{}}, cfg, runtime, InspectionOptions{NativeAuthentication: true})
		if !status.Ready || status.NativeAuthentication != "not_proven" {
			t.Fatalf("status = %#v", status)
		}
	})

	t.Run("capture failure", func(t *testing.T) {
		cfg, runtime := newFixture(t, configuration.ModelProviderAIGW)
		runner := &captureAdapterRunner{err: errors.New("status failed")}
		status := (codexAdapter{}).Inspect(context.Background(), Dependencies{Runner: runner}, cfg, runtime, InspectionOptions{NativeAuthentication: true})
		if !status.Ready || status.NativeAuthentication != "not_proven" || runner.calls != 1 {
			t.Fatalf("status = %#v, calls = %d", status, runner.calls)
		}
	})
}

func TestCodexAdapterApplyRequiresDiscovery(t *testing.T) {
	after := configuration.NewConfig()
	after.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true}
	if _, err := (codexAdapter{}).Apply(context.Background(), Dependencies{}, configuration.NewConfig(), after); err == nil || !strings.Contains(err.Error(), "surface discovery is unavailable") {
		t.Fatalf("Apply() error = %v", err)
	}
}

func TestClaudeAdapterApplyReportsPreparationAndRollbackFailures(t *testing.T) {
	enabledWithoutRoute := configuration.NewConfig()
	enabledWithoutRoute.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true}
	if _, err := (claudeAdapter{}).Apply(context.Background(), Dependencies{}, configuration.NewConfig(), enabledWithoutRoute); err == nil {
		t.Fatal("Apply() accepted an enabled adapter without a route")
	}

	configured := configuration.NewConfig()
	configured.Accounts["gateway"] = configuration.Account{Endpoints: configuration.Endpoints{Anthropic: "https://gateway.test"}}
	configured.Profiles["claude"] = configuration.Profile{Account: "gateway", Client: configuration.ClientClaude, Model: "claude-test"}
	configured.Routes[configuration.ClientClaude] = "claude"
	configured.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "/usr/bin/claude"}
	if _, err := (claudeAdapter{}).Apply(context.Background(), Dependencies{}, configuration.NewConfig(), configured); err == nil || !strings.Contains(err.Error(), "settings path is empty") {
		t.Fatalf("Apply() error = %v", err)
	}

	receipt, err := (claudeAdapter{}).Apply(context.Background(), Dependencies{
		ClaudeSettingsPath: filepath.Join(t.TempDir(), "settings.json"),
		AIGWExecutable:     filepath.Join(t.TempDir(), "aigw"),
	}, enabledWithoutRoute, configured)
	if err != nil {
		t.Fatal(err)
	}
	if err := receipt.Rollback(); err == nil {
		t.Fatal("Rollback() accepted an enabled preimage without a route")
	}
}

func TestClaudeAdapterReportsExecutableAndSecretFailures(t *testing.T) {
	loop := filepath.Join(t.TempDir(), "loop")
	if err := os.Symlink(loop, loop); err != nil {
		t.Skipf("symbolic link unavailable: %v", err)
	}
	cfg := configuration.NewConfig()
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: loop}
	status := (claudeAdapter{}).Inspect(context.Background(), Dependencies{}, cfg, configuration.Runtime{}, InspectionOptions{})
	if status.Ready || status.Issue != "Cannot inspect Claude executable" {
		t.Fatalf("status = %#v", status)
	}

	runtime := configuration.Runtime{AccountID: "gateway"}
	if _, err := (claudeAdapter{}).Verify(context.Background(), Dependencies{}, cfg, runtime); err == nil || !strings.Contains(err.Error(), "secret store is unavailable") {
		t.Fatalf("Verify() error = %v", err)
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
