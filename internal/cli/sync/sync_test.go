package sync

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"aigw-cli/internal/app"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/cli/repair"
	"aigw-cli/internal/client/claude"
	"aigw-cli/internal/client/codex"
	"aigw-cli/internal/client/discovery"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
)

type syncDiscovery struct{ result discovery.Result }

func (candidate syncDiscovery) Discover() discovery.Result { return candidate.result }

func TestDryRunPreservesUnusedChangedCredentialEntrypointWithoutWriting(t *testing.T) {
	for _, command := range []string{"sync", "repair"} {
		t.Run(command, func(t *testing.T) {
			store, _ := configuredRepairStore(t)
			root := t.TempDir()
			source := filepath.Join(root, "aigw")
			helper := filepath.Join(root, "data", "credential", "aigw")
			if err := os.WriteFile(source, []byte("AIGW fixture"), 0o700); err != nil {
				t.Fatal(err)
			}
			if _, err := secret.EnsureEntrypoint(source, helper); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(helper+".sha256", []byte("changed receipt\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(store.Path())
			if err != nil {
				t.Fatal(err)
			}
			runtime := invocation.Context{Clients: clients.Registry(), Executable: source, CredentialPath: helper, Config: store, Discovery: staticDiscovery{}, Out: &bytes.Buffer{}}
			if command == "sync" {
				cmd := NewCommand(runtime)
				cmd.SilenceErrors = true
				cmd.SilenceUsage = true
				cmd.SetArgs([]string{"--dry-run", "--json"})
				err = cmd.Execute()
			} else {
				cmd := repair.NewCommand(runtime)
				cmd.SilenceErrors, cmd.SilenceUsage = true, true
				cmd.SetArgs([]string{"--dry-run", "--json"})
				err = cmd.Execute()
			}
			if err != nil {
				t.Fatalf("%s dry-run inspected an unused cached entrypoint: %v", command, err)
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

func TestDryRunUsesOneResolvedReaderWithoutWriting(t *testing.T) {
	for _, operation := range []string{"sync", "repair"} {
		t.Run(operation, func(t *testing.T) {
			store, cfg := configuredRepairStore(t)
			root := t.TempDir()
			source, reader := filepath.Join(root, "source"), filepath.Join(root, "data", "credential", "reader")
			cfg.SetClientActivation(claude.ID, true, source, nil)
			if err := store.Save(cfg); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(store.Path())
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			out := &bytes.Buffer{}
			tokens := secret.NewMemoryStore()
			if err := tokens.Set("one", "synthetic-fixture"); err != nil {
				t.Fatal(err)
			}
			runtime := invocation.Context{Clients: clients.Registry(), Config: store, Executable: source, Secrets: tokens, Out: out, Discovery: staticDiscovery{}, Host: hostAt(root), ResolveCredentialPath: sync.OnceValues(func() (string, error) {
				calls++
				return reader, nil
			})}
			if operation == "sync" {
				cmd := NewCommand(runtime)
				cmd.SilenceErrors, cmd.SilenceUsage = true, true
				cmd.SetArgs([]string{"--dry-run", "--json"})
				err = cmd.Execute()
			} else {
				cmd := repair.NewCommand(runtime)
				cmd.SilenceErrors, cmd.SilenceUsage = true, true
				cmd.SetArgs([]string{"--dry-run", "--json"})
				err = cmd.Execute()
			}
			if err != nil {
				t.Fatal(err)
			}
			var result struct {
				Reader *app.CredentialEntrypointPlan `json:"credential_entrypoint"`
			}
			if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.Reader == nil || result.Reader.Path != reader || result.Reader.Action != "install" || calls != 1 {
				t.Fatalf("preview reader = %+v; resolutions=%d error=%v", result.Reader, calls, err)
			}
			after, err := os.ReadFile(store.Path())
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("preview changed configuration: %v", err)
			}
			if _, err := os.Lstat(reader); !os.IsNotExist(err) {
				t.Fatalf("preview wrote reader: %v", err)
			}
		})
	}
}

func TestSyncMetadataObservationScopeExpiresWithCommand(t *testing.T) {
	store, cfg := configuredRepairStore(t)
	root := t.TempDir()
	source := filepath.Join(root, "source")
	reader := filepath.Join(root, "data", "credential", "reader")
	cfg.SetClientActivation(claude.ID, true, source, nil)
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	lookups := 0
	tokens := secret.NewEnvironmentStore(func(key string) string {
		if key == secret.EnvironmentKey("one") {
			lookups++
			return "synthetic-fixture"
		}
		return ""
	})
	runtime := invocation.Context{Clients: clients.Registry(),
		Config: store, Executable: source, Secrets: tokens, Out: &bytes.Buffer{}, Discovery: staticDiscovery{},
		Host: hostAt(root), CredentialPath: reader,
	}
	cmd := NewCommand(runtime)
	cmd.SilenceErrors, cmd.SilenceUsage = true, true
	cmd.SetArgs([]string{"--dry-run", "--json"})
	for invocation := 1; invocation <= 2; invocation++ {
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if lookups != invocation {
			t.Fatalf("command %d observed Account metadata %d times; want one fresh observation per command", invocation, lookups)
		}
		if _, err := os.Stat(reader); !os.IsNotExist(err) {
			t.Fatalf("dry-run wrote a credential reader: %v", err)
		}
	}
}

func TestSyncPropagatesPlanningAndReconciliationFailures(t *testing.T) {
	t.Run("configuration load", func(t *testing.T) {
		command := NewCommand(invocation.Context{Clients: clients.Registry(), Config: store.NewStore(t.TempDir(), clients.Admission())})
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
			configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
			cfg := domain.NewConfig(clients.Admission())
			cfg.Accounts["one"] = domain.Account{Label: "One", Endpoints: domain.Endpoints{OpenAIResponses: "https://one.test/v1"}}
			cfg.Routes["one"] = domain.Route{
				Label: "One", Account: "one", Model: "gpt-test",
				Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}},
			}
			cfg.SetSelectedRoute(codex.ID, "one", "")
			cfg.SetClientActivation(codex.ID, true, "/portable/codex", []string{"/portable/config.toml"})
			if err := configStore.Save(cfg); err != nil {
				t.Fatal(err)
			}
			out := &bytes.Buffer{}
			errOut := &bytes.Buffer{}
			command := NewCommand(invocation.Context{Clients: clients.Registry(), Config: configStore, Out: out, RenderOut: out})
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
		configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
		cfg := domain.NewConfig(clients.Admission())
		cfg.Accounts["one"] = domain.Account{Label: "One", Endpoints: domain.Endpoints{OpenAIResponses: "https://one.test/v1"}}
		cfg.Routes["one"] = domain.Route{
			Label: "One", Account: "one", Model: "gpt-test",
			Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}},
		}
		cfg.SetSelectedRoute(codex.ID, "one", "")
		cfg.SetClientActivation(codex.ID, true, "/opt/codex", []string{""})
		if err := configStore.Save(cfg); err != nil {
			t.Fatal(err)
		}
		command := NewCommand(invocation.Context{Clients: clients.Registry(), Config: configStore, Discovery: syncDiscovery{}, Out: &bytes.Buffer{}})
		command.SilenceErrors = true
		command.SilenceUsage = true
		command.SetArgs([]string{"--dry-run"})
		if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "target is empty") {
			t.Fatalf("planning error = %v", err)
		}
	})
}

func TestSyncRollsBackRouteSelectionWhenProjectionFails(t *testing.T) {
	configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["one"] = domain.Account{Label: "One", Endpoints: domain.Endpoints{OpenAIResponses: "https://one.test/v1"}}
	cfg.Accounts["two"] = domain.Account{Label: "Two", Endpoints: domain.Endpoints{OpenAIResponses: "https://two.test/v1"}}
	cfg.Routes["one"] = domain.Route{
		Label: "One", Account: "one", Model: "gpt-test",
		Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}},
	}
	cfg.Routes["two"] = domain.Route{
		Label: "Two", Account: "two", Model: "gpt-test",
		Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}},
	}
	cfg.SetSelectedRoute(codex.ID, "one", "")
	cfg.SetClientActivation(codex.ID, true, "/opt/codex", []string{t.TempDir()})
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	secretStore := secret.NewMemoryStore()
	if err := secretStore.Set("two", "token"); err != nil {
		t.Fatal(err)
	}
	command := NewCommand(invocation.Context{Clients: clients.Registry(), Config: configStore, Secrets: secretStore, Discovery: syncDiscovery{}, Out: &bytes.Buffer{}})
	command.SilenceErrors = true
	command.SilenceUsage = true

	if err := command.Execute(); err == nil {
		t.Fatal("projection failure after Client Binding selection was accepted")
	}
	after, err := configStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := after.SelectedRoute(codex.ID); got != "one" {
		t.Fatalf("failed sync left Client Binding %q, want rolled-back binding one", got)
	}
}

func TestSyncReportsFailureWhenRepairingAnExistingProjection(t *testing.T) {
	configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	target := t.TempDir()
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["one"] = domain.Account{Label: "One", Endpoints: domain.Endpoints{OpenAIResponses: "https://one.test/v1"}}
	cfg.Routes["one"] = domain.Route{
		Label: "One", Account: "one", Model: "gpt-test",
		Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}},
	}
	cfg.SetSelectedRoute(codex.ID, "one", "")
	cfg.SetClientActivation(codex.ID, true, "/opt/codex", []string{target})
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	command := NewCommand(invocation.Context{Clients: clients.Registry(), Config: configStore, Discovery: syncDiscovery{}, Out: &bytes.Buffer{}})
	command.SilenceErrors = true
	command.SilenceUsage = true
	if err := command.Execute(); err == nil {
		t.Fatal("existing projection repair failure was accepted")
	}
}
