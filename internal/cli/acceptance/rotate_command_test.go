package cli_test

import (
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRotateRejectsReadOnlyEnvironmentBackendBeforeInput(t *testing.T) {
	app, out, _, _ := testApp(t, "must-not-be-read\n")
	saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-test")
	app.Secrets = secrets.NewEnvironmentStore(func(string) string { return "" })
	app.Interactive = true
	app.Prompt = &fakePrompt{secretErr: errors.New("prompt must not run")}

	err := execute(t, app, "rotate", "one", "--token-stdin")
	if err == nil || !strings.Contains(err.Error(), "cannot be rotated") {
		t.Fatalf("error = %v", err)
	}
	text := out.String()
	if !strings.Contains(text, secrets.EnvironmentKey("one")) || !strings.Contains(text, "No Token was read, validated, or changed") {
		t.Fatalf("output = %q", text)
	}
	if strings.Contains(err.Error(), "prompt must not run") || strings.Contains(text, "aigw check") {
		t.Fatalf("rotate prompted before rejecting the read-only backend: %v", err)
	}
}

func TestRotateSurfacesInputAndDependencyFailures(t *testing.T) {
	t.Run("config load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "new-token\n")
		app.Config = configuration.NewStore(t.TempDir())
		if err := execute(t, app, "rotate", "one", "--token-stdin"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("unknown account", func(t *testing.T) {
		app, _, _, _ := testApp(t, "new-token\n")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-test")
		err := execute(t, app, "rotate", "missing", "--token-stdin")
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "unknown account or profile") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("secret lookup", func(t *testing.T) {
		app, _, _, _ := testApp(t, "new-token\n")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-test")
		want := errors.New("keychain unavailable")
		app.Secrets = &failingSecretsStore{getErr: want}
		if err := execute(t, app, "rotate", "one", "--token-stdin"); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("non-interactive input", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-test")
		err := execute(t, app, "rotate", "one")
		if err == nil || !strings.Contains(err.Error(), "interactive terminal") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("prompt", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-test")
		want := errors.New("prompt cancelled")
		app.Interactive = true
		app.Prompt = &fakePrompt{secretErr: want}
		if err := execute(t, app, "rotate", "one"); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("token validation", func(t *testing.T) {
		app, _, _, _ := testApp(t, "new-token\n")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-test")
		app.HTTP.(*fakeHTTP).status = http.StatusUnauthorized
		err := execute(t, app, "rotate", "one", "--token-stdin")
		if err == nil || !strings.Contains(err.Error(), "Token validation failed") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("secret write", func(t *testing.T) {
		app, _, _, _ := testApp(t, "new-token\n")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-test")
		want := errors.New("keychain locked")
		app.Secrets = &failingSecretsStore{setErr: want}
		if err := execute(t, app, "rotate", "one", "--token-stdin"); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})
}

func TestRotateReportsProjectionRollbackFailure(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "new-token\n")
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt-test")
	cfg.Routes[configuration.ClientCodex] = "one"
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{t.TempDir()}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("one", "old-token"); err != nil {
		t.Fatal(err)
	}
	err := execute(t, app, "rotate", "one", "--token-stdin")
	if err == nil || !strings.Contains(err.Error(), "rollback also failed") {
		t.Fatalf("error = %v", err)
	}
	if token, getErr := secretStore.Get("one"); getErr != nil || token != "old-token" {
		t.Fatalf("token after rollback = %q, %v", token, getErr)
	}
}

func TestRotateDeletesNewTokenWhenAuthenticationRollbackStartsWithoutOldToken(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "new-token\n")
	target := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt-test")
	cfg.Routes[configuration.ClientCodex] = "one"
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	app.Runner = &failingRunner{err: errors.New("login failed"), remaining: 1}
	err := execute(t, app, "rotate", "one", "--token-stdin")
	if err == nil || !strings.Contains(err.Error(), "rollback also failed") {
		t.Fatalf("error = %v", err)
	}
	if secretExists(t, secretStore, "one") {
		t.Fatal("new token remains after rollback")
	}
}

func TestRotateSynchronizesCodexAuthenticationOnSuccess(t *testing.T) {
	app, out, secretStore, runner := testApp(t, "new-token\n")
	target := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt-test")
	cfg.Routes[configuration.ClientCodex] = "one"
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, app, "rotate", "one", "--token-stdin"); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 1 || !secretExists(t, secretStore, "one") || !strings.Contains(out.String(), "Client authentication synchronized") {
		t.Fatalf("plans=%#v output=%q", runner.plans, out.String())
	}
}

func TestRotateRollsBackSecretWhenAdapterSyncFails(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "new-secret\n")
	dir := t.TempDir()
	target := filepath.Join(dir, "configuration.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt-test")
	cfg.Routes[configuration.ClientCodex] = "one"
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/missing/codex", Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "old-secret")
	app.Runner = &failingRunner{err: errors.New("login failed"), remaining: 1}
	err := execute(t, app, "rotate", "one", "--token-stdin")
	if err == nil || !strings.Contains(err.Error(), "was rolled back") {
		t.Fatalf("error = %v", err)
	}
	got, _ := secretStore.Get("one")
	if got != "old-secret" {
		t.Fatalf("secret = %q, want old-secret", got)
	}
}

func TestRotateClaudeOnlyAccountDoesNotTouchCodexTargets(t *testing.T) {
	app, _, secretStore, runner := testApp(t, "new-claude-token\n")
	cfg := configuration.NewConfig()
	cfg.Accounts["codex-account"] = configuration.Account{Label: "Codex", Endpoints: configuration.Endpoints{OpenAIResponses: "https://codex.test/v1"}}
	cfg.Accounts["claude-account"] = configuration.Account{Label: "Claude", Endpoints: configuration.Endpoints{Anthropic: "https://claude.test"}}
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Account: "codex-account", Client: configuration.ClientCodex, Model: "gpt-test"}
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "claude-account", Client: configuration.ClientClaude, Model: "claude-test"}
	cfg.Routes[configuration.ClientCodex] = "gpt"
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/missing/codex", Targets: []string{filepath.Join(t.TempDir(), "unavailable-codex-configuration.toml")}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("codex-account", "codex-token"); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("claude-account", "old-claude-token"); err != nil {
		t.Fatal(err)
	}
	app.HTTP.(*fakeHTTP).handler = func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "claude.test" || req.Header.Get("X-Api-Key") != "new-claude-token" || req.Header.Get("Authorization") != "" {
			t.Fatalf("Claude token verification request = %s headers=%#v", req.URL, req.Header)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Request: req}, nil
	}

	if err := execute(t, app, "rotate", "claude-account", "--token-stdin"); err != nil {
		t.Fatalf("Claude-only token rotation touched Codex target: %v", err)
	}
	got, err := secretStore.Get("claude-account")
	if err != nil || got != "new-claude-token" {
		t.Fatalf("Claude token = %q, %v", got, err)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("Claude-only token rotation started Codex authentication: %#v", runner.plans)
	}
}
