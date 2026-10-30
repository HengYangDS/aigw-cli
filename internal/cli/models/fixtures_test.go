package models

import (
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/app"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
	"bytes"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func catalogRuntime(t *testing.T, cfg domain.Config, secretValues map[string]string, client app.HTTPDoer) (invocation.Context, *bytes.Buffer) {
	t.Helper()
	configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	if len(cfg.Routes) > 0 {
		if err := configStore.Save(cfg); err != nil {
			t.Fatal(err)
		}
	}
	secretStore := secret.NewMemoryStore()
	for account, value := range secretValues {
		if err := secretStore.Set(account, value); err != nil {
			t.Fatal(err)
		}
	}
	out := new(bytes.Buffer)
	return invocation.Context{Config: configStore, Secrets: secretStore, HTTP: client, Out: out, Width: 120}, out
}

func executeCatalogCommand(t *testing.T, command *cobra.Command, args ...string) error {
	t.Helper()
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs(args)
	return command.Execute()
}

func configuredCatalog() domain.Config {
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["gateway"] = domain.Account{Label: "Gateway", Endpoints: domain.Endpoints{OpenAIResponses: "https://gateway.test/v1", Anthropic: "https://gateway.test"}}
	cfg.Routes["codex"] = domain.Route{
		Label: "Codex", Account: "gateway", Model: "gpt-codex",
		Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}},
	}
	cfg.Routes["claude"] = domain.Route{
		Label: "Claude", Account: "gateway", Model: "gpt-claude",
		Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}},
	}
	cfg.SetSelectedRoute(codex.ID, "codex", "")
	return cfg
}

type faultingSecrets struct {
	values      map[string]string
	failAccount string
	getErr      error
}

func (store *faultingSecrets) Get(account string) (string, error) {
	if store.getErr != nil && (store.failAccount == "" || store.failAccount == account) {
		return "", store.getErr
	}
	value, ok := store.values[account]
	if !ok {
		return "", secret.ErrNotFound
	}
	return value, nil
}

func (*faultingSecrets) Set(string, string) error { return nil }

func (*faultingSecrets) Delete(string) error { return nil }

func (store *faultingSecrets) Exists(account string) (bool, error) {
	if store.getErr != nil && (store.failAccount == "" || store.failAccount == account) {
		return false, store.getErr
	}
	_, ok := store.values[account]
	return ok, nil
}

func saveCatalogConfig(t *testing.T, cfg domain.Config) store.Store {
	t.Helper()
	configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	return configStore
}

type catalogHTTPClient func(*http.Request) (*http.Response, error)

func (client catalogHTTPClient) Do(request *http.Request) (*http.Response, error) {
	return client(request)
}
