package setup

import (
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"

	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
)

type scriptedSecretStore struct {
	values      map[string]string
	getErr      error
	existsErr   error
	onGet       func(*scriptedSecretStore, string)
	onSet       func(*scriptedSecretStore, string)
	getCalls    int
	existsCalls int
}

// hostAt observes home as the user's home directory, so Claude Code's settings
// file is home/.claude/settings.json.
func hostAt(home string) discovery.Host {
	return discovery.Host{GOOS: runtime.GOOS, Home: home, Env: map[string]string{"HOME": home, "USERPROFILE": home}}
}

func (store *scriptedSecretStore) Get(name string) (string, error) {
	store.getCalls++
	if store.onGet != nil {
		store.onGet(store, name)
	}
	if store.getErr != nil {
		return "", store.getErr
	}
	if value, ok := store.values[name]; ok {
		return value, nil
	}
	return "", secret.ErrNotFound
}

func (store *scriptedSecretStore) Set(name, value string) error {
	if store.values == nil {
		store.values = map[string]string{}
	}
	store.values[name] = value
	if store.onSet != nil {
		store.onSet(store, name)
	}
	return nil
}

func (store *scriptedSecretStore) Delete(name string) error {
	delete(store.values, name)
	return nil
}

func (store *scriptedSecretStore) Exists(name string) (bool, error) {
	store.existsCalls++
	if store.existsErr != nil {
		return false, store.existsErr
	}
	_, ok := store.values[name]
	return ok, nil
}

func secretExists(t testing.TB, store secret.Store, account string) bool {
	t.Helper()
	present, err := store.Exists(account)
	if err != nil {
		t.Fatalf("observe credential for %q: %v", account, err)
	}
	return present
}

type setupHTTPClient func(*http.Request) (*http.Response, error)

func (do setupHTTPClient) Do(request *http.Request) (*http.Response, error) { return do(request) }

type errorReader struct{ err error }

func (reader errorReader) Read([]byte) (int, error) { return 0, reader.err }

type setupResponseBody struct {
	io.Reader
	closeErr error
}

func (body setupResponseBody) Close() error { return body.closeErr }

type setupDiscovery struct {
	result     discovery.Result
	onDiscover func()
}

func (candidate setupDiscovery) Discover(...string) discovery.Result {
	if candidate.onDiscover != nil {
		candidate.onDiscover()
	}
	return candidate.result
}

func manifestSetupConfig() domain.Config {
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["team"] = domain.Provider{Label: "Team", Endpoints: domain.Endpoints{Responses: "https://team.test/v1", Anthropic: "https://team.test"}}
	cfg.Models["claude"] = domain.Model{Label: "Claude", Provider: "team", Family: "claude-test", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.Models["codex"] = domain.Model{Label: "Codex", Provider: "team", Family: "gpt-test", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolResponses: {}}}
	cfg.SetSelectedModel(codex.ID, "codex", "")
	cfg.SetSelectedModel(claude.ID, "claude", "")
	cfg.Recommendations[codex.ID] = domain.ClientRecommendation{Primary: domain.ClientSelection{Model: "codex"}}
	cfg.Recommendations[claude.ID] = domain.ClientRecommendation{Primary: domain.ClientSelection{Model: "claude"}}
	cfg.SetClientActivation(codex.ID, true, "", nil)
	cfg.SetClientActivation(claude.ID, true, "", nil)
	return cfg
}

func TestManifestSetupClientSelectionRequiresConnectedModelAndUsableSurface(t *testing.T) {
	cfg := manifestSetupConfig()
	connected := []string{"team"}

	withoutCodexSurface := manifestSetupSelectedClients(cfg, connected, map[string]bool{
		claude.ID: true,
	})
	if len(withoutCodexSurface) != 1 || withoutCodexSurface[0] != claude.ID {
		t.Fatalf("clients without Codex surface = %#v", withoutCodexSurface)
	}

	if clients := manifestSetupSelectedClients(cfg, nil, map[string]bool{
		claude.ID: true,
	}); len(clients) != 0 {
		t.Fatalf("unconnected route selected clients = %#v", clients)
	}
}

func TestManifestSetupReportsCredentialRollbackDriftAfterConfigurationFailure(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "configuration.toml")
	manifestPath := filepath.Join(t.TempDir(), "team.toml")
	manifest := `version = 7
[recommendations.codex.primary]
route = "gpt"

[accounts.team]
label = "Team"
[accounts.team.endpoints]
openai_responses = "https://team.test/v1"

[models.gpt-test]
label = "GPT Test"

[routes.gpt]
label = "GPT"
account = "team"
model = "gpt-test"
upstream_model = "gpt-test"
interfaces = { openai_responses = ["text"] }
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	secretStore := &scriptedSecretStore{
		values: map[string]string{},
		onGet: func(secretStore *scriptedSecretStore, account string) {
			delete(secretStore.values, account)
		},
		onSet: func(_ *scriptedSecretStore, _ string) {
			if err := os.Mkdir(configPath, 0o700); err != nil {
				t.Fatal(err)
			}
		},
	}
	runtime := invocation.Context{Clients: clients.Registry(),
		Executable: filepath.Join(t.TempDir(), "aigw"),
		Config:     store.NewStore(configPath, clients.Admission()),
		Secrets:    secretStore,
		Discovery: setupDiscovery{result: discovery.Result{
			Executables: map[string]string{codex.ID: "/opt/codex"},
			Surfaces: []discovery.Surface{{
				ID:          string(codex.HomeDefault),
				ConfigPath:  filepath.Join(t.TempDir(), "codex", "configuration.toml"),
				Present:     true,
				AutoManaged: true,
			}},
		}},
		HTTP: setupHTTPClient(func(request *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}")), Request: request}, nil
		}),
		In:  strings.NewReader("token\n"),
		Out: io.Discard,
	}

	cmd := NewCommand(runtime)
	cmd.SetContext(context.Background())
	cmd.SilenceUsage, cmd.SilenceErrors = true, true
	cmd.SetArgs([]string{"--from", manifestPath, "--provider", "team", "--token-stdin"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "setup failed") || !strings.Contains(err.Error(), "credential rollback also failed") || !strings.Contains(err.Error(), "credential postimage changed") {
		t.Fatalf("error = %v, want configuration failure with rollback drift", err)
	}
	if secretExists(t, secretStore, "team") {
		t.Fatal("rollback recreated a credential removed before compensation")
	}
}
