package recovery

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/secrets"
)

type syncDiscovery struct{ result discovery.Result }

func (candidate syncDiscovery) Discover() discovery.Result { return candidate.result }

func TestDryRunRejectsChangedCredentialEntrypointWithoutWriting(t *testing.T) {
	for _, command := range []string{"sync", "repair"} {
		t.Run(command, func(t *testing.T) {
			store, _ := configuredRepairStore(t)
			root := t.TempDir()
			source := filepath.Join(root, "aigw")
			helper := filepath.Join(root, "data", "credential", "aigw")
			if err := os.WriteFile(source, []byte("AIGW fixture"), 0o700); err != nil {
				t.Fatal(err)
			}
			if _, err := credential.EnsureEntrypoint(source, helper); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(helper+".sha256", []byte("changed receipt\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(store.Path())
			if err != nil {
				t.Fatal(err)
			}
			runtime := invocation.Context{Executable: source, CredentialPath: helper, Config: store, Discovery: staticDiscovery{}, Out: &bytes.Buffer{}}
			if command == "sync" {
				cmd := NewSyncCommand(runtime)
				cmd.SilenceErrors = true
				cmd.SilenceUsage = true
				cmd.SetArgs([]string{"--dry-run", "--json"})
				err = cmd.Execute()
			} else {
				err = runRepair(t.Context(), runtime, true, true)
			}
			if err == nil || !strings.Contains(err.Error(), "differs from its recorded bytes") {
				t.Fatalf("%s dry-run admitted a changed entrypoint: %v", command, err)
			}
			after, readErr := os.ReadFile(store.Path())
			if readErr != nil || !bytes.Equal(after, before) {
				t.Fatalf("%s dry-run changed configuration: %v", command, readErr)
			}
			if _, statErr := os.Lstat(helper); statErr != nil {
				t.Fatalf("%s dry-run removed a changed helper: %v", command, statErr)
			}
		})
	}
}

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
			cfg.Routes["one"] = configuration.Route{
				Label: "One", Account: "one", Model: "gpt-test",
				Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}},
			}
			cfg.SetSelectedRoute(configuration.ClientCodex, "one")
			cfg.SetClientActivation(configuration.ClientCodex, true, "/portable/codex", []string{"/portable/config.toml"})
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
		cfg.Routes["one"] = configuration.Route{
			Label: "One", Account: "one", Model: "gpt-test",
			Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}},
		}
		cfg.SetSelectedRoute(configuration.ClientCodex, "one")
		cfg.SetClientActivation(configuration.ClientCodex, true, "/opt/codex", []string{""})
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
}

func TestSyncRollsBackRouteSelectionWhenProjectionFails(t *testing.T) {
	store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
	cfg := configuration.NewConfig()
	cfg.Accounts["one"] = configuration.Account{Label: "One", Endpoints: configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}}
	cfg.Accounts["two"] = configuration.Account{Label: "Two", Endpoints: configuration.Endpoints{OpenAIResponses: "https://two.test/v1"}}
	cfg.Routes["one"] = configuration.Route{
		Label: "One", Account: "one", Model: "gpt-test",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}},
	}
	cfg.Routes["two"] = configuration.Route{
		Label: "Two", Account: "two", Model: "gpt-test",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}},
	}
	cfg.SetSelectedRoute(configuration.ClientCodex, "one")
	cfg.SetClientActivation(configuration.ClientCodex, true, "/opt/codex", []string{t.TempDir()})
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	secretStore := secrets.NewMemoryStore()
	if err := secretStore.Set("two", "token"); err != nil {
		t.Fatal(err)
	}
	command := NewSyncCommand(invocation.Context{Config: store, Secrets: secretStore, Discovery: syncDiscovery{}, Out: &bytes.Buffer{}})
	command.SilenceErrors = true
	command.SilenceUsage = true

	if err := command.Execute(); err == nil {
		t.Fatal("projection failure after Client Binding selection was accepted")
	}
	after, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := after.SelectedRoute(configuration.ClientCodex); got != "one" {
		t.Fatalf("failed sync left Client Binding %q, want rolled-back binding one", got)
	}
}

func TestSyncReportsFailureWhenRepairingAnExistingProjection(t *testing.T) {
	store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
	target := t.TempDir()
	cfg := configuration.NewConfig()
	cfg.Accounts["one"] = configuration.Account{Label: "One", Endpoints: configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}}
	cfg.Routes["one"] = configuration.Route{
		Label: "One", Account: "one", Model: "gpt-test",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}},
	}
	cfg.SetSelectedRoute(configuration.ClientCodex, "one")
	cfg.SetClientActivation(configuration.ClientCodex, true, "/opt/codex", []string{target})
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
	cfg.Routes["one"] = configuration.Route{
		Label: "One", Account: "one", Model: "claude-test",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}},
	}
	cfg.SetSelectedRoute(configuration.ClientClaude, "one")
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	return store
}
