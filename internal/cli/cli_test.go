package cli_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/account"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/cli"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/config"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/secrets"
)

type fakeRunner struct {
	plans   []adapters.ProcessPlan
	output  []byte
	capture error
}

func (r *fakeRunner) Run(_ context.Context, plan adapters.ProcessPlan) error {
	r.plans = append(r.plans, plan)
	return nil
}

func (r *fakeRunner) RunCapture(_ context.Context, plan adapters.ProcessPlan) ([]byte, error) {
	r.plans = append(r.plans, plan)
	if r.capture != nil {
		return nil, r.capture
	}
	if r.output == nil {
		return []byte("AIGW_OK\n"), nil
	}
	return append([]byte(nil), r.output...), nil
}

type failingRunner struct {
	err       error
	remaining int
}

func (r *failingRunner) Run(_ context.Context, _ adapters.ProcessPlan) error {
	if r.remaining > 0 {
		r.remaining--
		return r.err
	}
	return nil
}

type fakeHTTP struct {
	status  int
	headers http.Header
	body    string
	handler func(*http.Request) (*http.Response, error)
}

func (f *fakeHTTP) Do(req *http.Request) (*http.Response, error) {
	f.headers = req.Header.Clone()
	if f.handler != nil {
		return f.handler(req)
	}
	body := f.body
	if body == "" {
		body = "{}"
	}
	return &http.Response{StatusCode: f.status, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
}

func testApp(t *testing.T, stdin string) (*cli.App, *bytes.Buffer, *secrets.MemoryStore, *fakeRunner) {
	t.Helper()
	out := new(bytes.Buffer)
	secretStore := secrets.NewMemoryStore()
	runner := &fakeRunner{}
	httpClient := &fakeHTTP{status: 200}
	app := &cli.App{
		Config:      config.NewStore(filepath.Join(t.TempDir(), "config.toml")),
		Secrets:     secretStore,
		Accounts:    account.NewMemoryStore(),
		Env:         []string{},
		In:          strings.NewReader(stdin),
		Out:         out,
		Err:         out,
		Interactive: false,
		Runner:      runner,
		HTTP:        httpClient,
	}
	return app, out, secretStore, runner
}

func addAccountProfile(cfg *domain.Config, profileName, accountName, label string, endpoints domain.Endpoints, client string, models domain.Models) {
	if _, exists := cfg.Accounts[accountName]; !exists {
		cfg.Accounts[accountName] = domain.Account{Label: label, Endpoints: endpoints}
	}
	cfg.Profiles[profileName] = domain.Profile{Label: label, Account: accountName, Client: client, Models: models}
}

func execute(t *testing.T, app *cli.App, args ...string) error {
	t.Helper()
	return cli.Execute(app, args)
}

func TestAddWithTokenStdinCreatesProfileWithoutPrintingSecret(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "top-secret\n")
	err := execute(t, app, "add", "dmx", "--label", "DMXAPI", "--openai-url", "https://example.test/v1", "--anthropic-url", "https://example.test", "--token-stdin")
	if err != nil {
		t.Fatal(err)
	}
	if !secretStore.Has("dmx") {
		t.Fatal("secret not stored")
	}
	if strings.Contains(out.String(), "top-secret") {
		t.Fatalf("secret leaked in output: %s", out.String())
	}
	cfg, err := app.Config.Load()
	if err != nil || cfg.Routes.Default != "dmx" || cfg.Profiles["dmx"].Label != "DMXAPI" {
		t.Fatalf("config = %#v, %v", cfg, err)
	}
}

func TestAddRefusesNonInteractiveImplicitTokenInput(t *testing.T) {
	app, _, _, _ := testApp(t, "top-secret\n")
	err := execute(t, app, "add", "dmx", "--label", "DMX", "--anthropic-url", "https://example.test")
	if err == nil || !strings.Contains(err.Error(), "--token-stdin") {
		t.Fatalf("error = %v", err)
	}
}

