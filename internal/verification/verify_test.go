package verification

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/process"
)

type httpDoer func(*http.Request) (*http.Response, error)

func (do httpDoer) Do(request *http.Request) (*http.Response, error) { return do(request) }

type responseBody struct {
	io.Reader
	closeErr error
}

func (body responseBody) Close() error { return body.closeErr }

type basicRunner struct{ err error }

func (runner basicRunner) Run(context.Context, process.Plan) error { return runner.err }

type captureRunner struct {
	output []byte
	err    error
}

func (runner captureRunner) Run(context.Context, process.Plan) error { return runner.err }
func (runner captureRunner) RunCapture(context.Context, process.Plan) ([]byte, error) {
	return runner.output, runner.err
}

type failingReader struct{ err error }

func (reader failingReader) Read([]byte) (int, error) { return 0, reader.err }

func verificationConfig() configuration.Config {
	cfg := configuration.NewConfig()
	cfg.Accounts["one"] = configuration.Account{Label: "One", Endpoints: configuration.Endpoints{
		OpenAIResponses: "https://one.test/v1",
		Anthropic:       "https://one.test",
	}}
	cfg.Profiles["codex"] = configuration.Profile{Label: "Codex", Account: "one", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-test"}}
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "one", Client: configuration.ClientClaude, Models: configuration.Models{configuration.ClientClaude: "claude-test"}}
	cfg.Routes.Default = "codex"
	cfg.Routes.Overrides[configuration.ClientClaude] = "claude"
	return cfg
}

