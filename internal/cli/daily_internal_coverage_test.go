package cli

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
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/config"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/discovery"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/secrets"
)

type dailyRunOnly struct{ err error }

func (runner dailyRunOnly) Run(context.Context, adapters.ProcessPlan) error { return runner.err }

type dailyCaptureOutput struct {
	output []byte
	err    error
}

func (runner dailyCaptureOutput) Run(context.Context, adapters.ProcessPlan) error { return runner.err }
func (runner dailyCaptureOutput) RunCapture(context.Context, adapters.ProcessPlan) ([]byte, error) {
	return runner.output, runner.err
}

func dailyCoverageConfig() domain.Config {
	cfg := domain.NewConfig()
	cfg.Accounts["one"] = domain.Account{Label: "One", Endpoints: domain.Endpoints{OpenAIResponses: "http://127.0.0.1:1234/v1", Anthropic: "https://one.test"}}
	cfg.Profiles["one"] = domain.Profile{Label: "One", Purpose: "Coverage", Account: "one", Models: domain.Models{domain.ClientCodex: "gpt", domain.ClientClaude: "claude"}}
	cfg.Routes.Default = "one"
	return cfg
}

func dailyCoverageApp(t *testing.T, cfg domain.Config) *App {
	t.Helper()
	store := config.NewStore(filepath.Join(t.TempDir(), "config.toml"))
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	return &App{Config: store, Secrets: secrets.NewMemoryStore(), Accounts: account.NewMemoryStore(), Discovery: reconciliationDiscovery{}, Out: out, Err: out}
}

func TestStatusHumanTransportAndProbeBranches(t *testing.T) {
	tests := []struct {
		name       string
		probe      *domain.AccountProbe
		credential bool
		want       string
	}{
		{name: "none", want: "Provider does not expose a probe"},
		{name: "unsupported", probe: &domain.AccountProbe{Kind: "future", BaseURL: "https://probe.test"}, want: "does not provide diagnostics"},
		{name: "supported missing", probe: &domain.AccountProbe{Kind: "dmxapi", BaseURL: "https://probe.test"}, want: "Disabled"},
		{name: "supported present", probe: &domain.AccountProbe{Kind: "dmxapi", BaseURL: "https://probe.test"}, credential: true, want: "Enabled"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := dailyCoverageConfig()
			providerAccount := cfg.Accounts["one"]
			providerAccount.AccountProbe = test.probe
			cfg.Accounts["one"] = providerAccount
			app := dailyCoverageApp(t, cfg)
			_ = app.Secrets.Set("one", "token")
			if test.credential {
				_ = app.Accounts.Set("one", account.Credential{SystemToken: "system", UserID: "user"})
			}
			if err := runStatus(nil, app, false); err != nil {
				t.Fatal(err)
			}
			text := app.Out.(*bytes.Buffer).String()
			if !strings.Contains(text, test.want) || !strings.Contains(text, "External loopback compatibility layer") {
				t.Fatalf("output = %q", text)
			}
		})
	}
}

func TestDailyRouteAndAdapterHelpers(t *testing.T) {
	if transportStatus("%").Kind != "" {
		t.Fatal("invalid URL has transport state")
	}
	if got := codexModelsEndpoint("https://one.test/models/"); got != "https://one.test/models" {
		t.Fatalf("models endpoint = %q", got)
	}

	cfg := dailyCoverageConfig()
	runtime, _, err := cfg.ResolveRuntime(domain.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	app := &App{}
	cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true}
	if ready, issue := adapterRouteReady(app, cfg, domain.ClientCodex, runtime); ready || !strings.Contains(issue, "executable") {
		t.Fatalf("ready=%v issue=%q", ready, issue)
	}
	cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: "codex"}
	if ready, issue := adapterRouteReady(app, cfg, domain.ClientCodex, runtime); ready || !strings.Contains(issue, "target") {
		t.Fatalf("ready=%v issue=%q", ready, issue)
	}
	cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: "codex", Targets: []string{filepath.Join(t.TempDir(), "missing.toml")}}
	if ready, issue := adapterRouteReady(app, cfg, domain.ClientCodex, runtime); ready || !strings.Contains(issue, "drift") {
		t.Fatalf("ready=%v issue=%q", ready, issue)
	}

	claudeRuntime, _, _ := cfg.ResolveRuntime(domain.ClientClaude, "")
	cfg.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true, Executable: "claude"}
	app.Shims = unreadableDailyShim(t)
	if ready, issue := adapterRouteReady(app, cfg, domain.ClientClaude, claudeRuntime); ready || !strings.Contains(issue, "Cannot read Claude shim") {
		t.Fatalf("ready=%v issue=%q", ready, issue)
	}
	app.Shims = missingDailyShim(t)
	if ready, issue := adapterRouteReady(app, cfg, domain.ClientClaude, claudeRuntime); ready || !strings.Contains(issue, "shim is missing") {
		t.Fatalf("ready=%v issue=%q", ready, issue)
	}
	assertDailyClaudeActivationBehavior(t, app, cfg, claudeRuntime)

	profiles := domain.NewConfig()
	profiles.Accounts["one"] = domain.Account{Label: "One", Endpoints: domain.Endpoints{Anthropic: "https://one.test"}}
	profiles.Profiles["skip"] = domain.Profile{Account: "one", Client: domain.ClientClaude, Models: domain.Models{domain.ClientClaude: "claude"}}
	profiles.Profiles["generic"] = domain.Profile{Account: "one"}
	if got := firstProfileForClient(profiles, domain.ClientCodex); got != "" {
		t.Fatalf("unexpected Codex profile %q", got)
	}
	if got := firstProfileForClient(profiles, domain.ClientClaude); got != "generic" {
		t.Fatalf("Claude profile = %q", got)
	}
}