func TestUseSetsDefaultOrClientOverride(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", domain.Endpoints{Anthropic: "https://one.test"}, "", domain.Models{})
	addAccountProfile(&cfg, "two", "two", "Two", domain.Endpoints{Anthropic: "https://two.test"}, "", domain.Models{})
	cfg.Routes.Default = "one"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "one-secret")
	_ = secretStore.Set("two", "two-secret")
	if err := execute(t, app, "use", "two", "--for", "claude"); err != nil {
		t.Fatal(err)
	}
	got, _ := app.Config.Load()
	if got.Routes.Default != "one" || got.Routes.Overrides["claude"] != "two" {
		t.Fatalf("routes = %#v", got.Routes)
	}
	if err := execute(t, app, "use", "two", "--all"); err != nil {
		t.Fatal(err)
	}
	got, _ = app.Config.Load()
	if got.Routes.Default != "two" || len(got.Routes.Overrides) != 0 {
		t.Fatalf("all routes = %#v", got.Routes)
	}
}

func TestUseForClaudeDoesNotRequireOrRewriteCodexTargets(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	cfg.Accounts["gateway"] = domain.Account{Label: "Gateway", Endpoints: domain.Endpoints{OpenAIResponses: "https://gateway.test/v1", Anthropic: "https://gateway.test"}}
	cfg.Profiles["gpt"] = domain.Profile{Label: "GPT", Account: "gateway", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-test"}}
	cfg.Profiles["claude-fable"] = domain.Profile{Label: "Claude Fable", Account: "gateway", Client: domain.ClientClaude, Models: domain.Models{domain.ClientClaude: "claude-fable"}}
	cfg.Profiles["claude-sonnet"] = domain.Profile{Label: "Claude Sonnet", Account: "gateway", Client: domain.ClientClaude, Models: domain.Models{domain.ClientClaude: "claude-sonnet"}}
	cfg.Routes.Default = "gpt"
	cfg.Routes.Overrides[domain.ClientClaude] = "claude-fable"
	cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Targets: []string{filepath.Join(t.TempDir(), "unavailable-codex-config.toml")}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("gateway", "test-token"); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app, "use", "claude-sonnet", "--for", "claude"); err != nil {
		t.Fatalf("Claude-only route change touched Codex target: %v", err)
	}
	got, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Routes.Default != "gpt" || got.Routes.Overrides[domain.ClientClaude] != "claude-sonnet" {
		t.Fatalf("routes = %#v", got.Routes)
	}
}

func TestUseRollsBackRouteWhenAdapterSyncFails(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	dir := t.TempDir()
	target := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, "", domain.Models{})
	addAccountProfile(&cfg, "two", "two", "Two", domain.Endpoints{OpenAIResponses: "https://two.test/v1"}, "", domain.Models{})
	cfg.Routes.Default = "one"
	cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: "/missing/codex", Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "old-secret")
	_ = secretStore.Set("two", "new-secret")
	app.Runner = &failingRunner{err: errors.New("login failed"), remaining: 1}
	err := execute(t, app, "use", "two")
	if err == nil || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("error = %v", err)
	}
	got, _ := app.Config.Load()
	if got.Routes.Default != "one" {
		t.Fatalf("route was not rolled back: %#v", got.Routes)
	}
}

func TestSyncReconcilesCodexConfigWithoutRebindingCredentials(t *testing.T) {
	app, _, secretStore, runner := testApp(t, "")
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig()
	cfg.Accounts["dmx"] = domain.Account{Label: "DMX", Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1"}}
	cfg.Profiles["gpt"] = domain.Profile{Label: "GPT", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-test"}}
	cfg.Routes.Default = "gpt"
	cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: "/usr/local/bin/codex", Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "test-token"); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app, "sync"); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("sync started credential binding plans: %#v", runner.plans)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `model = "gpt-test" # managed by AIGW`) {
		t.Fatalf("sync did not reconcile Codex config:\n%s", data)
	}
}

