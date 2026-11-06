package account

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
)

func TestProviderDiagnosticsRequireAProviderBeforeProviderWork(t *testing.T) {
	runtime := emptyProviderRuntime(t)
	accountCommand := NewCommand(runtime)
	accountCommand.SetArgs([]string{"diagnostics", "enable"})
	accountCommand.SilenceErrors = true
	accountCommand.SilenceUsage = true
	if err := accountCommand.Execute(); err == nil || !strings.Contains(err.Error(), "0 accounts") {
		t.Fatalf("account diagnostics enable error = %v", err)
	}
}

// emptyProviderRuntime is an interactive invocation without configuration.
func emptyProviderRuntime(t *testing.T) invocation.Context {
	t.Helper()
	return invocation.Context{
		Clients:     clients.Registry(),
		Config:      store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission()),
		Interactive: true,
		Out:         &bytes.Buffer{},
	}
}

func TestEditReturnsTransactionPreparationFailure(t *testing.T) {
	store, secretStore := blockedConfigurationStore(t)
	command := newEditCommand(invocation.Context{Clients: clients.Registry(), Config: store, Secrets: secretStore, Out: &bytes.Buffer{}})
	command.SetArgs([]string{"current", "--label", "Renamed"})
	command.SilenceErrors = true
	command.SilenceUsage = true

	if err := command.Execute(); err == nil {
		t.Fatal("transaction preparation failure was accepted")
	}
}

func blockedConfigurationStore(t *testing.T) (store.Store, secret.Store) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "configuration.toml")
	configStore := store.NewStore(path, clients.Admission())
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["current"] = domain.Provider{
		Label:     "Current",
		Endpoints: domain.Endpoints{OpenAIResponses: "https://current.test/v1"},
	}
	cfg.Models["current"] = domain.Model{
		Label: "Current", Provider: "current", Family: "gpt-current",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}},
	}
	cfg.SetSelectedModel(codex.ID, "current", "")
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	backup := path + ".bak"
	if err := os.Mkdir(backup, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backup, "blocker"), []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}
	return configStore, secret.NewMemoryStore()
}