func TestFullVerificationReadinessFailures(t *testing.T) {
	base := dailyCoverageConfig()
	app := &App{}
	if err := validateFullVerificationReadiness(app, base); err == nil || !strings.Contains(err.Error(), "enabled Claude") {
		t.Fatalf("error = %v", err)
	}

	base.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true, Executable: "claude"}
	app.Shims = unreadableDailyShim(t)
	if err := validateFullVerificationReadiness(app, base); err == nil || !strings.Contains(err.Error(), "read Claude launcher") {
		t.Fatalf("error = %v", err)
	}

	app.Shims = missingDailyShim(t)
	if err := validateFullVerificationReadiness(app, base); err == nil || !strings.Contains(err.Error(), "managed Claude launcher") {
		t.Fatalf("error = %v", err)
	}

	app.Shims = readyDailyShim(t)
	if err := validateFullVerificationReadiness(app, base); err == nil || !strings.Contains(err.Error(), "enabled Codex") {
		t.Fatalf("error = %v", err)
	}

	base.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: "codex", Targets: []string{filepath.Join(t.TempDir(), "missing.toml")}}
	if err := validateFullVerificationReadiness(app, base); err == nil || !strings.Contains(err.Error(), "synchronized Codex") {
		t.Fatalf("error = %v", err)
	}
}

func TestVerifyCodexResponseFailuresAndSentinelShapes(t *testing.T) {
	runtime := domain.Runtime{ProfileID: "one", AccountID: "one", Endpoint: "https://one.test/v1", Model: "gpt"}
	if err := verifyCodexResponse(context.Background(), &App{}, domain.Runtime{ProfileID: "one"}, "token"); err == nil || !strings.Contains(err.Error(), "no Codex model") {
		t.Fatalf("error = %v", err)
	}
	runtime.Endpoint = "://bad"
	if err := verifyCodexResponse(context.Background(), &App{}, runtime, "token"); err == nil {
		t.Fatal("expected URL error")
	}
	runtime.Endpoint = "https://one.test/v1"
	want := errors.New("network failed")
	app := &App{HTTP: setupCoverageHTTP(func(*http.Request) (*http.Response, error) { return nil, want })}
	if err := verifyCodexResponse(context.Background(), app, runtime, "token"); !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}

	tests := []struct {
		name   string
		status int
		body   io.ReadCloser
		want   string
	}{
		{name: "read", status: 200, body: setupCoverageBody{Reader: errorReader{err: want}}, want: "read Codex"},
		{name: "oversized", status: 200, body: io.NopCloser(strings.NewReader(strings.Repeat("x", verificationResponseLimit+1))), want: "exceeds"},
		{name: "authentication", status: 401, body: io.NopCloser(strings.NewReader(`{}`)), want: "authentication"},
		{name: "server", status: 500, body: io.NopCloser(strings.NewReader(`{}`)), want: "HTTP 500"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := &App{HTTP: setupCoverageHTTP(func(request *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: test.status, Body: test.body, Request: request}, nil
			})}
			if err := verifyCodexResponse(context.Background(), app, runtime, "token"); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v", err)
			}
		})
	}
	if hasVerificationSentinel([]byte("not-json")) || hasVerificationSentinel([]byte(`{"status":"failed","output_text":"AIGW_OK"}`)) {
		t.Fatal("invalid response accepted")
	}
	if !hasVerificationSentinel([]byte(`{"output":[{"content":[{"type":"text","text":" AIGW_OK "}]}]}`)) {
		t.Fatal("nested verification marker rejected")
	}
}

