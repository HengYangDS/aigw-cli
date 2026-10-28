package codex

import (
	"aigw-cli/internal/client"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/process"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestCodexConvergencePreservesEnabledIntentWithoutDiscoveredTargets(t *testing.T) {
	cfg := codexConfiguration("/explicit/config.toml")
	before := cfg.Clients[configuration.ClientCodex]

	if err := (codexAdapter{}).Converge(client.Dependencies{}, &cfg, discovery.Result{}); err != nil {
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
		cfg.Routes["codex"] = qualifiedRoute("", "gateway", "gpt-test", configuration.ProtocolOpenAIResponses)
		cfg.Clients[configuration.ClientCodex] = configuration.ClientBinding{Route: "codex", Enabled: true, ModelProvider: provider, Executable: "/usr/bin/codex", Targets: []string{target}}
		runtime, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
		if err != nil {
			t.Fatal(err)
		}
		runtime.CredentialCommand = filepath.Join(t.TempDir(), "aigw")
		if err := SyncConfig(target, runtime); err != nil {
			t.Fatal(err)
		}
		return cfg, runtime
	}

	t.Run("missing executable", func(t *testing.T) {
		cfg, runtime := newFixture(t, configuration.ModelProviderAIGW)
		adapter := cfg.Clients[configuration.ClientCodex]
		adapter.Executable = ""
		cfg.Clients[configuration.ClientCodex] = adapter
		status := (codexAdapter{}).Inspect(context.Background(), client.Dependencies{}, cfg, runtime)
		if status.Ready || !strings.Contains(status.Issue, "executable") {
			t.Fatalf("status = %#v", status)
		}
	})

	t.Run("missing target", func(t *testing.T) {
		cfg, runtime := newFixture(t, configuration.ModelProviderAIGW)
		adapter := cfg.Clients[configuration.ClientCodex]
		adapter.Targets = nil
		cfg.Clients[configuration.ClientCodex] = adapter
		status := (codexAdapter{}).Inspect(context.Background(), client.Dependencies{}, cfg, runtime)
		if status.Ready || !strings.Contains(status.Issue, "target") {
			t.Fatalf("status = %#v", status)
		}
	})

	t.Run("projection drift", func(t *testing.T) {
		cfg, runtime := newFixture(t, configuration.ModelProviderAIGW)
		adapter := cfg.Clients[configuration.ClientCodex]
		adapter.Targets = []string{filepath.Join(t.TempDir(), "missing.toml")}
		cfg.Clients[configuration.ClientCodex] = adapter
		status := (codexAdapter{}).Inspect(context.Background(), client.Dependencies{}, cfg, runtime)
		if status.Ready || !strings.Contains(status.Issue, "projection drift") {
			t.Fatalf("status = %#v", status)
		}
		if strings.Contains(status.Issue, adapter.Targets[0]) || strings.Contains(status.Checks[0].Detail, adapter.Targets[0]) {
			t.Fatalf("Codex inspection exposed a private target path: %#v", status)
		}
	})

	t.Run("external provider", func(t *testing.T) {
		cfg, runtime := newFixture(t, "amazon-bedrock")
		status := (codexAdapter{}).Inspect(context.Background(), client.Dependencies{}, cfg, runtime)
		if !status.Ready {
			t.Fatalf("status = %#v", status)
		}
	})

	t.Run("capture unavailable", func(t *testing.T) {
		cfg, runtime := newFixture(t, configuration.ModelProviderAIGW)
		status := (codexAdapter{}).Inspect(context.Background(), client.Dependencies{}, cfg, runtime)
		if !status.Ready {
			t.Fatalf("status = %#v", status)
		}
	})

	t.Run("inspection does not invoke the client", func(t *testing.T) {
		cfg, runtime := newFixture(t, configuration.ModelProviderAIGW)
		runner := &captureAdapterRunner{err: errors.New("status failed")}
		status := (codexAdapter{}).Inspect(context.Background(), client.Dependencies{Runner: runner}, cfg, runtime)
		if !status.Ready || runner.calls != 0 {
			t.Fatalf("status = %#v, calls = %d", status, runner.calls)
		}
	})
}

func TestCodexAdapterApplyRequiresDiscovery(t *testing.T) {
	after := configuration.NewConfig()
	after.SetClientActivation(configuration.ClientCodex, true, "", nil)
	if _, err := (codexAdapter{}).Apply(context.Background(), client.Dependencies{}, configuration.NewConfig(), after); err == nil || !strings.Contains(err.Error(), "surface discovery is unavailable") {
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
			cfg.Routes["codex"] = qualifiedRoute("Codex", "gateway", "gpt-test", configuration.ProtocolOpenAIResponses)
			cfg.Clients[configuration.ClientCodex] = configuration.ClientBinding{
				Route: "codex", Enabled: true, ModelProvider: test.provider,
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
			_, err := (codexAdapter{}).Apply(context.Background(), client.Dependencies{
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

func TestChangedClientsFollowPersistentCodexSemantics(t *testing.T) {
	target := filepath.Join(t.TempDir(), "configuration.toml")
	before := codexConfiguration(target)

	disabled := before.Clone()
	delete(disabled.Clients, configuration.ClientCodex)
	adapterRegistry := codexRegistry()
	if changed := adapterRegistry.ChangedClients(before, disabled); len(changed) != 1 || changed[0] != configuration.ClientCodex {
		t.Fatalf("adapter removal scope = %v, want Codex only", changed)
	}

	purpose := before.Clone()
	route := purpose.Routes["codex"]
	route.Purpose = "display only"
	purpose.Routes["codex"] = route
	if len(adapterRegistry.ChangedClients(before, purpose)) != 0 {
		t.Fatal("display-only purpose must not change the projection")
	}
}

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

func codexConfiguration(target string) configuration.Config {
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Endpoints: configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	cfg.Routes["codex"] = qualifiedRoute("", "gateway", "gpt-test", configuration.ProtocolOpenAIResponses)
	cfg.Clients[configuration.ClientCodex] = configuration.ClientBinding{Route: "codex", Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
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
	if err := SyncConfig(target, runtime); err != nil {
		t.Fatal(err)
	}
	return cfg, runtime
}
