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
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"
)

type scriptedSecretStore struct {
	values       map[string]string
	getErr       error
	setErrors    map[int]error
	deleteErrors map[string]error
	setCalls     int
}

func (store *scriptedSecretStore) Get(name string) (string, error) {
	if store.getErr != nil {
		return "", store.getErr
	}
	if value, ok := store.values[name]; ok {
		return value, nil
	}
	return "", secrets.ErrNotFound
}

func (store *scriptedSecretStore) Set(name, value string) error {
	store.setCalls++
	if err := store.setErrors[store.setCalls]; err != nil {
		return err
	}
	if store.values == nil {
		store.values = map[string]string{}
	}
	store.values[name] = value
	return nil
}

func (store *scriptedSecretStore) Delete(name string) error {
	if err := store.deleteErrors[name]; err != nil {
		return err
	}
	delete(store.values, name)
	return nil
}

func (store *scriptedSecretStore) Has(name string) bool {
	_, ok := store.values[name]
	return ok
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

type setupProcessRunner struct {
	err error
}

type setupDiscovery struct{ result discovery.Result }

func (candidate setupDiscovery) Discover() discovery.Result { return candidate.result }

func (runner setupProcessRunner) Run(context.Context, process.Plan) error { return runner.err }
func (runner setupProcessRunner) RunCapture(context.Context, process.Plan) ([]byte, error) {
	return nil, runner.err
}

func manifestSetupConfig() configuration.Config {
	cfg := configuration.NewConfig()
	cfg.Accounts["team"] = configuration.Account{Label: "Team", Endpoints: configuration.Endpoints{OpenAIResponses: "https://team.test/v1", Anthropic: "https://team.test"}}
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "team", Client: configuration.ClientClaude, Models: configuration.Models{configuration.ClientClaude: "claude-test"}}
	cfg.Profiles["codex"] = configuration.Profile{Label: "Codex", Account: "team", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-test"}}
	cfg.Routes.Default = "codex"
	cfg.Routes.Overrides[configuration.ClientClaude] = "claude"
	return cfg
}

func TestCollectManifestSetupCredentialsErrorAndPromptBranches(t *testing.T) {
	cfg := manifestSetupConfig()

	t.Run("secret backend error", func(t *testing.T) {
		want := errors.New("backend failed")
		app := invocation.Context{
			Executable: filepath.Join(t.TempDir(), "aigw"), Secrets: &scriptedSecretStore{getErr: want}}
		if _, err := collectManifestSetupCredentials(app, cfg, []string{"team"}, "team", false); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

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
	connected := map[string]manifestSetupCredential{"team": {account: "team", token: "token"}}

	withoutCodexSurface := manifestSetupSelectedClients(cfg, connected, map[string]bool{
		configuration.ClientClaude: true,
	})
	if len(withoutCodexSurface) != 1 || withoutCodexSurface[0] != configuration.ClientClaude {
		t.Fatalf("clients without Codex surface = %#v", withoutCodexSurface)
	}

	if clients := manifestSetupSelectedClients(cfg, map[string]manifestSetupCredential{}, map[string]bool{
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

func TestWriteAndRollbackManifestSetupCredentialsBranches(t *testing.T) {
	credentials := []manifestSetupCredential{
		{account: "old", token: "new-old", previous: "old-value", hadPrevious: true, write: true},
		{account: "new", token: "new-value", write: true},
	}

	t.Run("first write fails", func(t *testing.T) {
		want := errors.New("write failed")
		store := &scriptedSecretStore{values: map[string]string{}, setErrors: map[int]error{1: want}}
		if _, err := writeManifestSetupCredentials(invocation.Context{Secrets: store}, credentials); !errors.Is(err, want) || strings.Contains(err.Error(), "rollback also failed") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("partial write rollback fails", func(t *testing.T) {
		writeErr := errors.New("second write failed")
		rollbackErr := errors.New("restore failed")
		store := &scriptedSecretStore{
			values:    map[string]string{"old": "old-value"},
			setErrors: map[int]error{2: writeErr, 3: rollbackErr},
		}
		_, err := writeManifestSetupCredentials(invocation.Context{Secrets: store}, credentials)
		if !errors.Is(err, writeErr) || !strings.Contains(err.Error(), "rollback also failed") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("restore previous and delete new", func(t *testing.T) {
		store := &scriptedSecretStore{values: map[string]string{"old": "new-old", "new": "new-value"}}
		err := rollbackManifestSetupCredentials(invocation.Context{Secrets: store}, credentials, []int{0, 1})
		if err != nil || store.values["old"] != "old-value" || store.Has("new") {
			t.Fatalf("values=%#v error=%v", store.values, err)
		}
	})

	t.Run("delete rollback error", func(t *testing.T) {
		want := errors.New("delete failed")
		store := &scriptedSecretStore{values: map[string]string{"new": "new-value"}, deleteErrors: map[string]error{"new": want}}
		err := rollbackManifestSetupCredentials(invocation.Context{Secrets: store}, credentials, []int{1})
		if !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})
}

func TestSetupAccountClientAndRuntimeHelpers(t *testing.T) {
	cfg := configuration.NewConfig()
	cfg.Accounts["legacy"] = configuration.Account{Label: "Legacy", Endpoints: configuration.Endpoints{OpenAIResponses: "https://legacy.test/v1", Anthropic: "https://legacy.test"}}
	cfg.Accounts["other"] = configuration.Account{Label: "Other", Endpoints: configuration.Endpoints{Anthropic: "https://other.test"}}
	cfg.Profiles["legacy"] = configuration.Profile{Label: "Legacy"}
	cfg.Profiles["other"] = configuration.Profile{Label: "Other", Account: "other", Client: configuration.ClientClaude, Models: configuration.Models{configuration.ClientClaude: "claude-test", "future": "ignored", configuration.ClientCodex: ""}}
	cfg.Routes.Default = "legacy"

	clients := configuredClientsForAccount(cfg, "legacy")
	if len(clients) != 2 || clients[0] != configuration.ClientClaude || clients[1] != configuration.ClientCodex {
		t.Fatalf("clients = %#v", clients)
	}
	if runtime, ok := firstRuntimeForAccountClient(cfg, "legacy", configuration.ClientCodex); ok || runtime != (configuration.Runtime{}) {
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
		if _, _, err := setupToken(app, Request{Account: "one"}); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("prompt", func(t *testing.T) {
		app := invocation.Context{Prompt: scriptedSetupPrompt{value: "prompt-token"}}
		token, managed, err := setupToken(app, Request{PromptToken: true, Label: "One"})
		if err != nil || managed || token != "prompt-token" {
			t.Fatalf("token=%q managed=%v error=%v", token, managed, err)
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

func TestSetupClaudeVerificationHelper(t *testing.T) {
	cfg := manifestSetupConfig()
	want := errors.New("capture failed")
	app := invocation.Context{Runner: setupProcessRunner{err: want}}
	if err := verifyManifestSetupCredential(context.Background(), app, cfg, "team", "token", "/opt/claude", configuration.ClientClaude); !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestRollbackSetupRemovesSecretAndConfig(t *testing.T) {
	dir := t.TempDir()
	store := secrets.NewMemoryStore()
	if err := store.Set("one", "token"); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "configuration.toml")
	if err := os.WriteFile(configPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath+".bak", []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	app := invocation.Context{Secrets: store, Config: configuration.NewStore(configPath)}
	rollbackSetup(app, "one", true)
	if store.Has("one") {
		t.Fatal("secret remains")
	}
	for _, path := range []string{configPath, configPath + ".bak"} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s remains: %v", path, err)
		}
	}
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
