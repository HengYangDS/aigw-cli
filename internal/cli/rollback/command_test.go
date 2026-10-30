package rollback

import (
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/client/claude"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/store"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRollbackCoversLoadCheckpointAndBackupFailures(t *testing.T) {
	t.Run("current config load", func(t *testing.T) {
		command := NewCommand(invocation.Context{Clients: clients.Registry(), Config: store.NewStore(t.TempDir(), clients.Admission())})
		command.SilenceErrors = true
		command.SilenceUsage = true
		if err := command.Execute(); err == nil {
			t.Fatal("current configuration load failure was accepted")
		}
	})

	t.Run("invalid verified checkpoint", func(t *testing.T) {
		configStore := rollbackStore(t)
		if err := os.WriteFile(configStore.Path()+".verified.json", []byte("not-json"), 0o600); err != nil {
			t.Fatal(err)
		}
		command := NewCommand(invocation.Context{Clients: clients.Registry(), Config: configStore})
		command.SilenceErrors = true
		command.SilenceUsage = true
		if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "parse verified checkpoint") {
			t.Fatalf("rollback error = %v", err)
		}
	})

	t.Run("invalid previous backup", func(t *testing.T) {
		configStore := rollbackStore(t)
		if err := os.WriteFile(configStore.Path()+".bak", []byte("not = [toml"), 0o600); err != nil {
			t.Fatal(err)
		}
		command := NewCommand(invocation.Context{Clients: clients.Registry(), Config: configStore})
		command.SilenceErrors = true
		command.SilenceUsage = true
		command.SetArgs([]string{"--last-change"})
		if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "parse config") {
			t.Fatalf("rollback error = %v", err)
		}
	})
}

func rollbackStore(t *testing.T) store.Store {
	t.Helper()
	configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["one"] = domain.Account{Label: "One", Endpoints: domain.Endpoints{Anthropic: "https://one.test"}}
	cfg.Routes["one"] = domain.Route{
		Label: "One", Account: "one", Model: "claude-test",
		Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}},
	}
	cfg.SetSelectedRoute(claude.ID, "one", "")
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	return configStore
}
