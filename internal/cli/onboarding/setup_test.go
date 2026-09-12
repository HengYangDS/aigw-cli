package onboarding

import (
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/prompt"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/secrets"
	surfaceidentity "aigw-cli/internal/surface"
)

type scriptedSecretStore struct {
	values      map[string]string
	getErr      error
	existsErr   error
	onGet       func(*scriptedSecretStore, string)
	getCalls    int
	existsCalls int
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
	return "", secrets.ErrNotFound
}

func (store *scriptedSecretStore) Set(name, value string) error {
	if store.values == nil {
		store.values = map[string]string{}
	}
	store.values[name] = value
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

func secretExists(t testing.TB, store secrets.Store, account string) bool {
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
func (prompt scriptedSetupPrompt) Select(string, []prompt.Choice) (string, error) {
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

func (candidate setupDiscovery) Discover() discovery.Result {
	if candidate.onDiscover != nil {
		candidate.onDiscover()
	}
	return candidate.result
}

func manifestSetupConfig() configuration.Config {
	cfg := configuration.NewConfig()
	cfg.Accounts["team"] = configuration.Account{Label: "Team", Endpoints: configuration.Endpoints{OpenAIResponses: "https://team.test/v1", Anthropic: "https://team.test"}}
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "team", Client: configuration.ClientClaude, Model: "claude-test"}
	cfg.Profiles["codex"] = configuration.Profile{Label: "Codex", Account: "team", Client: configuration.ClientCodex, Model: "gpt-test"}
	cfg.Routes[configuration.ClientCodex] = "codex"
	cfg.Routes[configuration.ClientClaude] = "claude"
	return cfg
}

func TestManifestSetupCredentialObservation(t *testing.T) {
	cfg := manifestSetupConfig()

	t.Run("secret backend error", func(t *testing.T) {
		want := errors.New("backend failed")
		app := invocation.Context{
			Executable: filepath.Join(t.TempDir(), "aigw"), Secrets: &scriptedSecretStore{existsErr: want}}
		if _, err := collectManifestSetupCredentials(app, cfg, []string{"team"}, "team", false); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("missing token is observed without reading", func(t *testing.T) {
		store := &scriptedSecretStore{values: map[string]string{}}
		app := invocation.Context{Secrets: store, Interactive: true, Prompt: scriptedSetupPrompt{value: "new-token"}}
		credentials, err := collectManifestSetupCredentials(app, cfg, []string{"team"}, "team", false)
		if err != nil || len(credentials) != 1 || credentials[0].token != "new-token" {
			t.Fatalf("credentials=%#v error=%v", credentials, err)
		}
		if store.existsCalls != 1 || store.getCalls != 0 {
			t.Fatalf("exists calls=%d get calls=%d, want 1 and 0", store.existsCalls, store.getCalls)
		}
	})

	t.Run("connected token is read once", func(t *testing.T) {
		store := &scriptedSecretStore{values: map[string]string{"team": "existing-token"}}
		credentials, err := collectManifestSetupCredentials(invocation.Context{Secrets: store}, cfg, []string{"team"}, "team", false)
		if err != nil || len(credentials) != 1 || credentials[0].token != "existing-token" {
			t.Fatalf("credentials=%#v error=%v", credentials, err)
		}
		if store.existsCalls != 1 || store.getCalls != 1 {
			t.Fatalf("exists calls=%d get calls=%d, want 1 and 1", store.existsCalls, store.getCalls)
		}
	})

	t.Run("connected token read failure", func(t *testing.T) {
		want := errors.New("credential read failed")
		store := &scriptedSecretStore{values: map[string]string{"team": "existing-token"}, getErr: want}
		if _, err := collectManifestSetupCredentials(invocation.Context{Secrets: store}, cfg, []string{"team"}, "team", false); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
		if store.existsCalls != 1 || store.getCalls != 1 {
			t.Fatalf("exists calls=%d get calls=%d, want 1 and 1", store.existsCalls, store.getCalls)
		}
	})
}

func TestManifestSetupCredentialInput(t *testing.T) {
	cfg := manifestSetupConfig()
	t.Run("stdin read-only", func(t *testing.T) {
		app := invocation.Context{Secrets: secrets.NewEnvironmentStore(func(string) string { return "" }), In: strings.NewReader("token\n")}
		if _, err := collectManifestSetupCredentials(app, cfg, []string{"team"}, "team", true); err == nil || !strings.Contains(err.Error(), "read-only") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("stdin read", func(t *testing.T) {
		app := invocation.Context{Secrets: secrets.NewMemoryStore(), In: strings.NewReader("")}
		if _, err := collectManifestSetupCredentials(app, cfg, []string{"team"}, "team", true); err == nil {
			t.Fatal("expected stdin read failure")
		}
	})

	t.Run("missing read-only", func(t *testing.T) {
		app := invocation.Context{Secrets: secrets.NewEnvironmentStore(func(string) string { return "" })}
		if _, err := collectManifestSetupCredentials(app, cfg, []string{"team"}, "team", false); err == nil || !strings.Contains(err.Error(), "AIGW_TOKEN_TEAM") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("prompt error", func(t *testing.T) {
		want := errors.New("cancelled")
		app := invocation.Context{Secrets: secrets.NewMemoryStore(), Interactive: true, Prompt: scriptedSetupPrompt{err: want}}
		if _, err := collectManifestSetupCredentials(app, cfg, []string{"team"}, "team", false); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("empty prompt", func(t *testing.T) {
		app := invocation.Context{Secrets: secrets.NewMemoryStore(), Interactive: true, Prompt: scriptedSetupPrompt{}}
		if _, err := collectManifestSetupCredentials(app, cfg, []string{"team"}, "team", false); err == nil || !strings.Contains(err.Error(), "empty Token") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("stdin success", func(t *testing.T) {
		app := invocation.Context{Secrets: secrets.NewMemoryStore(), In: strings.NewReader("new-token\n")}
		credentials, err := collectManifestSetupCredentials(app, cfg, []string{"team"}, "team", true)
		if err != nil || len(credentials) != 1 || !credentials[0].write || credentials[0].token != "new-token" {
			t.Fatalf("credentials=%#v error=%v", credentials, err)
		}
	})

	t.Run("unknown selected account", func(t *testing.T) {
		app := invocation.Context{Secrets: secrets.NewMemoryStore()}
		if _, err := collectManifestSetupCredentials(app, cfg, []string{"team"}, "missing", false); err == nil || !strings.Contains(err.Error(), "unknown Account") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("selected account needs an explicit token outside an interactive terminal", func(t *testing.T) {
		app := invocation.Context{Secrets: secrets.NewMemoryStore()}
		if _, err := collectManifestSetupCredentials(app, cfg, []string{"team"}, "team", false); err == nil || !strings.Contains(err.Error(), "not connected") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestManifestSetupClientSelectionRequiresConnectedRouteAndUsableSurface(t *testing.T) {
	cfg := manifestSetupConfig()
	connected := map[string]setupCredential{"team": {account: "team", token: "token"}}

	withoutCodexSurface := manifestSetupSelectedClients(cfg, connected, map[string]bool{
		configuration.ClientClaude: true,
	})
	if len(withoutCodexSurface) != 1 || withoutCodexSurface[0] != configuration.ClientClaude {
		t.Fatalf("clients without Codex surface = %#v", withoutCodexSurface)
	}

	if clients := manifestSetupSelectedClients(cfg, map[string]setupCredential{}, map[string]bool{
		configuration.ClientClaude: true,
	}); len(clients) != 0 {
		t.Fatalf("unconnected route selected clients = %#v", clients)
	}
}

func TestManifestCredentialVerificationSkipsRoutesOwnedByAnotherAccount(t *testing.T) {
	cfg := manifestSetupConfig()
	cfg.Accounts["other"] = configuration.Account{Label: "Other", Endpoints: configuration.Endpoints{OpenAIResponses: "https://other.test/v1"}}
	calls := 0
	runtime := invocation.Context{HTTP: setupHTTPClient(func(request *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Request: request}, nil
	})}
	if err := verifyManifestSetupCredential(context.Background(), runtime, cfg, "other", "token", "", configuration.ClientCodex); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("verification called another Account's route %d times", calls)
	}
}

func TestGuidedSetupReportsCredentialRollbackDriftAfterConfigurationFailure(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "configuration.toml")
	store := &scriptedSecretStore{
		values: map[string]string{},
		onGet: func(store *scriptedSecretStore, account string) {
			if store.values[account] == "token" {
				delete(store.values, account)
			}
		},
	}
	runtime := invocation.Context{
		Executable: filepath.Join(t.TempDir(), "aigw"),
		Config:     configuration.NewStore(configPath),
		Secrets:    store,
		Discovery:  setupDiscovery{},
		HTTP: setupHTTPClient(func(request *http.Request) (*http.Response, error) {
			if err := os.Mkdir(configPath, 0o700); err != nil {
				t.Fatal(err)
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}")), Request: request}, nil
		}),
		In:        strings.NewReader("token\n"),
		Out:       io.Discard,
		RenderOut: io.Discard,
	}
	request := Request{Account: "team", Profile: "claude", Label: "Team", AnthropicURL: "https://team.test", Client: configuration.ClientClaude, Model: "claude-test", TokenStdin: true}

	err := runSetup(context.Background(), runtime, request)
	if err == nil || !strings.Contains(err.Error(), "setup failed") || !strings.Contains(err.Error(), "credential rollback also failed") || !strings.Contains(err.Error(), "credential postimage changed") {
		t.Fatalf("error = %v, want configuration failure with rollback drift", err)
	}
	if secretExists(t, store, "team") {
		t.Fatal("rollback recreated a credential removed before compensation")
	}
}

func TestFailedSetupPreservesBackendForUncompensatedCredential(t *testing.T) {
	root := t.TempDir()
	secretRoot := filepath.Join(root, "secrets")
	store, err := secrets.Select(secrets.Selection{
		GOOS: runtime.GOOS, Root: secretRoot,
		KeyringProbe: func(secrets.Store) error { return errors.New("isolated file backend") },
	})
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "configuration.toml")
	run := invocation.Context{
		Executable: filepath.Join(root, "aigw"), Config: configuration.NewStore(configPath), Secrets: store,
		Discovery: setupDiscovery{onDiscover: func() {
			if err := store.Set("team", "newer-token"); err != nil {
				t.Fatal(err)
			}
		}},
		HTTP: setupHTTPClient(func(request *http.Request) (*http.Response, error) {
			if err := os.Mkdir(configPath, 0o700); err != nil {
				t.Fatal(err)
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}")), Request: request}, nil
		}),
		In: strings.NewReader("token\n"), Out: io.Discard, RenderOut: io.Discard,
	}
	err = runSetup(t.Context(), run, Request{Account: "team", Profile: "claude", Label: "Team", AnthropicURL: "https://team.test", Client: configuration.ClientClaude, Model: "claude-test", TokenStdin: true})
	if err == nil || !strings.Contains(err.Error(), "credential postimage changed") {
		t.Fatalf("setup error = %v, want uncompensated credential", err)
	}
	if token, err := store.Get("team"); err != nil || token != "newer-token" {
		t.Fatalf("newer credential was not preserved: %v", err)
	}
	selected, err := os.ReadFile(filepath.Join(secretRoot, "backend"))
	if err != nil || strings.TrimSpace(string(selected)) != "file" {
		t.Fatalf("backend selection = %q, %v; want file while credential remains", selected, err)
	}
	fresh, err := secrets.Select(secrets.Selection{
		GOOS: runtime.GOOS, Root: secretRoot,
		KeyringProbe: func(secrets.Store) error {
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
	manifest := `version = 4
[recommended_routes]
codex = "gpt"

[accounts.team]
label = "Team"
[accounts.team.endpoints]
openai_responses = "https://team.test/v1"

[profiles.gpt]
label = "GPT"
account = "team"
client = "codex"
model = "gpt-test"
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	store := &scriptedSecretStore{
		values: map[string]string{},
		onGet: func(store *scriptedSecretStore, account string) {
			delete(store.values, account)
		},
	}
	runtime := invocation.Context{
		Executable: filepath.Join(t.TempDir(), "aigw"),
		Config:     configuration.NewStore(configPath),
		Secrets:    store,
		Discovery: setupDiscovery{result: discovery.Result{
			Executables: map[string]string{configuration.ClientCodex: "/opt/codex"},
			Surfaces: []discovery.Surface{{
				ID:          string(surfaceidentity.CodexHomeDefault),
				Authority:   string(surfaceidentity.AuthorityAIGW),
				ConfigPath:  filepath.Join(t.TempDir(), "codex", "configuration.toml"),
				Present:     true,
				AutoManaged: true,
			}},
		}},
		HTTP: setupHTTPClient(func(request *http.Request) (*http.Response, error) {
			if err := os.Mkdir(configPath, 0o700); err != nil {
				t.Fatal(err)
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}")), Request: request}, nil
		}),
		In:        strings.NewReader("token\n"),
		Out:       io.Discard,
		RenderOut: io.Discard,
	}

	err := runManifestSetup(context.Background(), runtime, Request{From: manifestPath, Account: "team", TokenStdin: true})
	if err == nil || !strings.Contains(err.Error(), "setup failed") || !strings.Contains(err.Error(), "credential rollback also failed") || !strings.Contains(err.Error(), "credential postimage changed") {
		t.Fatalf("error = %v, want configuration failure with rollback drift", err)
	}
	if secretExists(t, store, "team") {
		t.Fatal("rollback recreated a credential removed before compensation")
	}
}

func TestSetupAccountClientAndRuntimeHelpers(t *testing.T) {
	cfg := configuration.NewConfig()
	cfg.Accounts["legacy"] = configuration.Account{Label: "Legacy", Endpoints: configuration.Endpoints{OpenAIResponses: "https://legacy.test/v1", Anthropic: "https://legacy.test"}}
	cfg.Accounts["other"] = configuration.Account{Label: "Other", Endpoints: configuration.Endpoints{Anthropic: "https://other.test"}}
	cfg.Profiles["legacy"] = configuration.Profile{Label: "Legacy", Account: "legacy", Client: configuration.ClientCodex, Model: "gpt-legacy"}
	cfg.Profiles["other"] = configuration.Profile{Label: "Other", Account: "other", Client: configuration.ClientClaude, Model: "claude-test"}
	cfg.Routes[configuration.ClientCodex] = "legacy"

	clients := configuredClientsForAccount(cfg, "legacy")
	if len(clients) != 1 || clients[0] != configuration.ClientCodex {
		t.Fatalf("clients = %#v", clients)
	}
	if runtime, ok := firstRuntimeForAccountClient(cfg, "legacy", configuration.ClientCodex); !ok || runtime.Model != "gpt-legacy" {
		t.Fatalf("runtime = %#v, ok=%v", runtime, ok)
	}
	if runtime, ok := firstRuntimeForAccountClient(cfg, "other", configuration.ClientClaude); !ok || runtime.Model != "claude-test" {
		t.Fatalf("runtime = %#v, ok=%v", runtime, ok)
	}
}

func TestSetupTokenPromptAndBackendErrors(t *testing.T) {
	t.Run("backend error", func(t *testing.T) {
		want := errors.New("backend failed")
		app := invocation.Context{Secrets: &scriptedSecretStore{getErr: want}}
		if _, err := setupToken(app, Request{Account: "one"}); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("prompt", func(t *testing.T) {
		app := invocation.Context{Secrets: secrets.NewMemoryStore(), Prompt: scriptedSetupPrompt{value: "prompt-token"}}
		credential, err := setupToken(app, Request{Account: "one", PromptToken: true, Label: "One"})
		if err != nil || !credential.write || credential.token != "prompt-token" {
			t.Fatalf("credential=%#v error=%v", credential, err)
		}
	})
}

func TestVerifyCredentialRequestAndResponseErrors(t *testing.T) {
	account := configuration.Account{ID: "one", Endpoints: configuration.Endpoints{OpenAIResponses: "https://one.test/v1", Anthropic: "https://one.test"}}

	t.Run("unknown client", func(t *testing.T) {
		err := credential.Validate(context.Background(), nil, account, "token", "other")
		if err == nil || !strings.Contains(err.Error(), "unsupported") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("missing endpoint", func(t *testing.T) {
		err := credential.Validate(context.Background(), nil, configuration.Account{ID: "one", Endpoints: configuration.Endpoints{Anthropic: "https://one.test"}}, "token", configuration.ClientCodex)
		if err == nil || !strings.Contains(err.Error(), "no OpenAI") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("invalid URL", func(t *testing.T) {
		bad := account
		bad.Endpoints.OpenAIResponses = "://bad"
		if err := credential.Validate(context.Background(), nil, bad, "token", configuration.ClientCodex); err == nil {
			t.Fatal("expected request construction failure")
		}
	})

	t.Run("network", func(t *testing.T) {
		want := errors.New("network failed")
		app := invocation.Context{HTTP: setupHTTPClient(func(*http.Request) (*http.Response, error) { return nil, want })}
		if err := credential.Validate(context.Background(), app.HTTP, account, "token", configuration.ClientCodex); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("body read", func(t *testing.T) {
		want := errors.New("read failed")
		app := invocation.Context{HTTP: setupHTTPClient(func(request *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: setupResponseBody{Reader: errorReader{err: want}}, Request: request}, nil
		})}
		if err := credential.Validate(context.Background(), app.HTTP, account, "token", configuration.ClientCodex); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("body close", func(t *testing.T) {
		want := errors.New("close failed")
		app := invocation.Context{HTTP: setupHTTPClient(func(request *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: setupResponseBody{Reader: strings.NewReader("ok"), closeErr: want}, Request: request}, nil
		})}
		if err := credential.Validate(context.Background(), app.HTTP, account, "token", configuration.ClientCodex); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("duplicate client", func(t *testing.T) {
		calls := 0
		app := invocation.Context{HTTP: setupHTTPClient(func(request *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Request: request}, nil
		})}
		if err := credential.Validate(context.Background(), app.HTTP, account, "token", configuration.ClientCodex, configuration.ClientCodex); err != nil || calls != 1 {
			t.Fatalf("calls=%d error=%v", calls, err)
		}
	})
}

func TestRunSetupConfiguresDiscoveredClaudeClient(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	store := configuration.NewStore(path)
	secretStore := secrets.NewMemoryStore()
	if err := secretStore.Set("team", "token"); err != nil {
		t.Fatal(err)
	}
	runtime := invocation.Context{
		Executable:         filepath.Join(t.TempDir(), "aigw"),
		Config:             store,
		Secrets:            secretStore,
		Discovery:          setupDiscovery{result: discovery.Result{Executables: map[string]string{configuration.ClientClaude: "/opt/claude"}}},
		ClaudeSettingsPath: filepath.Join(t.TempDir(), ".claude", "settings.json"),
		HTTP: setupHTTPClient(func(request *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Request: request}, nil
		}),
		Out:         io.Discard,
		RenderOut:   io.Discard,
		Interactive: false,
	}
	request := Request{Account: "team", Profile: "claude", Label: "Team", AnthropicURL: "https://team.test", Client: configuration.ClientClaude, Model: "claude-test"}
	if err := runSetup(context.Background(), runtime, request); err != nil {
		t.Fatal(err)
	}
	cfg, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if adapter := cfg.Adapters[configuration.ClientClaude]; !adapter.Enabled || adapter.Executable != "/opt/claude" {
		t.Fatalf("Claude adapter = %#v", adapter)
	}
}
