package test

import (
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
	"bytes"
	"io"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) Do(request *http.Request) (*http.Response, error) { return fn(request) }

type observingSecretStore struct {
	value          string
	getErr         error
	existsErr      error
	getCalls       int
	existsCalls    int
	getAccounts    []string
	existsAccounts []string
}

func (store *observingSecretStore) Get(account string) (string, error) {
	store.getCalls++
	store.getAccounts = append(store.getAccounts, account)
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
	store.existsAccounts = append(store.existsAccounts, account)
	return store.value != "", store.existsErr
}

func configuredReadinessRuntime(t *testing.T) (invocation.Context, domain.Config, *bytes.Buffer) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "configuration.toml")
	configStore := store.NewStore(configPath, clients.Admission())
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["one"] = domain.Account{
		Label: "One",
		Endpoints: domain.Endpoints{
			Anthropic:       "https://claude.example.test",
			OpenAIResponses: "https://codex.example.test/v1",
		},
	}
	cfg.Routes["claude"] = domain.Route{
		Label: "Claude", Account: "one", Model: "claude-test",
		Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}},
	}
	cfg.Routes["codex"] = domain.Route{
		Label: "Codex", Account: "one", Model: "gpt-test",
		Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}},
	}
	cfg.SetSelectedRoute(claude.ID, "claude", "")
	cfg.SetSelectedRoute(codex.ID, "codex", "")
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	runtime := invocation.Context{Clients: clients.Registry(),
		Config: configStore, Secrets: secret.NewMemoryStore(),
		Out: out, RenderOut: out, Width: 120,
	}
	var err error
	runtime.Accounts, err = secret.NewDiagnosticCredentialStore(runtime.Secrets)
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