func TestUseCodexProfileOnSameAccountDoesNotRebindCredentials(t *testing.T) {
	app, _, secretStore, runner := testApp(t, "")
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig()
	cfg.Accounts["dmx"] = domain.Account{Label: "DMX", Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1"}}
	cfg.Profiles["sol"] = domain.Profile{Label: "Sol", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-5.6-sol-cdx"}}
	cfg.Profiles["terra"] = domain.Profile{Label: "Terra", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-5.6-terra-cdx"}}
	cfg.Routes.Default = "sol"
	cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: "/usr/local/bin/codex", Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "test-token"); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app, "use", "terra", "--for", "codex"); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 0 {
		t.Fatalf("same-account model switch rebound credentials: %#v", runner.plans)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `model = "gpt-5.6-terra-cdx" # managed by AIGW`) {
		t.Fatalf("Codex model was not switched:\n%s", data)
	}
}

func TestRotateRollsBackSecretWhenAdapterSyncFails(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "new-secret\n")
	dir := t.TempDir()
	target := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", domain.Endpoints{OpenAIResponses: "https://one.test/v1"}, "", domain.Models{})
	cfg.Routes.Default = "one"
	cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: "/missing/codex", Targets: []string{target}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("one", "old-secret")
	app.Runner = &failingRunner{err: errors.New("login failed"), remaining: 1}
	err := execute(t, app, "rotate", "one", "--token-stdin")
	if err == nil || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("error = %v", err)
	}
	got, _ := secretStore.Get("one")
	if got != "old-secret" {
		t.Fatalf("secret = %q, want old-secret", got)
	}
}

func TestRotateClaudeOnlyAccountDoesNotTouchCodexTargets(t *testing.T) {
	app, _, secretStore, runner := testApp(t, "new-claude-token\n")
	cfg := domain.NewConfig()
	cfg.Accounts["codex-account"] = domain.Account{Label: "Codex", Endpoints: domain.Endpoints{OpenAIResponses: "https://codex.test/v1"}}
	cfg.Accounts["claude-account"] = domain.Account{Label: "Claude", Endpoints: domain.Endpoints{Anthropic: "https://claude.test"}}
	cfg.Profiles["gpt"] = domain.Profile{Label: "GPT", Account: "codex-account", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-test"}}
	cfg.Profiles["claude"] = domain.Profile{Label: "Claude", Account: "claude-account", Client: domain.ClientClaude, Models: domain.Models{domain.ClientClaude: "claude-test"}}
	cfg.Routes.Default = "gpt"
	cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: "/missing/codex", Targets: []string{filepath.Join(t.TempDir(), "unavailable-codex-config.toml")}}
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
		if req.URL.Host != "claude.test" || req.Header.Get("Authorization") != "Bearer new-claude-token" {
			t.Fatalf("Claude token verification request = %s authorization=%q", req.URL, req.Header.Get("Authorization"))
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

func TestStatusShowsInheritanceAndJSONNeverContainsToken(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	addAccountProfile(&cfg, "dmx", "dmx", "DMX", domain.Endpoints{Anthropic: "https://example.test", OpenAIResponses: "https://example.test/v1"}, "", domain.Models{})
	cfg.Routes.Default = "dmx"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "never-print-this-secret")
	if err := execute(t, app); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Claude") || !strings.Contains(out.String(), "继承默认") {
		t.Fatalf("human status = %s", out.String())
	}
	out.Reset()
	if err := execute(t, app, "status", "--json"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "never-print-this-secret") || !strings.Contains(out.String(), `"secret_available": true`) {
		t.Fatalf("unsafe JSON status = %s", out.String())
	}
}

