package status

import (
	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/claudedesktop"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
	"bytes"
	"context"
	"io"
	"path/filepath"
	"slices"
	"testing"

	"github.com/spf13/cobra"
)

type observingSecretStore struct {
	value           string
	getErr          error
	existsErr       error
	getCalls        int
	existsCalls     int
	getProviders    []string
	existsProviders []string
}

func (store *observingSecretStore) Get(account string) (string, error) {
	store.getCalls++
	store.getProviders = append(store.getProviders, account)
	if store.getErr != nil {
		return "", store.getErr
	}
	if store.value == "" {
		return "", secret.ErrNotFound
	}
	return store.value, nil
}

func (*observingSecretStore) Set(string, string) error { return nil }

func (*observingSecretStore) Delete(string) error { return nil }

func (store *observingSecretStore) Exists(account string) (bool, error) {
	store.existsCalls++
	store.existsProviders = append(store.existsProviders, account)
	return store.value != "", store.existsErr
}

func configuredReadinessRuntime(t *testing.T) (invocation.Context, domain.Config, *bytes.Buffer) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "configuration.toml")
	configStore := store.NewStore(configPath, clients.Admission())
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["one"] = domain.Provider{
		Label: "One",
		Endpoints: domain.Endpoints{
			Anthropic: "https://claude.example.test",
			Responses: "https://codex.example.test/v1",
		},
	}
	cfg.Models["claude"] = domain.Model{
		Label: "Claude", Provider: "one", Family: "claude-test",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}},
	}
	cfg.Models["codex"] = domain.Model{
		Label: "Codex", Provider: "one", Family: "gpt-test",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolResponses: {}},
	}
	cfg.SetSelectedModel(claude.ID, "claude", "")
	cfg.SetSelectedModel(codex.ID, "codex", "")
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	runtime := invocation.Context{Clients: clients.Registry(),
		Config: configStore, Secrets: secret.NewMemoryStore(),
		Out: out, Width: 120,
	}
	var err error
	runtime.Providers, err = secret.NewDiagnosticCredentialStore(runtime.Secrets)
	if err != nil {
		t.Fatal(err)
	}
	return runtime, cfg, out
}

func executeCommand(command *cobra.Command) error {
	command.SetErr(io.Discard)
	command.SilenceErrors = true
	command.SilenceUsage = true
	return command.Execute()
}

// inspectedAdapter is a built-in client adapter whose projection inspection
// reports ready as the test decides, without reading native files.
type inspectedAdapter struct {
	adapter.Adapter
	ready bool
}

func (clientAdapter inspectedAdapter) Inspect(context.Context, adapter.Dependencies, domain.Config, domain.Runtime) adapter.Status {
	return adapter.Status{Ready: clientAdapter.ready}
}

// inspectProjections gives runtime the built-in adapters, each inspecting its
// client's projection as ready reports.
func inspectProjections(t *testing.T, runtime *invocation.Context, ready func(clientID string) bool) {
	t.Helper()
	builtin := []adapter.Adapter{claude.New(), claudedesktop.New(), codex.New(), hermes.New()}
	inspected := make([]adapter.Adapter, 0, len(builtin))
	for _, clientAdapter := range builtin {
		inspected = append(inspected, inspectedAdapter{Adapter: clientAdapter, ready: ready(clientAdapter.Spec().ID)})
	}
	registry, err := adapter.NewRegistry(inspected...)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(registry.IDs(), runtime.Clients.IDs()) {
		t.Fatalf("inspected clients = %v, want the built-in clients %v", registry.IDs(), runtime.Clients.IDs())
	}
	runtime.Clients = registry
}

func everyProjectionReady(string) bool { return true }
