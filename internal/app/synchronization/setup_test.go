package synchronization

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	builtinclients "aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform/process"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
)

type setupCredentials struct {
	secret.Store
	writes int
}

func (s *setupCredentials) Set(account, token string) error {
	s.writes++
	return s.Store.Set(account, token)
}

func setupTokenStore(t *testing.T, token string) *setupCredentials {
	t.Helper()
	store := secret.NewMemoryStore()
	if err := store.Set("team", token); err != nil {
		t.Fatal(err)
	}
	return &setupCredentials{Store: store}
}

type setupDiscovery func()

func (discover setupDiscovery) Discover() discovery.Result {
	if discover != nil {
		discover()
	}
	return discovery.Result{}
}

func setupConfiguration() domain.Config {
	cfg := domain.NewConfig(builtinclients.Admission())
	cfg.Accounts["team"] = domain.Account{Label: "Team", Endpoints: domain.Endpoints{Anthropic: "https://team.test"}}
	cfg.Routes["claude"] = domain.Route{Label: "Claude", Account: "team", Model: "claude-test", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.SetSelectedRoute(claude.ID, "claude", "")
	return cfg
}

func TestSetupPreservesAlreadyConfiguredState(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(filepath.Join(dir, "config.toml"))
	configStore := store.NewStore(filepath.Join(dir, "aigw.toml"), builtinclients.Admission())
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	preimage, err := os.ReadFile(configStore.Path())
	if err != nil {
		t.Fatal(err)
	}
	memory := secret.NewMemoryStore()
	if err := memory.Set("gateway", "old-token"); err != nil {
		t.Fatal(err)
	}
	credentials := &setupCredentials{Store: memory}
	syncer := Synchronizer{Registry: builtinClients, Config: configStore, Secrets: credentials,
		Discovery: setupDiscovery(func() { t.Fatal("setup discovered clients in an already configured installation") }),
		Runner: runnerFunc(func(context.Context, process.Plan) error {
			t.Fatal("setup changed existing native client credentials")
			return nil
		}),
	}
	_, err = syncer.Setup(t.Context(), cfg, cfg.Clone(), map[string]string{"gateway": "replacement-token"}, codex.ID)
	if err == nil || !strings.Contains(err.Error(), "already configured") {
		t.Fatalf("setup error = %v; want rejection of an already configured installation", err)
	}
	storedToken, tokenErr := credentials.Get("gateway")
	if tokenErr != nil || storedToken != "old-token" || credentials.writes != 0 {
		t.Fatalf("setup touched the prior Account Token: writes=%d, error=%v", credentials.writes, tokenErr)
	}
	after, readErr := os.ReadFile(configStore.Path())
	if readErr != nil || string(after) != string(preimage) {
		t.Fatalf("setup changed the prior configuration: %v", readErr)
	}
}

func TestCancelledSetupPreservesCredentialsAndConfiguration(t *testing.T) {
	for _, phase := range []string{"before_setup", "during_discovery"} {
		t.Run(phase, func(t *testing.T) {
			configStore := store.NewStore(filepath.Join(t.TempDir(), "aigw.toml"), builtinclients.Admission())
			credentials := setupTokenStore(t, "old-token")
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			discoverySource := setupDiscovery(nil)
			if phase == "before_setup" {
				cancel()
			} else {
				discoverySource = setupDiscovery(cancel)
			}
			runtime := Synchronizer{Registry: builtinClients, Config: configStore, Secrets: credentials, Discovery: discoverySource}
			_, err := runtime.Setup(ctx, domain.NewConfig(builtinclients.Admission()), setupConfiguration(), map[string]string{"team": "new-token"}, claude.ID)
			if !errors.Is(err, context.Canceled) {
				t.Errorf("setup error = %v, want cancellation", err)
			}
			if token, getErr := credentials.Get("team"); getErr != nil || token != "old-token" {
				t.Errorf("credential = %q, %v; want original Token", token, getErr)
			}
			if _, statErr := os.Stat(configStore.Path()); !errors.Is(statErr, os.ErrNotExist) {
				t.Errorf("cancelled setup created configuration: %v", statErr)
			}
			if phase == "before_setup" && credentials.writes != 0 {
				t.Errorf("cancelled setup attempted %d credential writes", credentials.writes)
			}
		})
	}
}

func TestSetupRejectsInvalidConfigurationBeforeCredentialMutation(t *testing.T) {
	configStore := store.NewStore(filepath.Join(t.TempDir(), "aigw.toml"), builtinclients.Admission())
	credentials := setupTokenStore(t, "original")
	after := setupConfiguration()
	after.SetSelectedRoute(claude.ID, "missing-route", "")
	_, err := (Synchronizer{Registry: builtinClients, Config: configStore, Secrets: credentials}).Setup(t.Context(), domain.NewConfig(builtinclients.Admission()), after, map[string]string{"team": "replacement"})
	if err == nil {
		t.Fatal("setup accepted an unresolved route")
	}
	if credentials.writes != 0 {
		t.Fatalf("invalid setup touched credentials %d times before rejection", credentials.writes)
	}
	if token, err := credentials.Get("team"); err != nil || token != "original" {
		t.Fatalf("invalid setup changed the original credential: %v", err)
	}
}

func TestSetupAdmitsCredentialOwnershipAndClientScopeBeforeWriting(t *testing.T) {
	for _, boundary := range []string{"account", "client"} {
		t.Run(boundary, func(t *testing.T) {
			configStore := store.NewStore(filepath.Join(t.TempDir(), "aigw.toml"), builtinclients.Admission())
			credentials := setupTokenStore(t, "original")
			tokens := map[string]string{"team": "replacement"}
			var clients []string
			if boundary == "account" {
				tokens = map[string]string{"unknown": "replacement"}
			} else {
				clients = []string{"unknown"}
			}
			syncer := Synchronizer{Registry: builtinClients, Config: configStore, Secrets: credentials, Discovery: setupDiscovery(nil)}
			_, err := syncer.Setup(t.Context(), domain.NewConfig(builtinclients.Admission()), setupConfiguration(), tokens, clients...)
			if err == nil || credentials.writes != 0 {
				t.Fatalf("invalid %s: error = %v, credential writes = %d", boundary, err, credentials.writes)
			}
		})
	}
}

func TestSetupCommitReportsBackendSelectionFailureWithoutErasingForeignState(t *testing.T) {
	for _, phase := range []string{"before preparation", "during commit"} {
		t.Run(phase, func(t *testing.T) {
			root := t.TempDir()
			secretRoot := filepath.Join(root, "secrets")
			if err := os.Mkdir(secretRoot, 0o700); err != nil {
				t.Fatal(err)
			}
			backendPath := filepath.Join(secretRoot, "backend")
			replaceBackend := func() {
				if err := os.WriteFile(backendPath, []byte("foreign-selection\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			secretStore, err := secret.Select(secret.Selection{
				GOOS: runtime.GOOS, Root: secretRoot,
				KeyringProbe: func(secret.Store) error { return errors.New("isolated file backend") },
			})
			if err != nil {
				t.Fatal(err)
			}
			configPath := filepath.Join(root, "configuration.toml")
			if err := os.Mkdir(configPath, 0o700); err != nil {
				t.Fatal(err)
			}
			run := Synchronizer{Registry: builtinClients,
				Config: store.NewStore(configPath, builtinclients.Admission()), Secrets: secretStore,
				Discovery: setupDiscovery(replaceBackend),
			}
			want := "backend selection rollback also failed"
			if phase == "before preparation" {
				replaceBackend()
				want = "observe automatic credential backend selection"
			}
			_, err = run.Setup(t.Context(), domain.NewConfig(builtinclients.Admission()), setupConfiguration(), map[string]string{"team": "token"}, claude.ID)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("commit error = %v, want %s", err, want)
			}
			if phase == "during commit" {
				var configErr *os.PathError
				present, observeErr := secretStore.Exists("team")
				if !errors.As(err, &configErr) || observeErr != nil || present {
					t.Fatalf("commit lost configuration failure or retained its credential: %v", err)
				}
			}
			if selected, err := os.ReadFile(backendPath); err != nil || string(selected) != "foreign-selection\n" {
				t.Fatalf("foreign backend state was not preserved: %q, %v", selected, err)
			}
		})
	}
}
