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

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
)

func TestRotateProviderNamePromptsWithProviderLabel(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["dmx"] = domain.Provider{Label: "DMXAPI", Endpoints: domain.Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Models["gpt-5.6-sol"] = qualifiedModel("GPT Route", "dmx", "gpt-5.6-sol", domain.ProtocolOpenAIResponses)
	cfg.SetSelectedModel(codex.ID, "gpt-5.6-sol", "")
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
	output := strings.Join(strings.Fields(out.String()), " ")
	for _, want := range []string{"Account DMXAPI", "Account ID dmx", "Account Token Validated and securely stored"} {
		if !strings.Contains(output, want) {
			t.Fatalf("rotate output lacks %q:\n%s", want, out.String())
		}
	}
}

func TestRotateWithoutNameRefusesAmbiguousProviders(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "")
	cfg := twoModelConfig()
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
	saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test")
	app.Secrets = secret.NewEnvironmentStore(func(string) string { return "" })
	app.Interactive = true
	app.Prompt = &scriptedPrompt{secretErr: errors.New("prompt must not run")}

	err := cli.Execute(app, []string{"rotate", "one", "--token-stdin"})
	if err == nil || !strings.Contains(err.Error(), "cannot be rotated") {
		t.Fatalf("error = %v", err)
	}
	text := out.String()
	if !strings.Contains(text, secret.EnvironmentKey("one")) || !strings.Contains(text, "No Token was read, validated, or changed") {
		t.Fatalf("output = %q", text)
	}
	if strings.Contains(err.Error(), "prompt must not run") || strings.Contains(text, "aigw check") {
		t.Fatalf("rotate prompted before rejecting the read-only backend: %v", err)
	}
}

func TestRotateSurfacesInputAndDependencyFailures(t *testing.T) {
	t.Run("config load", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "new-token\n")
		app.Config = store.NewStore(t.TempDir(), clients.Admission())
		if err := cli.Execute(app, []string{"rotate", "one", "--token-stdin"}); err == nil {
			t.Fatal("expected config load failure")
		}
	})

	t.Run("unknown account", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "new-token\n")
		saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test")
		err := cli.Execute(app, []string{"rotate", "missing", "--token-stdin"})
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "unknown account or route") {
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
			saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test")
			app.Secrets = &recordingCredentialStore[string]{backend: secret.NewMemoryStore(), getErr: test.get, setErr: test.set}
			want := cmp.Or(test.get, test.set)
			if err := cli.Execute(app, []string{"rotate", "one", "--token-stdin"}); !errors.Is(err, want) {
				t.Fatalf("error = %v, want %v", err, want)
			}
		})
	}

	t.Run("non-interactive input", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test")
		err := cli.Execute(app, []string{"rotate", "one"})
		if err == nil || !strings.Contains(err.Error(), "interactive terminal") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("prompt", func(t *testing.T) {
		app, _, _, _, _ := testApp(t, "")
		saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test")
		want := errors.New("prompt cancelled")
		app.Interactive = true
		app.Prompt = &scriptedPrompt{secretErr: want}
		if err := cli.Execute(app, []string{"rotate", "one"}); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})

	t.Run("token validation", func(t *testing.T) {
		app, _, _, _, httpClient := testApp(t, "new-token\n")
		saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test")
		httpClient.status = http.StatusUnauthorized
		err := cli.Execute(app, []string{"rotate", "one", "--token-stdin"})
		if err == nil || !strings.Contains(err.Error(), "Token validation failed") {
			t.Fatalf("error = %v", err)
		}
	})
}

