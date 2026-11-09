package setup

import (
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"

	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
	"aigw-cli/internal/terminal"
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

type scriptedSetupPrompt struct {
	value string
	err   error
}

func (prompt scriptedSetupPrompt) Secret(string) (string, error) { return prompt.value, prompt.err }
func (prompt scriptedSetupPrompt) Text(string) (string, error)   { return "", prompt.err }
func (prompt scriptedSetupPrompt) Select(string, []terminal.Choice) (string, error) {
	return "", prompt.err
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
	cfg.Providers["team"] = domain.Provider{Label: "Team", Endpoints: domain.Endpoints{OpenAIResponses: "https://team.test/v1", Anthropic: "https://team.test"}}
	cfg.Models["claude"] = domain.Model{Label: "Claude", Provider: "team", Family: "claude-test", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.Models["codex"] = domain.Model{Label: "Codex", Provider: "team", Family: "gpt-test", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
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

func TestGuidedSetupReportsCredentialRollbackDriftAfterConfigurationFailure(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "configuration.toml")
	secretStore := &scriptedSecretStore{
		values: map[string]string{},
		onGet: func(secretStore *scriptedSecretStore, account string) {
			if secretStore.values[account] == "token" {
				delete(secretStore.values, account)
			}
		},
	}
	runtime := invocation.Context{Clients: clients.Registry(),
		Executable: filepath.Join(t.TempDir(), "aigw"),
		Config:     store.NewStore(configPath, clients.Admission()),
		Secrets:    secretStore,
		Discovery:  setupDiscovery{},
		HTTP: setupHTTPClient(func(request *http.Request) (*http.Response, error) {
			if err := os.Mkdir(configPath, 0o700); err != nil {
				t.Fatal(err)
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}")), Request: request}, nil
		}),
		In:  strings.NewReader("token\n"),
		Out: io.Discard,
	}
	request := setupRequest{Provider: "team", Model: "claude", Label: "Team", AnthropicURL: "https://team.test", Client: claude.ID, UpstreamModel: "claude-test", TokenStdin: true}

	err := runSetup(context.Background(), runtime, request)
	if err == nil || !strings.Contains(err.Error(), "setup failed") || !strings.Contains(err.Error(), "credential rollback also failed") || !strings.Contains(err.Error(), "credential postimage changed") {
		t.Fatalf("error = %v, want configuration failure with rollback drift", err)
	}
	if secretExists(t, secretStore, "team") {
		t.Fatal("rollback recreated a credential removed before compensation")
	}
}

func TestFailedSetupPreservesBackendForUncompensatedCredential(t *testing.T) {
	root := t.TempDir()
	secretRoot := filepath.Join(root, "secrets")
	secretStore, err := secret.Select(secret.Selection{
		GOOS: runtime.GOOS, Root: secretRoot,
		KeyringProbe: func(secret.Store) error { return errors.New("isolated file backend") },
	})
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "configuration.toml")
	run := invocation.Context{Clients: clients.Registry(),
		Executable: filepath.Join(root, "aigw"), Config: store.NewStore(configPath, clients.Admission()), Secrets: secretStore,
		Discovery: setupDiscovery{onDiscover: func() {
			if err := secretStore.Set("team", "newer-token"); err != nil {
				t.Fatal(err)
			}
		}},
		HTTP: setupHTTPClient(func(request *http.Request) (*http.Response, error) {
			if err := os.Mkdir(configPath, 0o700); err != nil {
				t.Fatal(err)
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}")), Request: request}, nil
		}),
		In: strings.NewReader("token\n"), Out: io.Discard,
	}
	err = runSetup(t.Context(), run, setupRequest{Provider: "team", Model: "claude", Label: "Team", AnthropicURL: "https://team.test", Client: claude.ID, UpstreamModel: "claude-test", TokenStdin: true})
	if err == nil || !strings.Contains(err.Error(), "credential postimage changed") {
		t.Fatalf("setup error = %v, want uncompensated credential", err)
	}
	if token, err := secretStore.Get("team"); err != nil || token != "newer-token" {
		t.Fatalf("newer credential was not preserved: %v", err)
	}
	selected, err := os.ReadFile(filepath.Join(secretRoot, "backend"))
	if err != nil || strings.TrimSpace(string(selected)) != "file" {
		t.Fatalf("backend selection = %q, %v; want file while credential remains", selected, err)
	}
	fresh, err := secret.Select(secret.Selection{
		GOOS: runtime.GOOS, Root: secretRoot,
		KeyringProbe: func(secret.Store) error {
			t.Fatal("fresh invocation must reuse the retained backend without probing keyring")
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if token, err := fresh.Get("team"); err != nil || token != "newer-token" {
		t.Fatalf("fresh invocation cannot read the retained credential: %v", err)
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

func TestSetupNamesExplicitProtocolChoice(t *testing.T) {
	_, err := planSetup(domain.NewConfig(clients.Admission()), setupRequest{
		Model: "shared", Client: hermes.ID, UpstreamModel: "shared-model",
		OpenAIURL: "https://responses.test/v1", AnthropicURL: "https://messages.test",
	})
	if err == nil || !strings.Contains(err.Error(), "--protocol") {
		t.Fatalf("ambiguous Hermes setup error = %v, want explicit protocol guidance", err)
	}
	var rendered bytes.Buffer
	terminal.RenderError(terminal.NewRenderer(&rendered, false), err, false)
	if want := "Rerun the same aigw setup command with --protocol <responses|anthropic>"; !strings.Contains(rendered.String(), want) ||
		strings.Contains(rendered.String(), "aigw check") {
		t.Fatalf("ambiguous Hermes setup recommendation = %q, want %q", rendered.String(), want)
	}
	_, err = planSetup(domain.NewConfig(clients.Admission()), setupRequest{
		Model: "chat", Client: hermes.ID, UpstreamModel: "chat-model",
		Protocol: domain.ProtocolOpenAIChatCompletions,
	})
	if err == nil || !strings.Contains(err.Error(), "--chat-url") {
		t.Fatalf("missing Chat endpoint error = %v, want exact URL guidance", err)
	}

	for _, test := range []struct {
		name     string
		request  setupRequest
		protocol domain.EndpointProtocol
	}{
		{
			name: "Anthropic among two endpoints",
			request: setupRequest{
				Model: "shared", Client: hermes.ID, UpstreamModel: "shared-model",
				OpenAIURL: "https://responses.test/v1", AnthropicURL: "https://messages.test",
				Protocol: domain.ProtocolAnthropic,
			},
			protocol: domain.ProtocolAnthropic,
		},
		{
			name: "Chat Completions",
			request: setupRequest{
				Model: "chat", Client: hermes.ID, UpstreamModel: "chat-model",
				ChatURL: "https://chat.test/v1", Protocol: domain.ProtocolOpenAIChatCompletions,
			},
			protocol: domain.ProtocolOpenAIChatCompletions,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan, err := planSetup(domain.NewConfig(clients.Admission()), test.request)
			if err != nil || plan.config.Clients[hermes.ID].Protocol != test.protocol {
				t.Fatalf("Hermes setup protocol = %#v, %v", plan.config.Clients[hermes.ID], err)
			}
		})
	}
	command := NewCommand(invocation.Context{Clients: clients.Registry()})
	if command.Flags().Lookup("protocol") == nil || command.Flags().Lookup("chat-url") == nil {
		t.Fatal("guided setup does not expose the declared protocol and Chat endpoint choices")
	}
}

func TestSetupTokenPromptAndBackendErrors(t *testing.T) {
	t.Run("backend error", func(t *testing.T) {
		want := errors.New("backend failed")
		app := invocation.Context{Clients: clients.Registry(), Secrets: &scriptedSecretStore{getErr: want}}
		if _, err := setupToken(app, setupRequest{Provider: "one"}); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("prompt", func(t *testing.T) {
		app := invocation.Context{Clients: clients.Registry(), Secrets: secret.NewMemoryStore(), Prompt: scriptedSetupPrompt{value: "prompt-token"}}
		credential, err := setupToken(app, setupRequest{Provider: "one", PromptToken: true, Label: "One"})
		if err != nil || !credential.write || credential.token != "prompt-token" {
			t.Fatalf("credential=%#v error=%v", credential, err)
		}
	})
}

func TestRunSetupConfiguresDiscoveredClaudeClient(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	configStore := store.NewStore(path, clients.Admission())
	secretStore := secret.NewMemoryStore()
	if err := secretStore.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	runtime := invocation.Context{Clients: clients.Registry(),
		Executable: filepath.Join(t.TempDir(), "aigw"),
		Config:     configStore,
		Secrets:    secretStore,
		Discovery:  setupDiscovery{result: discovery.Result{Executables: map[string]string{claude.ID: "/opt/claude"}}},
		Host:       hostAt(t.TempDir()),
		HTTP: setupHTTPClient(func(request *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Request: request}, nil
		}),
		Out:         io.Discard,
		Interactive: false,
	}
	request := setupRequest{Provider: "team", Model: "claude", Label: "Team", AnthropicURL: "https://team.test", Client: claude.ID, UpstreamModel: "claude-test"}
	if err := runSetup(context.Background(), runtime, request); err != nil {
		t.Fatal(err)
	}
	cfg, err := configStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	if adapter := cfg.Clients[claude.ID]; !adapter.Enabled || adapter.Executable != "/opt/claude" {
		t.Fatalf("Claude adapter = %#v", adapter)
	}
}