func TestTestCommandAuthenticatesWithoutPrintingAuthorizationHeader(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	addAccountProfile(&cfg, "dmx", "dmx", "DMX", domain.Endpoints{OpenAIResponses: "https://example.test/v1"}, "", domain.Models{})
	cfg.Routes.Default = "dmx"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "unused-secret")
	if err := execute(t, app, "test", "--for", "codex"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "连接测试") || !strings.Contains(out.String(), "Codex") || !strings.Contains(out.String(), "HTTP 200") {
		t.Fatalf("test output = %s", out.String())
	}
	httpClient := app.HTTP.(*fakeHTTP)
	if httpClient.headers.Get("Authorization") != "Bearer unused-secret" {
		t.Fatalf("authorization header = %q", httpClient.headers.Get("Authorization"))
	}
	if strings.Contains(out.String(), "unused-secret") || strings.Contains(strings.ToLower(out.String()), "authorization") {
		t.Fatalf("credential leaked in output: %s", out.String())
	}
}

func TestVerifyCodexPerformsBoundedResponsesRequest(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	cfg.Accounts["dmx"] = domain.Account{Label: "DMX", Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1"}}
	cfg.Profiles["gpt"] = domain.Profile{Label: "GPT", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-test"}}
	cfg.Routes.Default = "gpt"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "verify-token"); err != nil {
		t.Fatal(err)
	}
	var requestBody string
	app.HTTP = &fakeHTTP{status: 200, handler: func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/responses" {
			t.Fatalf("request = %s %s", req.Method, req.URL)
		}
		data, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		requestBody = string(data)
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"status":"completed","output":[{"content":[{"type":"output_text","text":"AIGW_OK"}]}]}`)), Request: req}, nil
	}}

	if err := execute(t, app, "verify", "--for", "codex"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(requestBody, `"model":"gpt-test"`) || !strings.Contains(requestBody, `"store":false`) || !strings.Contains(requestBody, "AIGW_OK") {
		t.Fatalf("verify body = %s", requestBody)
	}
	if strings.Contains(out.String(), "verify-token") || !strings.Contains(out.String(), "真实协议验证") {
		t.Fatalf("verify output = %s", out.String())
	}
}

func TestVerifyClaudeUsesManagedProcessBoundary(t *testing.T) {
	app, out, secretStore, runner := testApp(t, "")
	shimDir := t.TempDir()
	app.Shims.BinDir = shimDir
	app.Shims.AIGWExecutable = filepath.Join(shimDir, "aigw")
	if _, err := app.Shims.EnableClaude(); err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig()
	cfg.Accounts["dmx"] = domain.Account{Label: "DMX", Endpoints: domain.Endpoints{Anthropic: "https://example.test"}}
	cfg.Profiles["claude-fable-5"] = domain.Profile{Label: "Claude Fable", Account: "dmx", Client: domain.ClientClaude, Models: domain.Models{domain.ClientClaude: "claude-fable-5"}}
	cfg.Routes.Default = "claude-fable-5"
	cfg.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true, Executable: "/opt/claude-real"}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "verify-token"); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app, "verify", "--for", "claude"); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 1 || runner.plans[0].Executable != "/opt/claude-real" || !strings.Contains(strings.Join(runner.plans[0].Args, " "), "AIGW_OK") {
		t.Fatalf("Claude verify plan = %#v", runner.plans)
	}
	if runner.plans[0].Replace {
		t.Fatal("Claude verification must capture a child process instead of replacing AIGW")
	}
	if strings.Contains(out.String(), "verify-token") || !strings.Contains(out.String(), "真实协议验证") {
		t.Fatalf("verify output = %s", out.String())
	}
}

func TestVerifyAllWritesVerifiedCheckpoint(t *testing.T) {
	app, _, secretStore, runner := testApp(t, "")
	shimDir := t.TempDir()
	app.Shims.BinDir = shimDir
	app.Shims.AIGWExecutable = filepath.Join(shimDir, "aigw")
	if _, err := app.Shims.EnableClaude(); err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig()
	cfg.Accounts["dmx"] = domain.Account{Label: "DMX", Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1", Anthropic: "https://example.test"}}
	cfg.Profiles["gpt"] = domain.Profile{Label: "GPT", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-test"}}
	cfg.Profiles["claude"] = domain.Profile{Label: "Claude", Account: "dmx", Client: domain.ClientClaude, Models: domain.Models{domain.ClientClaude: "claude-test"}}
	cfg.Routes.Default = "gpt"
	cfg.Routes.Overrides[domain.ClientClaude] = "claude"
	cfg.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true, Executable: "/opt/claude-real"}
	codexTarget := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(codexTarget, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: "/opt/codex", Targets: []string{codexTarget}}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	codexRuntime, _, err := cfg.ResolveRuntime(domain.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := adapters.SyncCodexConfig(codexTarget, codexRuntime); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "verify-token"); err != nil {
		t.Fatal(err)
	}
	app.HTTP = &fakeHTTP{status: http.StatusOK, body: `{"status":"completed","output_text":"AIGW_OK"}`}

	if err := execute(t, app, "verify", "--for", "all"); err != nil {
		t.Fatal(err)
	}
	if len(runner.plans) != 1 {
		t.Fatalf("Claude verification plans = %#v", runner.plans)
	}
	checkpoint, err := app.Config.LoadVerifiedCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	if len(checkpoint.Clients) != 2 || checkpoint.Config.Routes.Default != "gpt" {
		t.Fatalf("checkpoint = %#v", checkpoint)
	}
}

func TestVerifyAllRequiresSynchronizedClientAdapters(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	shimDir := t.TempDir()
	app.Shims.BinDir = shimDir
	app.Shims.AIGWExecutable = filepath.Join(shimDir, "aigw")
	if _, err := app.Shims.EnableClaude(); err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig()
	cfg.Accounts["dmx"] = domain.Account{Label: "DMX", Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1", Anthropic: "https://example.test"}}
	cfg.Profiles["gpt"] = domain.Profile{Label: "GPT", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-test"}}
	cfg.Profiles["claude"] = domain.Profile{Label: "Claude", Account: "dmx", Client: domain.ClientClaude, Models: domain.Models{domain.ClientClaude: "claude-test"}}
	cfg.Routes.Default = "gpt"
	cfg.Routes.Overrides[domain.ClientClaude] = "claude"
	cfg.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true, Executable: "/opt/claude-real"}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "verify-token"); err != nil {
		t.Fatal(err)
	}
	err := execute(t, app, "verify", "--for", "all")
	if err == nil || !strings.Contains(err.Error(), "requires an enabled Codex adapter") {
		t.Fatalf("error = %v", err)
	}
	if _, checkpointErr := app.Config.LoadVerifiedCheckpoint(); checkpointErr == nil {
		t.Fatal("verification checkpoint was written despite failed readiness preflight")
	}
}

func TestVerifyRejectsMissingResponseSentinel(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	cfg.Accounts["dmx"] = domain.Account{Label: "DMX", Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1"}}
	cfg.Profiles["gpt"] = domain.Profile{Label: "GPT", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-test"}}
	cfg.Routes.Default = "gpt"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "verify-token"); err != nil {
		t.Fatal(err)
	}
	app.HTTP = &fakeHTTP{status: http.StatusOK, body: `{"status":"completed","output_text":"not-the-sentinel"}`}
	err := execute(t, app, "verify", "--for", "codex")
	if err == nil || !strings.Contains(err.Error(), "required verification sentinel") {
		t.Fatalf("error = %v", err)
	}
}

func TestVerifyClaudeRejectsMissingResponseSentinel(t *testing.T) {
	app, _, secretStore, runner := testApp(t, "")
	shimDir := t.TempDir()
	app.Shims.BinDir = shimDir
	app.Shims.AIGWExecutable = filepath.Join(shimDir, "aigw")
	if _, err := app.Shims.EnableClaude(); err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig()
	cfg.Accounts["dmx"] = domain.Account{Label: "DMX", Endpoints: domain.Endpoints{Anthropic: "https://example.test"}}
	cfg.Profiles["claude"] = domain.Profile{Label: "Claude", Account: "dmx", Client: domain.ClientClaude, Models: domain.Models{domain.ClientClaude: "claude-test"}}
	cfg.Routes.Default = "claude"
	cfg.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true, Executable: "/opt/claude-real"}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "verify-token"); err != nil {
		t.Fatal(err)
	}
	runner.output = []byte("wrong response\n")
	err := execute(t, app, "verify", "--for", "claude")
	if err == nil || !strings.Contains(err.Error(), "required verification sentinel") {
		t.Fatalf("error = %v", err)
	}
}

func TestRollbackRestoresVerifiedCheckpointBeforeLastChangeBackup(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	verified := domain.NewConfig()
	verified.Accounts["dmx"] = domain.Account{Label: "DMX", Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1"}}
	verified.Profiles["stable"] = domain.Profile{Label: "Stable", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-stable"}}
	verified.Routes.Default = "stable"
	if err := app.Config.Save(verified); err != nil {
		t.Fatal(err)
	}
	if err := app.Config.SaveVerifiedCheckpoint(verified, []string{domain.ClientCodex}); err != nil {
		t.Fatal(err)
	}
	current := verified
	current.Profiles = map[string]domain.Profile{"experimental": {Label: "Experimental", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-experimental"}}}
	current.Routes.Default = "experimental"
	if err := app.Config.Save(current); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "token"); err != nil {
		t.Fatal(err)
	}

	if err := execute(t, app, "rollback"); err != nil {
		t.Fatal(err)
	}
	restored, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if restored.Routes.Default != "stable" {
		t.Fatalf("rollback default = %q, want stable", restored.Routes.Default)
	}
}

func TestTestCommandRejectsAuthenticationFailure(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	addAccountProfile(&cfg, "dmx", "dmx", "DMX", domain.Endpoints{Anthropic: "https://example.test"}, "", domain.Models{})
	cfg.Routes.Default = "dmx"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "rejected-secret")
	app.HTTP.(*fakeHTTP).status = 401
	err := execute(t, app, "test", "--for", "claude")
	if err == nil || !strings.Contains(err.Error(), "authentication rejected") || strings.Contains(err.Error(), "rejected-secret") {
		t.Fatalf("error = %v", err)
	}
}

func TestTestCommandUsesAccountTokenForRuntimeProfile(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	cfg.Accounts["dmx"] = domain.Account{Label: "DMXAPI", Endpoints: domain.Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Profiles["gpt-5.6-sol-cdx"] = domain.Profile{Label: "GPT", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-5.6-sol-cdx"}}
	cfg.Routes.Default = "gpt-5.6-sol-cdx"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "account-token")
	if err := execute(t, app, "test", "--for", "codex"); err != nil {
		t.Fatal(err)
	}
	if app.HTTP.(*fakeHTTP).headers.Get("Authorization") != "Bearer account-token" {
		t.Fatalf("authorization header = %q", app.HTTP.(*fakeHTTP).headers.Get("Authorization"))
	}
	if strings.Contains(out.String(), "account-token") || !strings.Contains(out.String(), "gpt-5.6-sol-cdx") {
		t.Fatalf("test output = %s", out.String())
	}
}

func TestTestCommandUsesCodexModelsEndpointAndRejectsNotFound(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	cfg.Accounts["dmx"] = domain.Account{Label: "DMXAPI", Endpoints: domain.Endpoints{OpenAIResponses: "https://dmx.test/v1"}}
	cfg.Profiles["gpt-5.6-sol-cdx"] = domain.Profile{Label: "GPT", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-5.6-sol-cdx"}}
	cfg.Routes.Default = "gpt-5.6-sol-cdx"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	_ = secretStore.Set("dmx", "account-token")
	var gotPath string
	app.HTTP.(*fakeHTTP).handler = func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":[]}`)), Request: req}, nil
	}
	if err := execute(t, app, "test", "--for", "codex"); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v1/models" {
		t.Fatalf("codex test path = %q", gotPath)
	}

	app.HTTP.(*fakeHTTP).handler = func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader(`{"message":"not found"}`)), Request: req}, nil
	}
	err := execute(t, app, "test", "--for", "codex")
	if err == nil || !strings.Contains(err.Error(), "HTTP 404") {
		t.Fatalf("error = %v", err)
	}
}