func TestVerifyClaudeFailureBranches(t *testing.T) {
	cfg := dailyCoverageConfig()
	runtime, _, _ := cfg.ResolveRuntime(domain.ClientClaude, "")
	if err := verifyClaudeInvocation(context.Background(), &App{}, cfg, runtime, "token"); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("error = %v", err)
	}
	cfg.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true, Executable: "claude"}
	if err := verifyClaudeInvocation(context.Background(), &App{Shims: missingDailyShim(t)}, cfg, runtime, "token"); err == nil || !strings.Contains(err.Error(), "launcher is missing") {
		t.Fatalf("error = %v", err)
	}
	if err := verifyClaudeRuntimeWithExecutable(context.Background(), &App{}, "/claude", domain.Runtime{ProfileID: "one"}, "token"); err == nil || !strings.Contains(err.Error(), "no Claude model") {
		t.Fatalf("error = %v", err)
	}
	if err := verifyClaudeRuntimeWithExecutable(context.Background(), &App{Runner: dailyRunOnly{}}, "/claude", runtime, "token"); err == nil || !strings.Contains(err.Error(), "runner is unavailable") {
		t.Fatalf("error = %v", err)
	}
	want := errors.New("capture failed")
	if err := verifyClaudeRuntimeWithExecutable(context.Background(), &App{Runner: dailyCaptureOutput{err: want}}, "/claude", runtime, "token"); !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
	if err := verifyClaudeRuntimeWithExecutable(context.Background(), &App{Runner: dailyCaptureOutput{output: []byte("wrong")}}, "/claude", runtime, "token"); err == nil || !strings.Contains(err.Error(), "expected AIGW_OK") {
		t.Fatalf("error = %v", err)
	}
}

func TestCodexReconciliationAndAuthenticationInputFailures(t *testing.T) {
	base := dailyCoverageConfig()
	base.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: "/codex", Targets: []string{"/target"}}
	if _, _, _, err := codexReconciliationInputs(&App{}, base, base); err == nil {
		t.Fatal("expected discovery error")
	}
	app := &App{Discovery: reconciliationDiscovery{}}
	before := cloneConfig(base)
	before.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Targets: []string{""}}
	if _, _, _, err := codexReconciliationInputs(app, before, base); err == nil {
		t.Fatal("expected before target error")
	}
	after := cloneConfig(base)
	after.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Targets: []string{""}}
	if _, _, _, err := codexReconciliationInputs(app, base, after); err == nil {
		t.Fatal("expected after target error")
	}
	badAir := filepath.Join(t.TempDir(), "config.toml")
	if err := os.Mkdir(badAir+".aigw-state.json", 0o700); err != nil {
		t.Fatal(err)
	}
	discovered := discovery.Result{Surfaces: []discovery.Surface{{ID: discovery.SurfaceAirCodex, Authority: discovery.AuthorityJetBrainsAI, ConfigPath: badAir, Present: true}}}
	if _, err := codexTargetRefs(discovered, []string{badAir}, false); err == nil {
		t.Fatal("expected Air projection identity read error")
	}

	if err := bindCodexAuthenticationTargets(context.Background(), &App{}, dailyCoverageConfig(), nil); err == nil || !strings.Contains(err.Error(), "enabled adapter") {
		t.Fatalf("error = %v", err)
	}
	missingExecutable := cloneConfig(base)
	missingExecutable.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true}
	if err := bindCodexAuthenticationTargets(context.Background(), &App{}, missingExecutable, nil); err == nil || !strings.Contains(err.Error(), "executable") {
		t.Fatalf("error = %v", err)
	}
	app = &App{Runner: dailyRunOnly{}, Secrets: secrets.NewMemoryStore()}
	if err := bindCodexAuthenticationTargets(context.Background(), app, base, nil); err == nil || !strings.Contains(err.Error(), "Token") {
		t.Fatalf("error = %v", err)
	}

	invalid := domain.NewConfig()
	invalid.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true}
	if _, ok := codexRouteAccount(invalid); ok {
		t.Fatal("invalid route returned an account")
	}
	if !codexProjectionChanged(base, invalid) {
		t.Fatal("invalid projection was not detected as changed")
	}
}
