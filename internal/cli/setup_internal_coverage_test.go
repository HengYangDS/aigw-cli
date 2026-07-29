package cli

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/config"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/secrets"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/shims"
)

type setupCoverageStore struct {
	values       map[string]string
	getErr       error
	setErrors    map[int]error
	deleteErrors map[string]error
	setCalls     int
}

func (store *setupCoverageStore) Get(name string) (string, error) {
	if store.getErr != nil {
		return "", store.getErr
	}
	if value, ok := store.values[name]; ok {
		return value, nil
	}
	return "", secrets.ErrNotFound
}

func (store *setupCoverageStore) Set(name, value string) error {
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

func (store *setupCoverageStore) Delete(name string) error {
	if err := store.deleteErrors[name]; err != nil {
		return err
	}
	delete(store.values, name)
	return nil
}

func (store *setupCoverageStore) Has(name string) bool {
	_, ok := store.values[name]
	return ok
}

type setupCoveragePrompt struct {
	value string
	err   error
}

func (prompt setupCoveragePrompt) Secret(string) (string, error) { return prompt.value, prompt.err }
func (prompt setupCoveragePrompt) Text(string) (string, error)   { return "", prompt.err }
func (prompt setupCoveragePrompt) Select(string, []Choice) (string, error) {
	return "", prompt.err
}

type setupCoverageHTTP func(*http.Request) (*http.Response, error)

func (do setupCoverageHTTP) Do(request *http.Request) (*http.Response, error) { return do(request) }

type setupCoverageBody struct {
	io.Reader
	closeErr error
}

func (body setupCoverageBody) Close() error { return body.closeErr }

type setupCoverageRunner struct {
	err error
}

func (runner setupCoverageRunner) Run(context.Context, adapters.ProcessPlan) error { return runner.err }
func (runner setupCoverageRunner) RunCapture(context.Context, adapters.ProcessPlan) ([]byte, error) {
	return nil, runner.err
}

func setupCoverageConfig() domain.Config {
	cfg := domain.NewConfig()
	cfg.Accounts["team"] = domain.Account{Label: "Team", Endpoints: domain.Endpoints{OpenAIResponses: "https://team.test/v1", Anthropic: "https://team.test"}}
	cfg.Profiles["claude"] = domain.Profile{Label: "Claude", Account: "team", Client: domain.ClientClaude, Models: domain.Models{domain.ClientClaude: "claude-test"}}
	cfg.Profiles["codex"] = domain.Profile{Label: "Codex", Account: "team", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-test"}}
	cfg.Routes.Default = "codex"
	cfg.Routes.Overrides[domain.ClientClaude] = "claude"
	return cfg
}

func TestCollectTeamSetupCredentialsErrorAndPromptBranches(t *testing.T) {
	cfg := setupCoverageConfig()

	t.Run("secret backend error", func(t *testing.T) {
		want := errors.New("backend failed")
		app := &App{Secrets: &setupCoverageStore{getErr: want}}
		if _, err := collectTeamSetupCredentials(app, cfg, []string{"team"}, false); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("stdin read-only", func(t *testing.T) {
		app := &App{Secrets: secrets.NewEnvironmentStore(func(string) string { return "" }), In: strings.NewReader("token\n")}
		if _, err := collectTeamSetupCredentials(app, cfg, []string{"team"}, true); err == nil || !strings.Contains(err.Error(), "read-only") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("stdin read", func(t *testing.T) {
		app := &App{Secrets: secrets.NewMemoryStore(), In: strings.NewReader("")}
		if _, err := collectTeamSetupCredentials(app, cfg, []string{"team"}, true); err == nil {
			t.Fatal("expected stdin read failure")
		}
	})

	t.Run("missing read-only", func(t *testing.T) {
		app := &App{Secrets: secrets.NewEnvironmentStore(func(string) string { return "" })}
		if _, err := collectTeamSetupCredentials(app, cfg, []string{"team"}, false); err == nil || !strings.Contains(err.Error(), "pre-provision") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("prompt error", func(t *testing.T) {
		want := errors.New("cancelled")
		app := &App{Secrets: secrets.NewMemoryStore(), Interactive: true, Prompt: setupCoveragePrompt{err: want}}
		if _, err := collectTeamSetupCredentials(app, cfg, []string{"team"}, false); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("empty prompt", func(t *testing.T) {
		app := &App{Secrets: secrets.NewMemoryStore(), Interactive: true, Prompt: setupCoveragePrompt{}}
		if _, err := collectTeamSetupCredentials(app, cfg, []string{"team"}, false); err == nil || !strings.Contains(err.Error(), "empty Token") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("stdin success", func(t *testing.T) {
		app := &App{Secrets: secrets.NewMemoryStore(), In: strings.NewReader("new-token\n")}
		credentials, err := collectTeamSetupCredentials(app, cfg, []string{"team"}, true)
		if err != nil || len(credentials) != 1 || !credentials[0].write || credentials[0].token != "new-token" {
			t.Fatalf("credentials=%#v error=%v", credentials, err)
		}
	})
}

func TestWriteAndRollbackTeamSetupCredentialsBranches(t *testing.T) {
	credentials := []teamSetupCredential{
		{account: "old", token: "new-old", previous: "old-value", hadPrevious: true, write: true},
		{account: "new", token: "new-value", write: true},
	}

	t.Run("first write fails", func(t *testing.T) {
		want := errors.New("write failed")
		store := &setupCoverageStore{values: map[string]string{}, setErrors: map[int]error{1: want}}
		if _, err := writeTeamSetupCredentials(&App{Secrets: store}, credentials); !errors.Is(err, want) || strings.Contains(err.Error(), "rollback also failed") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("partial write rollback fails", func(t *testing.T) {
		writeErr := errors.New("second write failed")
		rollbackErr := errors.New("restore failed")
		store := &setupCoverageStore{
			values:    map[string]string{"old": "old-value"},
			setErrors: map[int]error{2: writeErr, 3: rollbackErr},
		}
		_, err := writeTeamSetupCredentials(&App{Secrets: store}, credentials)
		if !errors.Is(err, writeErr) || !strings.Contains(err.Error(), "rollback also failed") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("restore previous and delete new", func(t *testing.T) {
		store := &setupCoverageStore{values: map[string]string{"old": "new-old", "new": "new-value"}}
		err := rollbackTeamSetupCredentials(&App{Secrets: store}, credentials, []int{0, 1})
		if err != nil || store.values["old"] != "old-value" || store.Has("new") {
			t.Fatalf("values=%#v error=%v", store.values, err)
		}
	})

	t.Run("delete rollback error", func(t *testing.T) {
		want := errors.New("delete failed")
		store := &setupCoverageStore{values: map[string]string{"new": "new-value"}, deleteErrors: map[string]error{"new": want}}
		err := rollbackTeamSetupCredentials(&App{Secrets: store}, credentials, []int{1})
		if !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})
}

func TestSetupAccountClientAndRuntimeHelpers(t *testing.T) {
	cfg := domain.NewConfig()
	cfg.Accounts["legacy"] = domain.Account{Label: "Legacy", Endpoints: domain.Endpoints{OpenAIResponses: "https://legacy.test/v1", Anthropic: "https://legacy.test"}}
	cfg.Accounts["other"] = domain.Account{Label: "Other", Endpoints: domain.Endpoints{Anthropic: "https://other.test"}}
	cfg.Profiles["legacy"] = domain.Profile{Label: "Legacy"}
	cfg.Profiles["other"] = domain.Profile{Label: "Other", Account: "other", Client: domain.ClientClaude, Models: domain.Models{domain.ClientClaude: "claude-test", "future": "ignored", domain.ClientCodex: ""}}
	cfg.Routes.Default = "legacy"

	clients := configuredClientsForAccount(cfg, "legacy")
	if len(clients) != 2 || clients[0] != domain.ClientClaude || clients[1] != domain.ClientCodex {
		t.Fatalf("clients = %#v", clients)
	}
	if runtime, ok := firstRuntimeForAccountClient(cfg, "legacy", domain.ClientCodex); ok || runtime != (domain.Runtime{}) {
		t.Fatalf("runtime = %#v, ok=%v", runtime, ok)
	}
	if runtime, ok := firstRuntimeForAccountClient(cfg, "other", domain.ClientClaude); !ok || runtime.Model != "claude-test" {
		t.Fatalf("runtime = %#v, ok=%v", runtime, ok)
	}
}

func TestSetupTokenPromptAndBackendErrors(t *testing.T) {
	t.Run("backend error", func(t *testing.T) {
		want := errors.New("backend failed")
		app := &App{Secrets: &setupCoverageStore{getErr: want}}
		if _, _, err := setupToken(app, setupRequest{Account: "one"}); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("prompt", func(t *testing.T) {
		app := &App{Prompt: setupCoveragePrompt{value: "prompt-token"}}
		token, managed, err := setupToken(app, setupRequest{PromptToken: true, Label: "One"})
		if err != nil || managed || token != "prompt-token" {
			t.Fatalf("token=%q managed=%v error=%v", token, managed, err)
		}
	})
}

func TestVerifyCredentialRequestAndResponseErrors(t *testing.T) {
	account := domain.Account{ID: "one", Endpoints: domain.Endpoints{OpenAIResponses: "https://one.test/v1", Anthropic: "https://one.test"}}

	t.Run("unknown client", func(t *testing.T) {
		err := verifyCredential(context.Background(), &App{}, account, "token", "other")
		if err == nil || !strings.Contains(err.Error(), "unsupported") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("missing endpoint", func(t *testing.T) {
		err := verifyCredential(context.Background(), &App{}, domain.Account{ID: "one", Endpoints: domain.Endpoints{Anthropic: "https://one.test"}}, "token", domain.ClientCodex)
		if err == nil || !strings.Contains(err.Error(), "no OpenAI") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("invalid URL", func(t *testing.T) {
		bad := account
		bad.Endpoints.OpenAIResponses = "://bad"
		if err := verifyCredential(context.Background(), &App{}, bad, "token", domain.ClientCodex); err == nil {
			t.Fatal("expected request construction failure")
		}
	})

	t.Run("network", func(t *testing.T) {
		want := errors.New("network failed")
		app := &App{HTTP: setupCoverageHTTP(func(*http.Request) (*http.Response, error) { return nil, want })}
		if err := verifyCredential(context.Background(), app, account, "token", domain.ClientCodex); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("body read", func(t *testing.T) {
		want := errors.New("read failed")
		app := &App{HTTP: setupCoverageHTTP(func(request *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: setupCoverageBody{Reader: errorReader{err: want}}, Request: request}, nil
		})}
		if err := verifyCredential(context.Background(), app, account, "token", domain.ClientCodex); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("body close", func(t *testing.T) {
		want := errors.New("close failed")
		app := &App{HTTP: setupCoverageHTTP(func(request *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: setupCoverageBody{Reader: strings.NewReader("ok"), closeErr: want}, Request: request}, nil
		})}
		if err := verifyCredential(context.Background(), app, account, "token", domain.ClientCodex); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("duplicate client", func(t *testing.T) {
		calls := 0
		app := &App{HTTP: setupCoverageHTTP(func(request *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Request: request}, nil
		})}
		if err := verifyCredential(context.Background(), app, account, "token", domain.ClientCodex, domain.ClientCodex); err != nil || calls != 1 {
			t.Fatalf("calls=%d error=%v", calls, err)
		}
	})
}

func TestSetupEndpointAndClaudeVerificationHelpers(t *testing.T) {
	for input, want := range map[string]string{
		"https://one.test/models/": "https://one.test/models",
		"https://one.test/v1/":     "https://one.test/v1/models",
		"https://one.test/":        "https://one.test/v1/models",
	} {
		if got := anthropicModelsEndpoint(input); got != want {
			t.Errorf("anthropicModelsEndpoint(%q) = %q, want %q", input, got, want)
		}
	}

	cfg := setupCoverageConfig()
	want := errors.New("capture failed")
	app := &App{Runner: setupCoverageRunner{err: want}}
	if err := verifyTeamSetupCredential(context.Background(), app, cfg, "team", "token", "/opt/claude"); !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestRollbackSetupRemovesLauncherSecretAndConfig(t *testing.T) {
	dir := t.TempDir()
	store := secrets.NewMemoryStore()
	if err := store.Set("one", "token"); err != nil {
		t.Fatal(err)
	}
	manager := shims.Manager{GOOS: "linux", BinDir: filepath.Join(dir, "bin"), AIGWExecutable: "/bin/aigw"}
	if _, err := manager.EnableClaude(); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(configPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath+".bak", []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	app := &App{Secrets: store, Shims: manager, Config: config.NewStore(configPath)}
	rollbackSetup(app, "one", true, true)
	if store.Has("one") {
		t.Fatal("secret remains")
	}
	for _, path := range []string{filepath.Join(manager.BinDir, "claude"), configPath, configPath + ".bak"} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s remains: %v", path, err)
		}
	}
}
