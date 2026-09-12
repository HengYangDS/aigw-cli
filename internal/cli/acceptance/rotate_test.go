package cli_test

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

func TestRotateAccountNamePromptsWithAccountLabel(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMXAPI", Endpoints: configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Profiles["gpt-5.6-sol"] = configuration.Profile{Label: "GPT Profile", Account: "dmx", Client: configuration.ClientCodex, Model: "gpt-5.6-sol"}
	cfg.Routes[configuration.ClientCodex] = "gpt-5.6-sol"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "old-token")
	prompt := &scriptedPrompt{secrets: []string{"new-token"}}
	app.Interactive = true
	app.Prompt = prompt
	if err := cli.Execute(app, []string{"rotate", "dmx"}); err != nil {
		t.Fatal(err)
	}
	if len(prompt.secretCalls) != 1 || prompt.secretCalls[0] != "Paste DMXAPI token: " {
		t.Fatalf("prompt labels = %q", prompt.secretCalls)
	}
}

func TestRotateWithoutNameRefusesAmbiguousAccounts(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	cfg := twoProfileConfig()
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "old-token")
	prompt := &scriptedPrompt{secrets: []string{"new-token"}}
	app.Interactive = true
	app.Prompt = prompt
	err := cli.Execute(app, []string{"rotate"})
	if err == nil || !strings.Contains(err.Error(), "account is required") {
		t.Fatalf("error = %v, want explicit account guidance", err)
	}
	got, _ := secretStore.Get("one")
	if got != "old-token" || len(prompt.secretCalls) != 0 {
		t.Fatalf("token=%q prompts=%d", got, len(prompt.secretCalls))
	}
}

func TestRotateRejectsReadOnlyEnvironmentBackendBeforeInput(t *testing.T) {
	app, out, _, _, _ := testApp(t, "must-not-be-read\n")
	saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-test")
	app.Secrets = secrets.NewEnvironmentStore(func(string) string { return "" })
	app.Interactive = true
	app.Prompt = &scriptedPrompt{secretErr: errors.New("prompt must not run")}

	err := cli.Execute(app, []string{"rotate", "one", "--token-stdin"})
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
		app, _, _, _, _ := testApp(t, "new-token\n")
		app.Config = configuration.NewStore(t.TempDir())
		if err := cli.Execute(app, []string{"rotate", "one", "--token-stdin"}); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("unknown account", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "new-token\n")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-test")
		err := cli.Execute(app, []string{"rotate", "missing", "--token-stdin"})
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "unknown account or profile") {
			t.Fatalf("error = %v", err)
		}
	})

	for _, test := range []struct {
		name     string
		get, set error
	}{
		{"secret lookup", errors.New("keychain unavailable"), nil},
		{"secret write", nil, errors.New("keychain locked")},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, _, _, _, _ := testApp(t, "new-token\n")
			saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-test")
			app.Secrets = &recordingCredentialStore[string]{backend: secrets.NewMemoryStore(), getErr: test.get, setErr: test.set}
			want := cmp.Or(test.get, test.set)
			if err := cli.Execute(app, []string{"rotate", "one", "--token-stdin"}); !errors.Is(err, want) {
				t.Fatalf("error = %v, want %v", err, want)
			}
		})
	}

	t.Run("non-interactive input", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-test")
		err := cli.Execute(app, []string{"rotate", "one"})
		if err == nil || !strings.Contains(err.Error(), "interactive terminal") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("prompt", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-test")
		want := errors.New("prompt cancelled")
		app.Interactive = true
		app.Prompt = &scriptedPrompt{secretErr: want}
		if err := cli.Execute(app, []string{"rotate", "one"}); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("token validation", func(t *testing.T) {
		app, _, _, _, httpClient := testApp(t, "new-token\n")
		saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-test")
		httpClient.status = http.StatusUnauthorized
		err := cli.Execute(app, []string{"rotate", "one", "--token-stdin"})
		if err == nil || !strings.Contains(err.Error(), "Token validation failed") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestRotateLeavesClientConfigurationOutsideItsScope(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "new-token\n")
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
	err := cli.Execute(app, []string{"rotate", "one", "--token-stdin"})
	if err != nil {
		t.Fatalf("rotation depended on client configuration: %v", err)
	}
	if token, getErr := secretStore.Get("one"); getErr != nil || token != "new-token" {
		t.Fatalf("rotated token = %q, %v", token, getErr)
	}
}

func TestRotateReportsTokenStorageWithoutNativeClientWrites(t *testing.T) {
	for _, hasTarget := range []bool{true, false} {
		t.Run(fmt.Sprintf("target=%t", hasTarget), func(t *testing.T) {
			app, out, secretStore, runner, _ := testApp(t, "new-token\n")
			target := filepath.Join(t.TempDir(), "configuration.toml")
			if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg := configuration.NewConfig()
			addAccountProfile(&cfg, "one", "one", "One", configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "gpt-test")
			cfg.Routes[configuration.ClientCodex] = "one"
			cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{target}}
			wantMessage := "clients control their refresh timing"
			if !hasTarget {
				cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/codex"}
			}
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := cli.Execute(app, []string{"rotate", "one", "--token-stdin"}); err != nil {
				t.Fatal(err)
			}
			if len(runner.plans) != 0 || !secretExists(t, secretStore, "one") || !strings.Contains(out.String(), wantMessage) {
				t.Fatalf("binding calls=%d output=%q", len(runner.plans), out.String())
			}
		})
	}
}

func TestRotateValidationUsesCommandContext(t *testing.T) {
	app, _, store, _, httpClient := testApp(t, "new-token\n")
	saveCommandProfile(t, app, configuration.Endpoints{Anthropic: "https://one.test"}, configuration.ClientClaude, "claude-test")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	httpClient.handler = func(req *http.Request) (*http.Response, error) {
		if err := req.Context().Err(); err != nil {
			return nil, err
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Request: req}, nil
	}
	command := cli.NewRoot(app)
	command.SetArgs([]string{"rotate", "one", "--token-stdin"})
	if err := command.ExecuteContext(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("rotation lost command cancellation: %v", err)
	}
	if secretExists(t, store, "one") {
		t.Fatal("cancelled rotation persisted a token")
	}
}

func TestRotateClaudeOnlyAccountDoesNotTouchCodexTargets(t *testing.T) {
	app, _, secretStore, runner, httpClient := testApp(t, "new-claude-token\n")
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
	httpClient.handler = func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "claude.test" || req.Header.Get("X-Api-Key") != "new-claude-token" || req.Header.Get("Authorization") != "" {
			t.Fatalf("Claude token verification request = %s headers=%#v", req.URL, req.Header)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Request: req}, nil
	}

	if err := cli.Execute(app, []string{"rotate", "claude-account", "--token-stdin"}); err != nil {
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