// A Token that rotation could not validate names the retry when the provider
// rejected it or the endpoint did not answer, the endpoint's edit for any
// other status, and the previous Token only when one exists.
func TestRotateNamesTheFixOfItsValidationFailure(t *testing.T) {
	offline := func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") }
	for _, test := range []struct {
		name     string
		args     []string
		status   int
		handler  func(*http.Request) (*http.Response, error)
		previous bool
		fix      string
		impact   string
	}{
		{name: "prompted Token", args: []string{"rotate", "one"}, status: http.StatusUnauthorized,
			fix: "aigw rotate one", impact: "No Token was stored."},
		{name: "piped Token", args: []string{"rotate", "one", "--token-stdin"}, status: http.StatusForbidden, previous: true,
			fix: "aigw rotate one --token-stdin", impact: "No Token was stored; the previous Token is unchanged."},
		{name: "unreachable endpoint", args: []string{"rotate", "--token-stdin"}, handler: offline,
			fix: "aigw rotate --token-stdin", impact: "No Token was stored."},
		{name: "another status", args: []string{"rotate", "one", "--token-stdin"}, status: http.StatusNotFound,
			fix: "aigw account edit one --anthropic-url <url>", impact: "No Token was stored."},
		{name: "server error", args: []string{"rotate", "one", "--token-stdin"}, status: http.StatusServiceUnavailable,
			fix: "aigw rotate one --token-stdin", impact: "No Token was stored."},
		{name: "rate limit", args: []string{"rotate", "one", "--token-stdin"}, status: http.StatusTooManyRequests,
			fix: "aigw rotate one --token-stdin", impact: "No Token was stored."},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, out, secretStore, _, httpClient := testApp(t, "new-token\n")
			saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test")
			if test.previous {
				if err := secretStore.Set("one", "old-token"); err != nil {
					t.Fatal(err)
				}
			}
			httpClient.status, httpClient.handler = test.status, test.handler
			app.Interactive, app.Prompt = true, &scriptedPrompt{secrets: []string{"new-token"}}
			if err := cli.Execute(app, test.args); err == nil {
				t.Fatal("an unvalidated Token was stored")
			}
			if !strings.Contains(out.String(), "Impact\n  "+test.impact+"\n") || !strings.Contains(out.String(), "Fix\n  "+test.fix+"\n") {
				t.Fatalf("rotation failure output:\n%s", out.String())
			}
		})
	}
	// As JSON, the retry keeps --json, and a script can run it as it stands.
	app, out, _, _, httpClient := testApp(t, "new-token\n")
	saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test")
	httpClient.status = http.StatusUnauthorized
	if err := cli.Execute(app, []string{"rotate", "one", "--token-stdin", "--json"}); err == nil ||
		!strings.Contains(out.String(), `"next_action": "aigw rotate one --json --token-stdin"`) {
		t.Fatalf("rotation failure document = %v\n%s", err, out.String())
	}
}

// Without a terminal, rotate refuses a Token prompt, and its fix pipes the
// Token to provider rotate, which outlives rotate.
func TestRotateWithoutATerminalNamesProviderRotate(t *testing.T) {
	app, out, _, _, httpClient := testApp(t, "")
	saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test")
	if err := cli.Execute(app, []string{"rotate", "one"}); err == nil || httpClient.calls != 0 ||
		!strings.Contains(out.String(), "Fix\n  aigw provider rotate one --token-stdin\n") {
		t.Fatalf("rotate without a terminal = %v, %d requests\n%s", err, httpClient.calls, out.String())
	}
}

func TestRotateLeavesClientConfigurationOutsideItsScope(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "new-token\n")
	cfg := domain.NewConfig(clients.Admission())
	addProviderModel(&cfg, "one", "one", "One", domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, codex.ID, "gpt-test")
	cfg.SetSelectedModel(codex.ID, "one", "")
	cfg.SetClientActivation(codex.ID, true, "/opt/codex", []string{t.TempDir()})
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
			cfg := domain.NewConfig(clients.Admission())
			addProviderModel(&cfg, "one", "one", "One", domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, codex.ID, "gpt-test")
			cfg.SetSelectedModel(codex.ID, "one", "")
			cfg.SetClientActivation(codex.ID, true, "/opt/codex", []string{target})
			wantMessage := "clients control their refresh timing"
			if !hasTarget {
				cfg.SetClientActivation(codex.ID, true, "/opt/codex", nil)
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
	saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test")
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

func TestRotateClaudeOnlyProviderDoesNotTouchCodexTargets(t *testing.T) {
	app, _, secretStore, runner, httpClient := testApp(t, "new-claude-token\n")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["codex-account"] = domain.Provider{Label: "Codex", Endpoints: domain.Endpoints{OpenAIResponses: "https://codex.test/v1"}}
	cfg.Providers["claude-account"] = domain.Provider{Label: "Claude", Endpoints: domain.Endpoints{Anthropic: "https://claude.test"}}
	cfg.Models["gpt"] = qualifiedModel("GPT", "codex-account", "gpt-test", domain.ProtocolOpenAIResponses)
	cfg.Models["claude"] = qualifiedModel("Claude", "claude-account", "claude-test", domain.ProtocolAnthropic)
	cfg.SetSelectedModel(codex.ID, "gpt", "")
	cfg.SetSelectedModel(claude.ID, "claude", "")
	cfg.SetClientActivation(codex.ID, true, "/missing/codex", []string{filepath.Join(t.TempDir(), "unavailable-codex-configuration.toml")})
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
