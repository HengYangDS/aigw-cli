package cli_test

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/cli"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/config"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

type closeFailingBody struct {
	io.Reader
	err error
}

func (body closeFailingBody) Close() error { return body.err }

func saveDailyProfile(t *testing.T, app *cli.App, endpoints domain.Endpoints, client string, models domain.Models) {
	t.Helper()
	cfg := domain.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", endpoints, client, models)
	cfg.Routes.Default = "one"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestRotateSurfacesInputAndDependencyFailures(t *testing.T) {
	t.Run("config load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "new-token\n")
		app.Config = config.NewStore(t.TempDir())
		if err := execute(t, app, "rotate", "one", "--token-stdin"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("unknown account", func(t *testing.T) {
		app, _, _, _ := testApp(t, "new-token\n")
		saveDailyProfile(t, app, domain.Endpoints{Anthropic: "https://one.test"}, "", domain.Models{})
		err := execute(t, app, "rotate", "missing", "--token-stdin")
		if err == nil || !strings.Contains(err.Error(), "Unknown account or profile") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("secret lookup", func(t *testing.T) {
		app, _, _, _ := testApp(t, "new-token\n")
		saveDailyProfile(t, app, domain.Endpoints{Anthropic: "https://one.test"}, "", domain.Models{})
		want := errors.New("keychain unavailable")
		app.Secrets = &failingSecretsStore{getErr: want}
		if err := execute(t, app, "rotate", "one", "--token-stdin"); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("non-interactive input", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveDailyProfile(t, app, domain.Endpoints{Anthropic: "https://one.test"}, "", domain.Models{})
		err := execute(t, app, "rotate", "one")
		if err == nil || !strings.Contains(err.Error(), "interactive terminal") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("prompt", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveDailyProfile(t, app, domain.Endpoints{Anthropic: "https://one.test"}, "", domain.Models{})
		want := errors.New("prompt cancelled")
		app.Interactive = true
		app.Prompt = &fakePrompt{secretErr: want}
		if err := execute(t, app, "rotate", "one"); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("token validation", func(t *testing.T) {
		app, _, _, _ := testApp(t, "new-token\n")
		saveDailyProfile(t, app, domain.Endpoints{Anthropic: "https://one.test"}, "", domain.Models{})
		app.HTTP.(*fakeHTTP).status = http.StatusUnauthorized
		err := execute(t, app, "rotate", "one", "--token-stdin")
		if err == nil || !strings.Contains(err.Error(), "Token validation failed") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("secret write", func(t *testing.T) {
		app, _, _, _ := testApp(t, "new-token\n")
		saveDailyProfile(t, app, domain.Endpoints{Anthropic: "https://one.test"}, "", domain.Models{})
		want := errors.New("keychain locked")
		app.Secrets = &failingSecretsStore{setErr: want}
		if err := execute(t, app, "rotate", "one", "--token-stdin"); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})
}

func TestRotateReportsProjectionRollbackFailure(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "new-token\n")
	cfg := domain.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, domain.ClientCodex, domain.Models{domain.ClientCodex: "gpt-test"})
	cfg.Routes.Default = "one"
	cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{t.TempDir()}}
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
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, domain.ClientCodex, domain.Models{domain.ClientCodex: "gpt-test"})
	cfg.Routes.Default = "one"
	cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	app.Runner = &failingRunner{err: errors.New("login failed"), remaining: 1}
	err := execute(t, app, "rotate", "one", "--token-stdin")
	if err == nil || !strings.Contains(err.Error(), "rollback also failed") {
		t.Fatalf("error = %v", err)
	}
	if secretStore.Has("one") {
		t.Fatal("new token remains after rollback")
	}
}

func TestRotateSynchronizesCodexAuthenticationOnSuccess(t *testing.T) {
	app, out, secretStore, runner := testApp(t, "new-token\n")
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, domain.ClientCodex, domain.Models{domain.ClientCodex: "gpt-test"})
	cfg.Routes.Default = "one"
	cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, app, "rotate", "one", "--token-stdin"); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 1 || !secretStore.Has("one") || !strings.Contains(out.String(), "Codex authentication synchronized") {
		t.Fatalf("plans=%#v output=%q", runner.plans, out.String())
	}
}

func TestTestCommandSurfacesOperationalFailures(t *testing.T) {
	t.Run("config load", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		app.Config = config.NewStore(t.TempDir())
		if err := execute(t, app, "test"); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("not configured", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		err := execute(t, app, "test")
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "not configured") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("missing token", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveDailyProfile(t, app, domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, domain.ClientCodex, domain.Models{domain.ClientCodex: "gpt"})
		err := execute(t, app, "test", "--for", "codex")
		if err == nil || !strings.Contains(err.Error(), "is unavailable") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("transport", func(t *testing.T) {
		app, _, secretStore, _ := testApp(t, "")
		saveDailyProfile(t, app, domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, domain.ClientCodex, domain.Models{domain.ClientCodex: "gpt"})
		_ = secretStore.Set("one", "token")
		want := errors.New("network down")
		app.HTTP.(*fakeHTTP).handler = func(*http.Request) (*http.Response, error) { return nil, want }
		if err := execute(t, app, "test", "--for", "codex"); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("response close", func(t *testing.T) {
		app, _, secretStore, _ := testApp(t, "")
		saveDailyProfile(t, app, domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, domain.ClientCodex, domain.Models{domain.ClientCodex: "gpt"})
		_ = secretStore.Set("one", "token")
		want := errors.New("close failed")
		app.HTTP.(*fakeHTTP).handler = func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: closeFailingBody{Reader: strings.NewReader("ok"), err: want}, Request: req}, nil
		}
		if err := execute(t, app, "test", "--for", "codex"); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("server status", func(t *testing.T) {
		app, _, secretStore, _ := testApp(t, "")
		saveDailyProfile(t, app, domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, domain.ClientCodex, domain.Models{domain.ClientCodex: "gpt"})
		_ = secretStore.Set("one", "token")
		app.HTTP.(*fakeHTTP).status = http.StatusInternalServerError
		err := execute(t, app, "test", "--for", "codex")
		if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestSyncHumanPreviewHandlesDisabledAndEnabledAdapters(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		app, out, _, _ := testApp(t, "")
		if err := execute(t, app, "sync", "--dry-run"); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "Adapter is disabled") {
			t.Fatalf("output = %q", out.String())
		}
	})

	t.Run("enabled", func(t *testing.T) {
		app, out, _, _ := testApp(t, "")
		target := filepath.Join(t.TempDir(), "config.toml")
		if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		cfg := domain.NewConfig()
		addAccountProfile(&cfg, "one", "one", "One", domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, domain.ClientCodex, domain.Models{domain.ClientCodex: "gpt"})
		cfg.Routes.Default = "one"
		cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
		if err := app.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		if err := execute(t, app, "sync", "--dry-run"); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), target) || !strings.Contains(out.String(), "initial-project") {
			t.Fatalf("output = %q", out.String())
		}
	})
}

func TestRollbackHandlesAbsentAndLastChangeBackups(t *testing.T) {
	t.Run("no restore source", func(t *testing.T) {
		app, _, _, _ := testApp(t, "")
		saveDailyProfile(t, app, domain.Endpoints{Anthropic: "https://one.test"}, domain.ClientClaude, domain.Models{domain.ClientClaude: "claude-one"})
		err := execute(t, app, "rollback")
		if err == nil || !strings.Contains(err.Error(), "No fully verified checkpoint") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("last change", func(t *testing.T) {
		app, out, _, _ := testApp(t, "")
		before := domain.NewConfig()
		addAccountProfile(&before, "one", "one", "One", domain.Endpoints{Anthropic: "https://one.test"}, domain.ClientClaude, domain.Models{domain.ClientClaude: "claude-one"})
		before.Routes.Default = "one"
		if err := app.Config.Save(before); err != nil {
			t.Fatal(err)
		}
		after := before
		after.Profiles = map[string]domain.Profile{"two": {Label: "Two", Account: "one", Client: domain.ClientClaude, Models: domain.Models{domain.ClientClaude: "claude-two"}}}
		after.Routes.Default = "two"
		if err := app.Config.Save(after); err != nil {
			t.Fatal(err)
		}
		if err := execute(t, app, "rollback", "--last-change"); err != nil {
			t.Fatal(err)
		}
		got, err := app.Config.Load()
		if err != nil || got.Routes.Default != "one" || !strings.Contains(out.String(), "Previous configuration") {
			t.Fatalf("config=%#v output=%q error=%v", got, out.String(), err)
		}
	})
}

func TestVerifyCommandRejectsInvalidInputsAndMissingState(t *testing.T) {
	tests := []struct {
		name string
		args []string
		prep func(*cli.App)
		want string
	}{
		{name: "unknown client", args: []string{"verify", "--for", "bogus"}, want: "--for must be"},
		{name: "profile with all", args: []string{"verify", "--for", "all", "--profile", "one"}, want: "--profile cannot be used"},
		{name: "config load", args: []string{"verify", "--for", "codex"}, prep: func(app *cli.App) { app.Config = config.NewStore(t.TempDir()) }, want: "read config"},
		{name: "unknown profile", args: []string{"verify", "--for", "codex", "--profile", "missing"}, prep: func(app *cli.App) {
			saveDailyProfile(t, app, domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, domain.ClientCodex, domain.Models{domain.ClientCodex: "gpt"})
		}, want: "unknown profile"},
		{name: "missing token", args: []string{"verify", "--for", "codex"}, prep: func(app *cli.App) {
			saveDailyProfile(t, app, domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, domain.ClientCodex, domain.Models{domain.ClientCodex: "gpt"})
		}, want: "is unavailable"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app, _, _, _ := testApp(t, "")
			if test.prep != nil {
				test.prep(app)
			}
			err := execute(t, app, test.args...)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}
