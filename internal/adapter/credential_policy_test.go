package adapter_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/claudedesktop"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform/process"
	"aigw-cli/internal/secret"
)

type fixedDiscovery struct{ result discovery.Result }

func (source fixedDiscovery) Discover() discovery.Result { return source.result }

func configuredClient(t *testing.T, id string) (domain.Config, adapter.Dependencies, discovery.Result) {
	t.Helper()
	root := t.TempDir()
	executable := filepath.Join(root, id)
	if err := os.WriteFile(executable, []byte("public client fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["gateway"] = domain.Account{Endpoints: domain.Endpoints{OpenAIResponses: "https://gateway.test/v1", Anthropic: "https://gateway.test"}}
	spec, _ := clients.Admission().Spec(id)
	cfg.Routes[id] = domain.Route{
		Account: "gateway", Model: "fixture", UpstreamModel: "fixture",
		Interfaces: map[domain.EndpointProtocol][]domain.Capability{spec.EndpointProtocols[0]: {}},
	}
	target := filepath.Join(root, "config.toml")
	if id == claudedesktop.ID {
		target = filepath.Join(root, "Claude-3p", "configLibrary")
	}
	clientAdapter := domain.ClientBinding{Route: id, Enabled: true, Protocol: spec.EndpointProtocols[0], Executable: executable}
	if id != claude.ID {
		clientAdapter.Targets = []string{target}
	}
	cfg.Clients[id] = clientAdapter
	deps := adapter.Dependencies{AIGWExecutable: filepath.Join(root, "aigw"), Host: discovery.Host{GOOS: runtime.GOOS, Home: root, Env: map[string]string{"HOME": root, "USERPROFILE": root}}}
	discovered := discovery.Result{
		Executables: map[string]string{id: executable},
		Surfaces:    []discovery.Surface{{ID: "codex-home-default", Authority: "aigw", ConfigPath: target, AutoManaged: true}, {ID: "hermes-home-default", Authority: "aigw", ConfigPath: target}},
	}
	deps.Discovery = fixedDiscovery{result: discovered}
	return cfg, deps, discovered
}

func TestCredentialPolicyControlsProjectionAndSurvivesDiscovery(t *testing.T) {
	registry := clients.Registry()
	for _, id := range registry.IDs() {
		t.Run(id, func(t *testing.T) {
			before, deps, discovered := configuredClient(t, id)
			after := before.Clone()
			clientAdapter := after.Clients[id]
			clientAdapter.CredentialCommand = filepath.Join(t.TempDir(), "credential adapter")
			after.Clients[id] = clientAdapter
			converged, err := registry.Converge(deps, after, discovered, id)
			if err != nil || converged.Clients[id].CredentialCommand != clientAdapter.CredentialCommand {
				t.Fatalf("credential policy lost during discovery: %v", err)
			}
			if got := registry.ChangedClients(before, converged); !reflect.DeepEqual(got, []string{id}) {
				t.Fatalf("command-only change selected %v", got)
			}
			if _, _, err := registry.Apply(context.Background(), deps, domain.NewConfig(clients.Admission()), before, id); err != nil {
				t.Fatal(err)
			}
			plans, err := registry.Plan(deps, before, converged, id)
			if err != nil || len(plans) != 1 {
				t.Fatalf("projection plan = %v, %v", plans, err)
			}
			wantSurface := map[string]string{
				claude.ID:        "claude-settings",
				claudedesktop.ID: "claude-desktop-config-library",
				codex.ID:         "codex-home-default",
				hermes.ID:        "hermes-home-default",
			}[id]
			if plans[0].Surface != wantSurface {
				t.Fatalf("projection surface = %q, want %q", plans[0].Surface, wantSurface)
			}
			if _, _, err := registry.Apply(context.Background(), deps, before, converged, id); err != nil {
				t.Fatal(err)
			}
			clientRuntime, err := converged.ResolveRuntime(id, "")
			if err != nil {
				t.Fatal(err)
			}
			if status := registry.Inspect(context.Background(), deps, converged, id, clientRuntime); !status.Ready {
				t.Fatalf("projected credential command is not ready: %#v", status)
			}
		})
	}
}

func TestDisabledCredentialPolicyRequiresExplicitEnable(t *testing.T) {
	registry := clients.Registry()
	for _, id := range registry.IDs() {
		cfg, deps, discovered := configuredClient(t, id)
		command := filepath.Join(t.TempDir(), "credential adapter")
		cfg.Clients[id] = domain.ClientBinding{CredentialCommand: command}
		after, err := registry.Converge(deps, cfg, discovered, id)
		if err != nil {
			t.Fatal(err)
		}
		if clientAdapter := after.Clients[id]; clientAdapter.Enabled || clientAdapter.CredentialCommand != command {
			t.Fatalf("disabled policy changed: %#v", clientAdapter)
		}
		if err := registry.Withdraw(&after, id); err != nil {
			t.Fatal(err)
		}
		if _, exists := after.Clients[id]; exists {
			t.Fatal("full withdrawal retained host credential policy")
		}
	}
}

func TestExplicitDisableSurvivesDiscoveryWithoutAnExternalHelper(t *testing.T) {
	registry := clients.Registry()
	for _, id := range registry.IDs() {
		t.Run(id, func(t *testing.T) {
			cfg, deps, observed := configuredClient(t, id)
			cfg.SetClientActivation(id, false, "", nil)
			deps.Secrets = secret.NewMemoryStore()
			if err := deps.Secrets.Set("gateway", "public-token"); err != nil {
				t.Fatal(err)
			}
			after, err := registry.Converge(deps, cfg, observed, id)
			if err != nil {
				t.Fatal(err)
			}
			if after.Clients[id].Enabled {
				t.Fatal("discovery re-enabled explicit disabled intent")
			}
		})
	}
}

func TestMissingCodexTargetRetainsExplicitIntentAndCredentialPolicy(t *testing.T) {
	cfg, deps, _ := configuredClient(t, codex.ID)
	command := filepath.Join(t.TempDir(), "credential adapter")
	clientAdapter := cfg.Clients[codex.ID]
	clientAdapter.Targets = nil
	clientAdapter.CredentialCommand = command
	cfg.Clients[codex.ID] = clientAdapter
	after, err := clients.Registry().Converge(deps, cfg, discovery.Result{}, codex.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := cfg.Clients[codex.ID]
	if got := after.Clients[codex.ID]; !reflect.DeepEqual(got, want) {
		t.Fatalf("missing-target policy = %#v", got)
	}
}

type rejectingClient struct{ calls int }

func (runner *rejectingClient) RunCaptureStreams(ctx context.Context, plan process.Plan) ([]byte, []byte, error) {
	output, err := runner.RunCapture(ctx, plan)
	if err != nil {
		return nil, output, err
	}
	return output, nil, nil
}

func (runner *rejectingClient) RunCapture(_ context.Context, plan process.Plan) ([]byte, error) {
	runner.calls++
	if reflect.DeepEqual(plan.Args, []string{"--version"}) {
		return []byte("codex-cli 0.1.0"), nil
	}
	return []byte("public-secret-canary"), errors.New("public-secret-canary")
}

func TestExternalCredentialFailuresKeepUnknownSecretsOutOfDiagnostics(t *testing.T) {
	registry := clients.Registry()
	for _, id := range registry.IDs() {
		t.Run(id, func(t *testing.T) {
			cfg, deps, _ := configuredClient(t, id)
			clientAdapter := cfg.Clients[id]
			clientAdapter.CredentialCommand = filepath.Join(t.TempDir(), "credential adapter")
			cfg.Clients[id] = clientAdapter
			if _, _, err := registry.Apply(context.Background(), deps, domain.NewConfig(clients.Admission()), cfg, id); err != nil {
				t.Fatal(err)
			}
			clientRuntime, err := cfg.ResolveRuntime(id, "")
			if err != nil {
				t.Fatal(err)
			}
			runner := &rejectingClient{}
			deps.Runner = runner
			_, err = registry.Verify(context.Background(), deps, cfg, id, clientRuntime, "")
			if id == claudedesktop.ID {
				if err == nil || !strings.Contains(err.Error(), "explicit native Chat, Cowork, or Code acceptance") || runner.calls != 0 {
					t.Fatalf("Claude Desktop verification = %v, calls = %d", err, runner.calls)
				}
				return
			}
			if id == hermes.ID && runtime.GOOS == "windows" {
				if err == nil || strings.Contains(err.Error(), "public-secret-canary") || !strings.Contains(err.Error(), "cannot isolate") || runner.calls != 0 {
					t.Fatalf("Windows Hermes verification = %v, calls = %d", err, runner.calls)
				}
				return
			}
			if err == nil || strings.Contains(err.Error(), "public-secret-canary") || !strings.Contains(err.Error(), "diagnostics suppressed") {
				t.Fatalf("external client failure = %v", err)
			}
			wantCalls := 1
			if id != claude.ID {
				wantCalls++
			}
			if runner.calls != wantCalls {
				t.Fatalf("client calls = %d, want %d without retry", runner.calls, wantCalls)
			}
		})
	}
}
