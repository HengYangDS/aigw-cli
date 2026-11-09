package codex

import (
	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform/process"
	"aigw-cli/internal/secret"
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
	before := cfg.Clients[ID]

	if err := (codexAdapter{}).Converge(adapter.Dependencies{}, &cfg, discovery.Result{}); err != nil {
		t.Fatal(err)
	}
	if got := cfg.Clients[ID]; !reflect.DeepEqual(got, before) {
		t.Fatalf("Codex convergence changed explicit intent without a discovered target: got %#v want %#v", got, before)
	}
}

func TestCodexAdapterReportsReadinessStates(t *testing.T) {
	newFixture := func(t *testing.T, provider string) (domain.Config, domain.Runtime) {
		t.Helper()
		target := filepath.Join(t.TempDir(), "config.toml")
		if err := os.WriteFile(target, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		cfg := domain.NewConfig(testAdmission())
		cfg.Providers["gateway"] = domain.Provider{Endpoints: domain.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
		cfg.Models["codex"] = qualifiedModel("", "gateway", "gpt-test", domain.ProtocolOpenAIResponses)
		cfg.Clients[ID] = domain.ClientBinding{Model: "codex", Enabled: true, ModelProvider: provider, Executable: "/usr/bin/codex", Targets: []string{target}}
		runtime, err := cfg.ResolveRuntime(ID, "")
		if err != nil {
			t.Fatal(err)
		}
		runtime.CredentialCommand = filepath.Join(t.TempDir(), "aigw")
		if _, err := ReconcileConfigs(nil, []TargetRef{HomeTarget(target)}, runtime, runtime, false); err != nil {
			t.Fatal(err)
		}
		return cfg, runtime
	}

	t.Run("missing executable", func(t *testing.T) {
		cfg, runtime := newFixture(t, modelProviderAIGW)
		clientAdapter := cfg.Clients[ID]
		clientAdapter.Executable = ""
		cfg.Clients[ID] = clientAdapter
		status := (codexAdapter{}).Inspect(context.Background(), adapter.Dependencies{}, cfg, runtime)
		if status.Ready || !strings.Contains(status.Issue, "executable") {
			t.Fatalf("status = %#v", status)
		}
	})

	t.Run("missing target", func(t *testing.T) {
		cfg, runtime := newFixture(t, modelProviderAIGW)
		clientAdapter := cfg.Clients[ID]
		clientAdapter.Targets = nil
		cfg.Clients[ID] = clientAdapter
		status := (codexAdapter{}).Inspect(context.Background(), adapter.Dependencies{}, cfg, runtime)
		if status.Ready || !strings.Contains(status.Issue, "target") {
			t.Fatalf("status = %#v", status)
		}
	})

	t.Run("projection drift", func(t *testing.T) {
		cfg, runtime := newFixture(t, modelProviderAIGW)
		clientAdapter := cfg.Clients[ID]
		clientAdapter.Targets = []string{filepath.Join(t.TempDir(), "missing.toml")}
		cfg.Clients[ID] = clientAdapter
		status := (codexAdapter{}).Inspect(context.Background(), adapter.Dependencies{}, cfg, runtime)
		if status.Ready || !strings.Contains(status.Issue, "projection drift") {
			t.Fatalf("status = %#v", status)
		}
		if strings.Contains(status.Issue, clientAdapter.Targets[0]) || strings.Contains(status.Checks[0].Detail, clientAdapter.Targets[0]) {
			t.Fatalf("Codex inspection exposed a private target path: %#v", status)
		}
	})

	// A configured file that was deleted with its folder, while the credential
	// reader of the real flow is configured, is found missing before the
	// reader is read through it. Outside Codex's default home, sync does not
	// create it, so the step creates its folder or moves Codex to another file.
	t.Run("deleted file with the credential reader", func(t *testing.T) {
		cfg, runtime := newFixture(t, modelProviderAIGW)
		if err := os.RemoveAll(filepath.Dir(cfg.Clients[ID].Targets[0])); err != nil {
			t.Fatal(err)
		}
		source := filepath.Join(t.TempDir(), "aigw")
		if err := os.WriteFile(source, []byte("AIGW fixture"), 0o700); err != nil {
			t.Fatal(err)
		}
		reader, err := secret.VersionedEntrypointPath(filepath.Join(t.TempDir(), "data"), source, "aigw")
		if err != nil {
			t.Fatal(err)
		}
		root := t.TempDir()
		deps := adapter.Dependencies{AIGWExecutable: reader, Discovery: fixedDiscoverer{}, Host: discovery.Host{Home: root}}
		status := (codexAdapter{}).Inspect(context.Background(), deps, cfg, runtime)
		// The folder went with the file, and AIGW creates neither, so the step
		// creates both. Outside this test's home it names the file by its own
		// name, no private path.
		if status.Ready || len(status.Checks) != 1 || status.Checks[0].Detail != "Codex configuration target is unavailable" ||
			status.RepairAction != "Create config.toml and the folder that holds it, or choose another path, then run the same command again." {
			t.Fatalf("status = %#v", status)
		}
	})

	t.Run("external provider", func(t *testing.T) {
		cfg, runtime := newFixture(t, "amazon-bedrock")
		status := (codexAdapter{}).Inspect(context.Background(), adapter.Dependencies{}, cfg, runtime)
		if !status.Ready {
			t.Fatalf("status = %#v", status)
		}
	})

	t.Run("capture unavailable", func(t *testing.T) {
		cfg, runtime := newFixture(t, modelProviderAIGW)
		status := (codexAdapter{}).Inspect(context.Background(), adapter.Dependencies{}, cfg, runtime)
		if !status.Ready {
			t.Fatalf("status = %#v", status)
		}
	})

	t.Run("inspection does not invoke the client", func(t *testing.T) {
		cfg, runtime := newFixture(t, modelProviderAIGW)
		runner := &captureAdapterRunner{err: errors.New("status failed")}
		status := (codexAdapter{}).Inspect(context.Background(), adapter.Dependencies{Runner: runner}, cfg, runtime)
		if !status.Ready || runner.calls != 0 {
			t.Fatalf("status = %#v, calls = %d", status, runner.calls)
		}
	})
}

func TestCodexAdapterApplyRequiresDiscovery(t *testing.T) {
	after := domain.NewConfig(testAdmission())
	after.SetClientActivation(ID, true, "", nil)
	if _, err := (codexAdapter{}).Apply(context.Background(), adapter.Dependencies{}, domain.NewConfig(testAdmission()), after); err == nil || !strings.Contains(err.Error(), "surface discovery is unavailable") {
		t.Fatalf("Apply() error = %v", err)
	}
}

func TestCodexAdapterProjectsCredentialHelperOnlyForProviderTokenProviders(t *testing.T) {
	for _, test := range []struct {
		name           string
		provider       string
		authentication domain.Authentication
		wantCommand    bool
	}{
		{name: "aigw account token", provider: modelProviderAIGW, authentication: domain.AuthenticationProviderToken, wantCommand: true},
		{name: "provider account token", provider: "amazon-bedrock", authentication: domain.AuthenticationProviderToken, wantCommand: true},
		{name: "client native", provider: "amazon-bedrock", authentication: domain.AuthenticationClientNative},
	} {
		t.Run(test.name, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "config.toml")
			cfg := domain.NewConfig(testAdmission())
			cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
			cfg.Models["codex"] = qualifiedModel("Codex", "gateway", "gpt-test", domain.ProtocolOpenAIResponses)
			cfg.Clients[ID] = domain.ClientBinding{
				Model: "codex", Enabled: true, ModelProvider: test.provider,
				Authentication: test.authentication, Executable: "/usr/bin/codex", Targets: []string{target},
			}
			discovered := discovery.Result{
				Executables: map[string]string{ID: "/usr/bin/codex"},
				Surfaces: []discovery.Surface{{
					ID:          "codex-home-default",
					ConfigPath:  target,
					AutoManaged: true,
				}},
			}
			command := filepath.Join(t.TempDir(), "aigw")
			_, err := (codexAdapter{}).Apply(context.Background(), adapter.Dependencies{
				Discovery:      fixedDiscoverer{result: discovered},
				AIGWExecutable: command,
			}, domain.NewConfig(testAdmission()), cfg)
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
	delete(disabled.Clients, ID)
	adapterRegistry := codexRegistry()
	if changed := adapterRegistry.ChangedClients(before, disabled); len(changed) != 1 || changed[0] != ID {
		t.Fatalf("adapter removal scope = %v, want Codex only", changed)
	}

	purpose := before.Clone()
	route := purpose.Models["codex"]
	route.Purpose = "display only"
	purpose.Models["codex"] = route
	if len(adapterRegistry.ChangedClients(before, purpose)) != 0 {
		t.Fatal("display-only purpose must not change the projection")
	}
}

type fixedDiscoverer struct{ result discovery.Result }

func (discoverer fixedDiscoverer) Discover(...string) discovery.Result { return discoverer.result }

func qualifiedModel(label, account, model string, protocols ...domain.EndpointProtocol) domain.Model {
	interfaces := make(map[domain.EndpointProtocol][]domain.Capability, len(protocols))
	for _, protocol := range protocols {
		interfaces[protocol] = []domain.Capability{}
	}
	return domain.Model{
		Label: label, Provider: account, Family: model, UpstreamModel: model, Protocols: interfaces,
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

func codexConfiguration(target string) domain.Config {
	cfg := domain.NewConfig(testAdmission())
	cfg.Providers["gateway"] = domain.Provider{Endpoints: domain.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	cfg.Models["codex"] = qualifiedModel("", "gateway", "gpt-test", domain.ProtocolOpenAIResponses)
	cfg.Clients[ID] = domain.ClientBinding{Model: "codex", Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
	return cfg
}

func codexVerificationFixture(t *testing.T) (domain.Config, domain.Runtime) {
	t.Helper()
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := codexConfiguration(target)
	configured := cfg.Clients[ID]
	configured.Executable = filepath.Join(t.TempDir(), "codex")
	cfg.Clients[ID] = configured
	if err := os.WriteFile(cfg.Clients[ID].Executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	runtime, err := cfg.ResolveRuntime(ID, "")
	if err != nil {
		t.Fatal(err)
	}
	runtime.CredentialCommand = filepath.Join(t.TempDir(), "aigw")
	if _, err := ReconcileConfigs(nil, []TargetRef{HomeTarget(target)}, runtime, runtime, false); err != nil {
		t.Fatal(err)
	}
	return cfg, runtime
}

// A configured target whose folder is a regular file names that file to move,
// on the checks that inspect it, and never a sync that would refuse with
// "Resolve not a directory".
func TestACodexAdapterInspectsAFileInTheWay(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "work", "config.toml")
	if err := os.WriteFile(filepath.Join(root, "work"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := codexConfiguration(target)
	configured := cfg.Clients[ID]
	configured.ModelProvider = modelProviderAIGW
	cfg.Clients[ID] = configured
	runtime, err := cfg.ResolveRuntime(ID, "")
	if err != nil {
		t.Fatal(err)
	}
	runtime.CredentialCommand = filepath.Join(t.TempDir(), "aigw")
	deps := adapter.Dependencies{Discovery: fixedDiscoverer{}, Host: discovery.Host{Home: root}}
	status := (codexAdapter{}).Inspect(context.Background(), deps, cfg, runtime)
	want := "Move the file ~/work, or choose another path, then run the same command again."
	if status.Ready || status.RepairAction != want {
		t.Fatalf("a file in the way = %#v", status)
	}
}
