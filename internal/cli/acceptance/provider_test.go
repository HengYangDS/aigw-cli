package cli_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
)

// The provider commands own what `aigw rotate`, `aigw balance` and `aigw add`
// covered beyond their own tests: each validation failure of a rotation names
// its fix, a rotation reaches no client's files, a balance request keeps its
// credentials at their origin, and an add that cannot take the configuration
// lock touches no credential.

// A Token that rotation could not validate names the same rotation with a
// Token that the provider accepts when the provider rejected it, the retry
// when the endpoint did not answer, the endpoint's edit for any other status,
// and the previous Token only when one exists.
func TestProviderRotateNamesTheFixOfItsValidationFailure(t *testing.T) {
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
		{name: "prompted Token", args: []string{"provider", "rotate", "one"}, status: http.StatusUnauthorized,
			fix: "Run `aigw provider rotate one` again and enter a Token that provider one accepts.", impact: "No Token was stored."},
		{name: "piped Token", args: []string{"provider", "rotate", "one", "--token-stdin"}, status: http.StatusForbidden, previous: true,
			fix: "Pipe a Token that provider one accepts into `aigw provider rotate one --token-stdin`.", impact: "No Token was stored; the previous Token is unchanged."},
		{name: "unreachable endpoint", args: []string{"provider", "rotate", "one", "--token-stdin"}, handler: offline,
			fix: "aigw provider rotate one --token-stdin", impact: "No Token was stored."},
		{name: "another status", args: []string{"provider", "rotate", "one", "--token-stdin"}, status: http.StatusNotFound,
			fix: "aigw provider edit one --endpoint anthropic=<url>", impact: "No Token was stored."},
		{name: "server error", args: []string{"provider", "rotate", "one", "--token-stdin"}, status: http.StatusServiceUnavailable,
			fix: "aigw provider rotate one --token-stdin", impact: "No Token was stored."},
		{name: "rate limit", args: []string{"provider", "rotate", "one", "--token-stdin"}, status: http.StatusTooManyRequests,
			fix: "aigw provider rotate one --token-stdin", impact: "No Token was stored."},
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
	// As JSON, the fix keeps --json, and it is guidance, since only the
	// operator has a Token that the provider accepts.
	app, out, _, _, httpClient := testApp(t, "new-token\n")
	saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test")
	httpClient.status = http.StatusUnauthorized
	if err := cli.Execute(app, []string{"provider", "rotate", "one", "--token-stdin", "--json"}); err == nil ||
		!strings.Contains(out.String(), `"next_detail": "Pipe a Token that provider one accepts into `+"`aigw provider rotate one --json --token-stdin`"+`."`) ||
		strings.Contains(out.String(), `"next_action"`) {
		t.Fatalf("rotation failure document = %v\n%s", err, out.String())
	}
}

func TestProviderRotateLeavesClientConfigurationOutsideItsScope(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "new-token\n")
	cfg := domain.NewConfig(clients.Admission())
	addProviderModel(&cfg, "one", "One", domain.Endpoints{Responses: "https://one.test/v1"}, codex.ID, "gpt-test")
	cfg.SetSelectedModel(codex.ID, domain.NewModelRef("one", "gpt-test"), "")
	cfg.SetClientActivation(codex.ID, true, "/opt/codex", []string{t.TempDir()})
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("one", "old-token"); err != nil {
		t.Fatal(err)
	}
	err := cli.Execute(app, []string{"provider", "rotate", "one", "--token-stdin"})
	if err != nil {
		t.Fatalf("rotation depended on client configuration: %v", err)
	}
	if token, getErr := secretStore.Get("one"); getErr != nil || token != "new-token" {
		t.Fatalf("rotated token = %q, %v", token, getErr)
	}
}

func TestProviderRotateReportsTokenStorageWithoutNativeClientWrites(t *testing.T) {
	for _, hasTarget := range []bool{true, false} {
		t.Run(fmt.Sprintf("target=%t", hasTarget), func(t *testing.T) {
			app, out, secretStore, runner, _ := testApp(t, "new-token\n")
			target := filepath.Join(t.TempDir(), "configuration.toml")
			if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg := domain.NewConfig(clients.Admission())
			addProviderModel(&cfg, "one", "One", domain.Endpoints{Responses: "https://one.test/v1"}, codex.ID, "gpt-test")
			cfg.SetSelectedModel(codex.ID, domain.NewModelRef("one", "gpt-test"), "")
			cfg.SetClientActivation(codex.ID, true, "/opt/codex", []string{target})
			wantMessage := "a running client decides when that is"
			if !hasTarget {
				cfg.SetClientActivation(codex.ID, true, "/opt/codex", nil)
			}
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := cli.Execute(app, []string{"provider", "rotate", "one", "--token-stdin"}); err != nil {
				t.Fatal(err)
			}
			if len(runner.plans) != 0 || !secretExists(t, secretStore, "one") || !strings.Contains(out.String(), wantMessage) {
				t.Fatalf("binding calls=%d output=%q", len(runner.plans), out.String())
			}
		})
	}
}

