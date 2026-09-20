package client_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aigw-cli/internal/client"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"
)

type fixedDiscovery struct{ result discovery.Result }

func (source fixedDiscovery) Discover() discovery.Result { return source.result }

func configuredClient(t *testing.T, id string) (configuration.Config, client.Dependencies, discovery.Result) {
	t.Helper()
	root := t.TempDir()
	executable := filepath.Join(root, id)
	if err := os.WriteFile(executable, []byte("public client fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := configuration.NewConfig()
	cfg.Accounts["gateway"] = configuration.Account{Endpoints: configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1", Anthropic: "https://gateway.test"}}
	spec, _ := configuration.ClientSpecFor(id)
	cfg.Profiles[id] = configuration.Profile{Client: id, Account: "gateway", Model: "fixture", Protocol: spec.EndpointProtocols[0]}
	cfg.Routes[id] = id
	target := filepath.Join(root, "config.toml")
	adapter := configuration.ClientBinding{Enabled: true, Executable: executable}
	if id != configuration.ClientClaude {
		adapter.Targets = []string{target}
	}
	cfg.Clients[id] = adapter
	deps := client.Dependencies{AIGWExecutable: filepath.Join(root, "aigw"), ClaudeSettingsPath: filepath.Join(root, "settings.json")}
	discovered := discovery.Result{
		Executables: map[string]string{id: executable},
		Surfaces:    []discovery.Surface{{ID: "codex-home-default", Authority: "aigw", ConfigPath: target, AutoManaged: true}, {ID: "hermes-home-default", Authority: "aigw", ConfigPath: target}},
	}
	deps.Discovery = fixedDiscovery{result: discovered}
	return cfg, deps, discovered
}

func TestCredentialPolicyControlsProjectionAndSurvivesDiscovery(t *testing.T) {
	registry := client.DefaultRegistry()
	for _, id := range registry.IDs() {
		t.Run(id, func(t *testing.T) {
			before, deps, discovered := configuredClient(t, id)
			after := before.Clone()
			adapter := after.Clients[id]
			adapter.CredentialCommand = filepath.Join(t.TempDir(), "credential adapter")
			after.Clients[id] = adapter
			converged, err := registry.Converge(deps, after, discovered, id)
			if err != nil || converged.Clients[id].CredentialCommand != adapter.CredentialCommand {
				t.Fatalf("credential policy lost during discovery: %v", err)
			}
			if got := registry.ChangedClients(before, converged); !reflect.DeepEqual(got, []string{id}) {
				t.Fatalf("command-only change selected %v", got)
			}
			if err := registry.Apply(context.Background(), deps, configuration.NewConfig(), before, id); err != nil {
				t.Fatal(err)
			}
			plans, err := registry.Plan(deps, before, converged, id)
			if err != nil || len(plans) != 1 {
				t.Fatalf("projection plan = %v, %v", plans, err)
			}
			if err := registry.Apply(context.Background(), deps, before, converged, id); err != nil {
				t.Fatal(err)
			}
			runtime, err := converged.ResolveRuntime(id, "")
			if err != nil {
				t.Fatal(err)
			}
			if status := registry.Inspect(context.Background(), deps, converged, id, runtime); !status.Ready {
				t.Fatalf("projected credential command is not ready: %#v", status)
			}
		})
	}
}

func TestDisabledCredentialPolicyRequiresExplicitEnable(t *testing.T) {
	registry := client.DefaultRegistry()
	for _, id := range registry.IDs() {
		cfg, deps, discovered := configuredClient(t, id)
		command := filepath.Join(t.TempDir(), "credential adapter")
		cfg.Clients[id] = configuration.ClientBinding{CredentialCommand: command}
		after, err := registry.Converge(deps, cfg, discovered, id)
		if err != nil {
			t.Fatal(err)
		}
		if adapter := after.Clients[id]; adapter.Enabled || adapter.CredentialCommand != command {
			t.Fatalf("disabled policy changed: %#v", adapter)
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
	registry := client.DefaultRegistry()
	for _, id := range registry.IDs() {
		t.Run(id, func(t *testing.T) {
			cfg, deps, observed := configuredClient(t, id)
			cfg.Clients[id] = configuration.ClientBinding{Enabled: false}
			deps.Secrets = secrets.NewMemoryStore()
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

func TestMissingCodexTargetRetainsOnlyCredentialPolicy(t *testing.T) {
	cfg, deps, _ := configuredClient(t, configuration.ClientCodex)
	command := filepath.Join(t.TempDir(), "credential adapter")
	adapter := cfg.Clients[configuration.ClientCodex]
	adapter.Targets = nil
	adapter.CredentialCommand = command
	cfg.Clients[configuration.ClientCodex] = adapter
	after, err := client.DefaultRegistry().Converge(deps, cfg, discovery.Result{}, configuration.ClientCodex)
	if err != nil {
		t.Fatal(err)
	}
	want := configuration.ClientBinding{CredentialCommand: command}
	if got := after.Clients[configuration.ClientCodex]; !reflect.DeepEqual(got, want) {
		t.Fatalf("missing-target policy = %#v", got)
	}
}

type rejectingClient struct{ calls int }

func (runner *rejectingClient) RunCapture(_ context.Context, plan process.Plan) ([]byte, error) {
	runner.calls++
	if reflect.DeepEqual(plan.Args, []string{"--version"}) {
		return []byte("codex-cli 0.1.0"), nil
	}
	return []byte("public-secret-canary"), errors.New("public-secret-canary")
}

func TestExternalCredentialFailuresKeepUnknownSecretsOutOfDiagnostics(t *testing.T) {
	registry := client.DefaultRegistry()
	for _, id := range registry.IDs() {
		t.Run(id, func(t *testing.T) {
			cfg, deps, _ := configuredClient(t, id)
			adapter := cfg.Clients[id]
			adapter.CredentialCommand = filepath.Join(t.TempDir(), "credential adapter")
			cfg.Clients[id] = adapter
			if err := registry.Apply(context.Background(), deps, configuration.NewConfig(), cfg, id); err != nil {
				t.Fatal(err)
			}
			runtime, err := cfg.ResolveRuntime(id, "")
			if err != nil {
				t.Fatal(err)
			}
			runner := &rejectingClient{}
			deps.Runner = runner
			_, err = registry.Verify(context.Background(), deps, cfg, id, runtime, "")
			if err == nil || strings.Contains(err.Error(), "public-secret-canary") || !strings.Contains(err.Error(), "diagnostics suppressed") {
				t.Fatalf("external client failure = %v", err)
			}
			wantCalls := 1
			if id != configuration.ClientClaude {
				wantCalls++
			}
			if runner.calls != wantCalls {
				t.Fatalf("client calls = %d, want %d without retry", runner.calls, wantCalls)
			}
		})
	}
}
