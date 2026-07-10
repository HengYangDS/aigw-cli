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

type fakeRunner struct{ plans []adapters.ProcessPlan }

func (r *fakeRunner) Run(_ context.Context, plan adapters.ProcessPlan) error {
	r.plans = append(r.plans, plan)
	return nil
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
		In:          strings.NewReader(stdin),
		Out:         out,
		Err:         out,
		Interactive: false,
		Runner:      runner,
		HTTP:        httpClient,
	}
	return app, out, secretStore, runner
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
	cfg.Profiles["one"] = domain.Profile{Label: "One", Endpoints: domain.Endpoints{Anthropic: "https://one.test"}}
	cfg.Profiles["two"] = domain.Profile{Label: "Two", Endpoints: domain.Endpoints{Anthropic: "https://two.test"}}
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

func TestUseRollsBackRouteWhenAdapterSyncFails(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	dir := t.TempDir()
	target := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig()
	cfg.Profiles["one"] = domain.Profile{Label: "One", Endpoints: domain.Endpoints{OpenAIResponses: "https://one.test/v1"}}
	cfg.Profiles["two"] = domain.Profile{Label: "Two", Endpoints: domain.Endpoints{OpenAIResponses: "https://two.test/v1"}}
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

func TestRotateRollsBackSecretWhenAdapterSyncFails(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "new-secret\n")
	dir := t.TempDir()
	target := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig()
	cfg.Profiles["one"] = domain.Profile{Label: "One", Endpoints: domain.Endpoints{OpenAIResponses: "https://one.test/v1"}}
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

func TestStatusShowsInheritanceAndJSONNeverContainsToken(t *testing.T) {
	app, out, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	cfg.Profiles["dmx"] = domain.Profile{Label: "DMX", Endpoints: domain.Endpoints{Anthropic: "https://example.test", OpenAIResponses: "https://example.test/v1"}}
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
	cfg.Profiles["dmx"] = domain.Profile{Label: "DMX", Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1"}}
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

func TestTestCommandRejectsAuthenticationFailure(t *testing.T) {
	app, _, secretStore, _ := testApp(t, "")
	cfg := domain.NewConfig()
	cfg.Profiles["dmx"] = domain.Profile{Label: "DMX", Endpoints: domain.Endpoints{Anthropic: "https://example.test"}}
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