func TestProviderRotateValidationUsesCommandContext(t *testing.T) {
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
	command.SetArgs([]string{"provider", "rotate", "one", "--token-stdin"})
	if err := command.ExecuteContext(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("rotation lost command cancellation: %v", err)
	}
	if secretExists(t, store, "one") {
		t.Fatal("cancelled rotation persisted a token")
	}
}

func TestProviderRotateClaudeOnlyProviderDoesNotTouchCodexTargets(t *testing.T) {
	app, _, secretStore, runner, httpClient := testApp(t, "new-claude-token\n")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["codex-account"] = domain.Provider{Label: "Codex", Endpoints: domain.Endpoints{Responses: "https://codex.test/v1"}}
	cfg.Providers["claude-account"] = domain.Provider{Label: "Claude", Endpoints: domain.Endpoints{Anthropic: "https://claude.test"}}
	cfg.Models[domain.NewModelRef("codex-account", "gpt-test")] = qualifiedModel("GPT", "codex-account", "gpt-test", domain.ProtocolResponses)
	cfg.Models[domain.NewModelRef("claude-account", "claude-test")] = qualifiedModel("Claude", "claude-account", "claude-test", domain.ProtocolAnthropic)
	cfg.SetSelectedModel(codex.ID, domain.NewModelRef("codex-account", "gpt-test"), "")
	cfg.SetSelectedModel(claude.ID, domain.NewModelRef("claude-account", "claude-test"), "")
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

	if err := cli.Execute(app, []string{"provider", "rotate", "claude-account", "--token-stdin"}); err != nil {
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

func TestProviderBalanceKeepsDiagnosticCredentialsAtTheirOrigin(t *testing.T) {
	var followed atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		followed.Add(1)
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(target.Close)
	origin := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL+request.URL.RequestURI(), http.StatusFound)
	}))
	t.Cleanup(origin.Close)
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["dmx"] = domain.Provider{
		Label:        "DMXAPI",
		Endpoints:    domain.Endpoints{Responses: origin.URL + "/v1"},
		BalanceProbe: &domain.BalanceProbe{Kind: "dmxapi", BaseURL: origin.URL},
	}
	cfg.Models[domain.NewModelRef("dmx", "gpt-test")] = domain.Model{Label: "GPT", Provider: "dmx", Family: "gpt-test", ID: "gpt-test", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolResponses: {}}}
	cfg.SetSelectedModel(codex.ID, domain.NewModelRef("dmx", "gpt-test"), "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := app.Providers.Set("dmx", secret.DiagnosticCredential{SystemToken: "system", UserID: "user"}); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "sk-abcd-middle-wxyz"); err != nil {
		t.Fatal(err)
	}
	app.HTTP = origin.Client()
	err := cli.Execute(app, []string{"provider", "balance", "dmx"})
	if followed.Load() != 0 || err == nil || !strings.Contains(err.Error()+out.String(), "302") {
		t.Fatalf("balance followed=%d, error=%v; want the origin's redirect refused", followed.Load(), err)
	}
}

func TestProviderAddRefusesAnUnavailableLockBeforeCredentialAccess(t *testing.T) {
	app, _, secretStore, _, _ := testApp(t, "token\n")
	recorder := &recordingCredentialStore[string]{backend: secretStore}
	app.Secrets = recorder
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	app.Config = store.NewStore(filepath.Join(blocker, "nested", "configuration.toml"), clients.Admission())
	err := cli.Execute(app, []string{"provider", "add", "dmx", "--endpoint", "anthropic=https://example.test", "--token-stdin"})
	if err == nil || !strings.Contains(err.Error(), "create config directory for lock") {
		t.Fatalf("lock admission error = %v", err)
	}
	if len(recorder.setCalls)+len(recorder.deleteCalls)+len(recorder.getCalls)+len(recorder.existsCalls) != 0 {
		t.Fatal("lock failure touched credentials")
	}
}
