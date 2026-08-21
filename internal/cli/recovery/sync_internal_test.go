package recovery

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
)

type syncDiscovery struct{ result discovery.Result }

func (candidate syncDiscovery) Discover() discovery.Result { return candidate.result }

func TestSyncPropagatesPlanningAndReconciliationFailures(t *testing.T) {
	t.Run("configuration load", func(t *testing.T) {
		command := NewSyncCommand(invocation.Context{Config: configuration.NewStore(t.TempDir())})
		command.SilenceErrors = true
		command.SilenceUsage = true
		if err := command.Execute(); err == nil {
			t.Fatal("configuration load failure was accepted")
		}
	})

	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "planning", args: []string{"--dry-run"}},
		{name: "reconciliation"},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
			cfg := configuration.NewConfig()
			cfg.Accounts["one"] = configuration.Account{Label: "One", Endpoints: configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}}
			cfg.Profiles["one"] = configuration.Profile{Label: "One", Account: "one", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-test"}}
			cfg.Routes.Default = "one"
			cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/portable/codex", Targets: []string{"/portable/config.toml"}}
			if err := store.Save(cfg); err != nil {
				t.Fatal(err)
			}
			out := &bytes.Buffer{}
			errOut := &bytes.Buffer{}
			command := NewSyncCommand(invocation.Context{Config: store, Out: out, RenderOut: out})
			command.SilenceErrors = true
			command.SilenceUsage = true
			command.SetOut(out)
			command.SetErr(errOut)
			command.SetArgs(test.args)

			err := command.Execute()
			if err == nil || !strings.Contains(err.Error(), "discovery") {
				t.Fatalf("sync error = %v, want discovery failure", err)
			}
			if strings.Contains(out.String(), "completed") {
				t.Fatalf("sync reported completion after failure: %q", out.String())
			}
			combined := strings.ToLower(out.String() + errOut.String())
			for _, residue := range []string{"usage:", "warning", "traceback"} {
				if strings.Contains(combined, residue) {
					t.Fatalf("sync emitted %q after a handled failure: %q", residue, combined)
				}
			}
		})
	}
}

func TestSyncReportsProjectionPlanningAndApplyFailures(t *testing.T) {
	t.Run("planning", func(t *testing.T) {
		store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
		cfg := configuration.NewConfig()
		cfg.Accounts["one"] = configuration.Account{Label: "One", Endpoints: configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}}
		cfg.Profiles["one"] = configuration.Profile{Label: "One", Account: "one", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-test"}}
		cfg.Routes.Default = "one"
		cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{""}}
		if err := store.Save(cfg); err != nil {
			t.Fatal(err)
		}
		command := NewSyncCommand(invocation.Context{Config: store, Discovery: syncDiscovery{}, Out: &bytes.Buffer{}})
		command.SilenceErrors = true
		command.SilenceUsage = true
		command.SetArgs([]string{"--dry-run"})
		if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "target is empty") {
			t.Fatalf("planning error = %v", err)
		}
	})

	t.Run("apply", func(t *testing.T) {
		store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
		target := t.TempDir()
		cfg := configuration.NewConfig()
		cfg.Accounts["one"] = configuration.Account{Label: "One", Endpoints: configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}}
		cfg.Profiles["one"] = configuration.Profile{Label: "One", Account: "one", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-test"}}
		cfg.Routes.Default = "one"
		cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
		if err := store.Save(cfg); err != nil {
			t.Fatal(err)
		}
		command := NewSyncCommand(invocation.Context{Config: store, Discovery: syncDiscovery{}, Out: &bytes.Buffer{}})
		command.SilenceErrors = true
		command.SilenceUsage = true
		if err := command.Execute(); err == nil {
			t.Fatal("projection apply failure was accepted")
		}
	})
}

func TestSyncReportsFailureWhenRepairingAnExistingProjection(t *testing.T) {
	store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
	target := t.TempDir()
	cfg := configuration.NewConfig()
	cfg.Accounts["one"] = configuration.Account{Label: "One", Endpoints: configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}}
	cfg.Profiles["one"] = configuration.Profile{Label: "One", Account: "one", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-test"}}
	cfg.Routes.Default = "one"
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	command := NewSyncCommand(invocation.Context{Config: store, Discovery: syncDiscovery{}, Out: &bytes.Buffer{}})
	command.SilenceErrors = true
	command.SilenceUsage = true
	if err := command.Execute(); err == nil {
		t.Fatal("existing projection repair failure was accepted")
	}
}

func TestRollbackCoversLoadCheckpointAndBackupFailures(t *testing.T) {
	t.Run("current config load", func(t *testing.T) {
		command := NewRollbackCommand(invocation.Context{Config: configuration.NewStore(t.TempDir())})
		command.SilenceErrors = true
		command.SilenceUsage = true
		if err := command.Execute(); err == nil {
			t.Fatal("current configuration load failure was accepted")
		}
	})

	t.Run("invalid verified checkpoint", func(t *testing.T) {
		store := rollbackStore(t)
		if err := os.WriteFile(store.Path()+".verified.json", []byte("not-json"), 0o600); err != nil {
			t.Fatal(err)
		}
		command := NewRollbackCommand(invocation.Context{Config: store})
		command.SilenceErrors = true
		command.SilenceUsage = true
		if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "parse verified checkpoint") {
			t.Fatalf("rollback error = %v", err)
		}
	})

	t.Run("invalid previous backup", func(t *testing.T) {
		store := rollbackStore(t)
		if err := os.WriteFile(store.Path()+".bak", []byte("not = [toml"), 0o600); err != nil {
			t.Fatal(err)
		}
		command := NewRollbackCommand(invocation.Context{Config: store})
		command.SilenceErrors = true
		command.SilenceUsage = true
		command.SetArgs([]string{"--last-change"})
		if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "parse config") {
			t.Fatalf("rollback error = %v", err)
		}
	})
}

func rollbackStore(t *testing.T) configuration.Store {
	t.Helper()
	store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
	cfg := configuration.NewConfig()
	cfg.Accounts["one"] = configuration.Account{Label: "One", Endpoints: configuration.Endpoints{Anthropic: "https://one.test"}}
	cfg.Profiles["one"] = configuration.Profile{Label: "One", Account: "one", Client: configuration.ClientClaude, Models: configuration.Models{configuration.ClientClaude: "claude-test"}}
	cfg.Routes.Default = "one"
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	return store
}
