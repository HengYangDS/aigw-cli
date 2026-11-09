package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/store"
)

// verifiedRecoveryError reports a configuration outcome as a failed commit does.
type verifiedRecoveryError struct{ restored bool }

func (verifiedRecoveryError) Error() string                        { return "client file write failed" }
func (recovery verifiedRecoveryError) ConfigurationRestored() bool { return recovery.restored }

func TestRollbackFailureReportsVerifiedConfigurationRecovery(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want string
	}{
		{name: "restored", err: fmt.Errorf("rollback %w", verifiedRecoveryError{restored: true}), want: "Configuration was restored"},
		{name: "incomplete", err: verifiedRecoveryError{}, want: "Configuration restoration was incomplete"},
		{name: "unverified", err: errors.New("rollback synchronization preflight failed"), want: "may have changed; inspect them"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := rollbackFailureImpact(test.err); !strings.Contains(got, test.want) {
				t.Fatalf("rollback failure impact = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRollbackCoversLoadSnapshotAndBackupFailures(t *testing.T) {
	run := func(configStore store.Store, args ...string) error {
		command := NewCommand(invocation.Context{Clients: clients.Registry(), Config: configStore})
		command.SetArgs(append([]string{"rollback"}, args...))
		return executeManifestCommand(command)
	}
	t.Run("current config load", func(t *testing.T) {
		if err := run(store.NewStore(t.TempDir(), clients.Admission())); err == nil {
			t.Fatal("current configuration load failure was accepted")
		}
	})
	t.Run("invalid verified snapshot", func(t *testing.T) {
		configStore := rollbackStore(t)
		if err := os.WriteFile(configStore.Path()+".verified.json", []byte("not-json"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := run(configStore); err == nil || !strings.Contains(errors.Unwrap(err).Error(), "parse verified checkpoint") {
			t.Fatalf("rollback error = %v", err)
		}
	})
	t.Run("invalid previous backup", func(t *testing.T) {
		configStore := rollbackStore(t)
		if err := os.WriteFile(configStore.Path()+".bak", []byte("not = [toml"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := run(configStore, "--last-change"); err == nil || !strings.Contains(errors.Unwrap(err).Error(), "parse config") {
			t.Fatalf("rollback error = %v", err)
		}
	})
}

func rollbackStore(t *testing.T) store.Store {
	t.Helper()
	configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["one"] = domain.Provider{Label: "One", Endpoints: domain.Endpoints{Anthropic: "https://one.test"}}
	cfg.Models["one"] = domain.Model{
		Label: "One", Provider: "one", Family: "claude-test",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}},
	}
	cfg.SetSelectedModel(claude.ID, "one", "")
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	return configStore
}