func TestValidateFullReadiness(t *testing.T) {
	cfg := verificationConfig()
	if err := ValidateFullReadiness(cfg); err == nil || !strings.Contains(err.Error(), "enabled Claude") {
		t.Fatalf("disabled Claude error = %v", err)
	}
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: filepath.Join(t.TempDir(), "missing")}
	if err := ValidateFullReadiness(cfg); err == nil || !strings.Contains(err.Error(), "available Claude") {
		t.Fatalf("missing executable error = %v", err)
	}
	claudeExecutable := filepath.Join(t.TempDir(), "claude")
	if goruntime.GOOS == "windows" {
		claudeExecutable += ".exe"
	}
	if err := os.WriteFile(claudeExecutable, []byte("fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: claudeExecutable}
	if err := ValidateFullReadiness(cfg); err == nil || !strings.Contains(err.Error(), "enabled Codex") {
		t.Fatalf("disabled Codex error = %v", err)
	}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "codex", Targets: []string{filepath.Join(t.TempDir(), "missing.toml")}}
	if err := ValidateFullReadiness(cfg); err == nil || !strings.Contains(err.Error(), "synchronized Codex") {
		t.Fatalf("drift error = %v", err)
	}
	target := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	codexRuntime, _, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := codex.SyncConfig(target, codexRuntime); err != nil {
		t.Fatal(err)
	}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "codex", Targets: []string{target}}
	if err := ValidateFullReadiness(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestValidateFullReadinessReportsInspectionAndRouteErrors(t *testing.T) {
	cfg := verificationConfig()
	loop := filepath.Join(t.TempDir(), "claude")
	if err := os.Symlink(loop, loop); err != nil {
		t.Fatal(err)
	}
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: loop}
	if err := ValidateFullReadiness(cfg); err == nil || !strings.Contains(err.Error(), "inspect Claude executable") {
		t.Fatalf("Claude inspection error = %v", err)
	}

	claudeExecutable := filepath.Join(t.TempDir(), "claude")
	if goruntime.GOOS == "windows" {
		claudeExecutable += ".exe"
	}
	if err := os.WriteFile(claudeExecutable, []byte("fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: claudeExecutable}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "codex", Targets: []string{"unused"}}
	cfg.Routes.Default = "missing"
	delete(cfg.Routes.Overrides, configuration.ClientCodex)
	if err := ValidateFullReadiness(cfg); err == nil || !strings.Contains(err.Error(), "resolve the Codex route") {
		t.Fatalf("Codex route error = %v", err)
	}
}

func TestVerifyCodexResponse(t *testing.T) {
	runtime := configuration.Runtime{ProfileID: "one", AccountID: "one", Endpoint: "https://one.test/v1", Model: "gpt"}
	if err := VerifyCodexResponse(context.Background(), nil, configuration.Runtime{ProfileID: "one"}, "token"); err == nil || !strings.Contains(err.Error(), "no Codex model") {
		t.Fatalf("model error = %v", err)
	}
	runtime.Endpoint = "://bad"
	if err := VerifyCodexResponse(context.Background(), nil, runtime, "token"); err == nil {
		t.Fatal("invalid URL accepted")
	}
	runtime.Endpoint = "https://one.test/v1"
	want := errors.New("network failed")
	if err := VerifyCodexResponse(context.Background(), httpDoer(func(*http.Request) (*http.Response, error) { return nil, want }), runtime, "token"); !errors.Is(err, want) {
		t.Fatalf("network error = %v", err)
	}

	tests := []struct {
		name   string
		status int
		body   io.ReadCloser
		want   string
	}{
		{name: "read", status: 200, body: responseBody{Reader: failingReader{err: want}}, want: "read Codex"},
		{name: "oversized", status: 200, body: io.NopCloser(strings.NewReader(strings.Repeat("x", responseLimit+1))), want: "exceeds"},
		{name: "authentication", status: 401, body: io.NopCloser(strings.NewReader(`{}`)), want: "authentication"},
		{name: "server", status: 500, body: io.NopCloser(strings.NewReader(`{}`)), want: "HTTP 500"},
		{name: "sentinel", status: 200, body: io.NopCloser(strings.NewReader(`{}`)), want: "expected AIGW_OK"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doer := httpDoer(func(request *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: test.status, Body: test.body, Request: request}, nil
			})
			if err := VerifyCodexResponse(context.Background(), doer, runtime, "token"); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
	var request *http.Request
	doer := httpDoer(func(candidate *http.Request) (*http.Response, error) {
		request = candidate
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"status":"completed","output_text":"AIGW_OK"}`)), Request: candidate}, nil
	})
	if err := VerifyCodexResponse(context.Background(), doer, runtime, "token"); err != nil {
		t.Fatal(err)
	}
	if request.URL.Path != "/v1/responses" || request.Header.Get("Authorization") != "Bearer token" {
		t.Fatalf("request = %#v", request)
	}
}

func TestHasResponseSentinel(t *testing.T) {
	for _, data := range [][]byte{
		[]byte("not-json"),
		[]byte(`{"status":"failed","output_text":"AIGW_OK"}`),
		[]byte(`{"status":"completed","output_text":"wrong"}`),
	} {
		if HasResponseSentinel(data) {
			t.Fatalf("invalid response accepted: %s", data)
		}
	}
	for _, data := range [][]byte{
		[]byte(`{"output_text":" AIGW_OK "}`),
		[]byte(`{"output":[{"content":[{"type":"text","text":" AIGW_OK "}]}]}`),
	} {
		if !HasResponseSentinel(data) {
			t.Fatalf("valid response rejected: %s", data)
		}
	}
}

func TestVerifyClaude(t *testing.T) {
	cfg := verificationConfig()
	runtime, _, err := cfg.ResolveRuntime(configuration.ClientClaude, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyClaudeInvocation(context.Background(), nil, cfg, runtime, "token"); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("disabled error = %v", err)
	}
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: filepath.Join(t.TempDir(), "missing")}
	if err := VerifyClaudeInvocation(context.Background(), nil, cfg, runtime, "token"); err == nil || !strings.Contains(err.Error(), "executable is unavailable") {
		t.Fatalf("missing executable error = %v", err)
	}
	loop := filepath.Join(t.TempDir(), "claude")
	if err := os.Symlink(loop, loop); err != nil {
		t.Fatal(err)
	}
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: loop}
	if err := VerifyClaudeInvocation(context.Background(), nil, cfg, runtime, "token"); err == nil || !strings.Contains(err.Error(), "inspect Claude executable") {
		t.Fatalf("inspection error = %v", err)
	}
	want := errors.New("launcher failed")
	if err := VerifyClaudeRuntime(context.Background(), nil, "claude", configuration.Runtime{ProfileID: "one"}, "token"); err == nil || !strings.Contains(err.Error(), "no Claude model") {
		t.Fatalf("model error = %v", err)
	}
	if err := VerifyClaudeRuntime(context.Background(), basicRunner{}, "claude", runtime, "token"); err == nil || !strings.Contains(err.Error(), "runner is unavailable") {
		t.Fatalf("runner error = %v", err)
	}
	if err := VerifyClaudeRuntime(context.Background(), captureRunner{err: want}, "claude", runtime, "token"); !errors.Is(err, want) {
		t.Fatalf("capture error = %v", err)
	}
	if err := VerifyClaudeRuntime(context.Background(), captureRunner{output: []byte("wrong")}, "claude", runtime, "token"); err == nil || !strings.Contains(err.Error(), "expected AIGW_OK") {
		t.Fatalf("sentinel error = %v", err)
	}
	executable := filepath.Join(t.TempDir(), "claude")
	if goruntime.GOOS == "windows" {
		executable += ".exe"
	}
	if err := os.WriteFile(executable, []byte("fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executable}
	if err := VerifyClaudeInvocation(context.Background(), captureRunner{output: []byte(" AIGW_OK \n")}, cfg, runtime, "token"); err != nil {
		t.Fatal(err)
	}
}
