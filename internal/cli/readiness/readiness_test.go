package readiness

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/account"
	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"

	"github.com/spf13/cobra"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) Do(request *http.Request) (*http.Response, error) { return fn(request) }

type failingReader struct{ err error }

func (reader failingReader) Read([]byte) (int, error) { return 0, reader.err }

type closeErrorBody struct{ io.Reader }

func (closeErrorBody) Close() error { return errors.New("close failed") }

type readinessProcessRunner struct {
	plans []process.Plan
	err   error
}

func (runner *readinessProcessRunner) Run(context.Context, process.Plan) error { return nil }

func (runner *readinessProcessRunner) RunCapture(_ context.Context, plan process.Plan) ([]byte, error) {
	runner.plans = append(runner.plans, plan)
	return nil, runner.err
}

func configuredReadinessRuntime(t *testing.T) (invocation.Context, configuration.Config) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "configuration.toml")
	store := configuration.NewStore(configPath)
	cfg := configuration.NewConfig()
	cfg.Accounts["one"] = configuration.Account{
		Label: "One",
		Endpoints: configuration.Endpoints{
			Anthropic:       "https://claude.example.test",
			OpenAIResponses: "https://codex.example.test/v1",
		},
	}
	cfg.Profiles["one"] = configuration.Profile{
		Label:   "One",
		Account: "one",
		Models: configuration.Models{
			configuration.ClientClaude: "claude-test",
			configuration.ClientCodex:  "gpt-test",
		},
	}
	cfg.Routes.Default = "one"
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	runtime := invocation.Context{
		Config: store, Secrets: secrets.NewMemoryStore(), Accounts: account.NewMemoryStore(),
		Out: out, RenderOut: out, Width: 120,
	}
	return runtime, cfg
}

func output(runtime invocation.Context) string { return runtime.Out.(*bytes.Buffer).String() }

func executeCommand(command *cobra.Command) error {
	command.SetErr(io.Discard)
	command.SilenceErrors = true
	command.SilenceUsage = true
	return command.Execute()
}

func TestRunStatusCoversSelectionDiagnosticsAndReadyNextActions(t *testing.T) {
	runtime, cfg := configuredReadinessRuntime(t)
	originalProbe := probeAdapterRoute
	originalAuthentication := probeCodexAuthentication
	t.Cleanup(func() {
		probeAdapterRoute = originalProbe
		probeCodexAuthentication = originalAuthentication
	})
	probeAdapterRoute = func(invocation.Context, configuration.Config, string, configuration.Runtime) (bool, string) {
		return true, ""
	}
	probeCodexAuthentication = func(invocation.Context, string, []string) bool { return true }
	if err := runtime.Secrets.Set("one", "token"); err != nil {
		t.Fatal(err)
	}
	if err := RunStatus(runtime, false); err != nil {
		t.Fatal(err)
	}
	if got := output(runtime); !strings.Contains(got, "aigw check") {
		t.Fatalf("ready status = %q", got)
	}

	accountConfig := cfg.Accounts["one"]
	accountConfig.AccountProbe = &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://probe.example.test"}
	cfg.Accounts["one"] = accountConfig
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	runtime.Out.(*bytes.Buffer).Reset()
	if err := RunStatus(runtime, false); err != nil {
		t.Fatal(err)
	}
	if got := output(runtime); !strings.Contains(got, "aigw account connect one") {
		t.Fatalf("missing diagnostic credential status = %q", got)
	}
	if err := runtime.Accounts.Set("one", account.Credential{SystemToken: "system", UserID: "user"}); err != nil {
		t.Fatal(err)
	}
	runtime.Out.(*bytes.Buffer).Reset()
	if err := RunStatus(runtime, false); err != nil {
		t.Fatal(err)
	}
	if got := output(runtime); !strings.Contains(got, "Precise balance") || !strings.Contains(got, "Enabled") {
		t.Fatalf("enabled diagnostic status = %q", got)
	}

	cfg.Profiles["codex-only"] = configuration.Profile{Label: "Codex only", Purpose: "Selection", Account: "one", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-test"}}
	cfg.Profiles["one"] = configuration.Profile{Label: "Claude only", Account: "one", Client: configuration.ClientClaude, Models: configuration.Models{configuration.ClientClaude: "claude-test"}}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	runtime.Out.(*bytes.Buffer).Reset()
	if err := RunStatus(runtime, false); err != nil {
		t.Fatal(err)
	}
	if got := output(runtime); !strings.Contains(got, "aigw use codex-only --for codex") {
		t.Fatalf("route selection status = %q", got)
	}
}

func TestRunStatusJSONNotConfiguredAndLoadErrors(t *testing.T) {
	emptyPath := filepath.Join(t.TempDir(), "configuration.toml")
	emptyOut := &bytes.Buffer{}
	emptyRuntime := invocation.Context{Config: configuration.NewStore(emptyPath), Out: emptyOut, RenderOut: emptyOut, Secrets: secrets.NewMemoryStore(), Accounts: account.NewMemoryStore()}
	if err := RunStatus(emptyRuntime, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(emptyOut.String(), "aigw setup") {
		t.Fatalf("empty status = %q", emptyOut.String())
	}
	emptyOut.Reset()
	if err := RunStatus(emptyRuntime, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(emptyOut.String(), `"profiles": 0`) {
		t.Fatalf("json status = %q", emptyOut.String())
	}

	badPath := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(badPath, []byte("not = [toml"), 0o600); err != nil {
		t.Fatal(err)
	}
	badRuntime := invocation.Context{Config: configuration.NewStore(badPath), Out: io.Discard}
	if err := RunStatus(badRuntime, false); err == nil {
		t.Fatal("malformed configuration was accepted")
	}
}

func TestAdapterRouteReadyCoversClaudeExecutableAndCodexOutcomes(t *testing.T) {
	runtime, cfg := configuredReadinessRuntime(t)
	clientRuntime, _, err := cfg.ResolveRuntime(configuration.ClientClaude, "")
	if err != nil {
		t.Fatal(err)
	}
	if ready, issue := AdapterRouteReady(runtime, cfg, configuration.ClientClaude, clientRuntime); ready || !strings.Contains(issue, "disabled") {
		t.Fatalf("disabled adapter ready=%v issue=%q", ready, issue)
	}
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true}
	if ready, issue := AdapterRouteReady(runtime, cfg, configuration.ClientClaude, clientRuntime); ready || !strings.Contains(issue, "executable") {
		t.Fatalf("missing executable ready=%v issue=%q", ready, issue)
	}

	executable := filepath.Join(t.TempDir(), "claude")
	if goruntime.GOOS == "windows" {
		executable += ".exe"
	}
	if err := os.WriteFile(executable, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: filepath.Join(t.TempDir(), "missing")}
	if ready, issue := AdapterRouteReady(runtime, cfg, configuration.ClientClaude, clientRuntime); ready || issue != "Claude executable is unavailable" {
		t.Fatalf("missing executable ready=%v issue=%q", ready, issue)
	}
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "\x00"}
	if ready, issue := AdapterRouteReady(runtime, cfg, configuration.ClientClaude, clientRuntime); ready || issue != "Cannot inspect Claude executable" {
		t.Fatalf("invalid executable ready=%v issue=%q", ready, issue)
	}
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executable}
	if ready, issue := AdapterRouteReady(runtime, cfg, configuration.ClientClaude, clientRuntime); !ready || issue != "" {
		t.Fatalf("ready executable ready=%v issue=%q", ready, issue)
	}

	codexRuntime, _, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "codex"}
	if ready, issue := AdapterRouteReady(runtime, cfg, configuration.ClientCodex, codexRuntime); ready || !strings.Contains(issue, "target") {
		t.Fatalf("missing Codex target ready=%v issue=%q", ready, issue)
	}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "codex", Targets: []string{filepath.Join(t.TempDir(), "missing.toml")}}
	if ready, issue := AdapterRouteReady(runtime, cfg, configuration.ClientCodex, codexRuntime); ready || !strings.Contains(issue, "projection drift") {
		t.Fatalf("drifted Codex target ready=%v issue=%q", ready, issue)
	}
	originalValidate := validateCodexConfig
	t.Cleanup(func() { validateCodexConfig = originalValidate })
	validateCodexConfig = func(string, configuration.Runtime) error { return nil }
	if ready, issue := AdapterRouteReady(runtime, cfg, configuration.ClientCodex, codexRuntime); !ready || issue != "" {
		t.Fatalf("ready Codex target ready=%v issue=%q", ready, issue)
	}
}

func TestCodexAuthenticationProofUsesThePublicStatusCommandForEveryTarget(t *testing.T) {
	runner := &readinessProcessRunner{}
	targets := []string{
		filepath.Join(t.TempDir(), "first", "config.toml"),
		filepath.Join(t.TempDir(), "second", "config.toml"),
	}
	runtime := invocation.Context{Runner: runner}
	if !CodexAuthenticationProven(runtime, "codex", targets) {
		t.Fatal("successful native status probes did not prove authentication")
	}
	if len(runner.plans) != len(targets) {
		t.Fatalf("status plans = %d, want %d", len(runner.plans), len(targets))
	}
	for index, plan := range runner.plans {
		if !slices.Equal(plan.Args, []string{"login", "status"}) {
			t.Fatalf("status args = %#v", plan.Args)
		}
		wantHome := filepath.Dir(targets[index])
		if !slices.Contains(plan.Env, "CODEX_HOME="+wantHome) {
			t.Fatalf("status environment does not select %q: %#v", wantHome, plan.Env)
		}
		if plan.Stdin != "" {
			t.Fatalf("status probe received secret input: %#v", plan)
		}
	}

	runner.err = errors.New("not logged in")
	if CodexAuthenticationProven(runtime, "codex", targets[:1]) {
		t.Fatal("failed native status probe proved authentication")
	}
	if CodexAuthenticationProven(runtime, "codex", nil) {
		t.Fatal("an empty target set proved authentication")
	}
	if CodexAuthenticationProven(runtime, "", targets[:1]) {
		t.Fatal("an empty executable proved authentication")
	}
	if CodexAuthenticationProven(invocation.Context{}, "codex", targets[:1]) {
		t.Fatal("missing capture capability proved authentication")
	}
}

func TestStatusSeparatesCodexProjectionFromNativeAuthentication(t *testing.T) {
	runtime, cfg := configuredReadinessRuntime(t)
	if err := runtime.Secrets.Set("one", "token"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "config.toml")
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{
		Enabled: true, Executable: "codex", Targets: []string{target},
	}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	originalValidate := validateCodexConfig
	originalAuthentication := probeCodexAuthentication
	originalAdapter := probeAdapterRoute
	t.Cleanup(func() {
		validateCodexConfig = originalValidate
		probeCodexAuthentication = originalAuthentication
		probeAdapterRoute = originalAdapter
	})
	validateCodexConfig = func(string, configuration.Runtime) error { return nil }
	probeAdapterRoute = func(invocation.Context, configuration.Config, string, configuration.Runtime) (bool, string) {
		return true, ""
	}
	probeCodexAuthentication = func(invocation.Context, string, []string) bool { return false }

	if err := RunStatus(runtime, false); err != nil {
		t.Fatal(err)
	}
	got := output(runtime)
	if !strings.Contains(got, "Projection ready") || !strings.Contains(got, "Native authentication not proven") {
		t.Fatalf("status does not separate readiness facts: %q", got)
	}
	if !strings.Contains(got, "aigw adapter auth codex") || strings.Contains(got, "aigw repair") {
		t.Fatalf("status next action = %q", got)
	}

	runtime.Out.(*bytes.Buffer).Reset()
	if err := RunStatus(runtime, true); err != nil {
		t.Fatal(err)
	}
	jsonStatus := output(runtime)
	if !strings.Contains(jsonStatus, `"adapter_ready": true`) ||
		!strings.Contains(jsonStatus, `"native_authentication": "not_proven"`) {
		t.Fatalf("JSON readiness facts = %q", jsonStatus)
	}

	probeCodexAuthentication = func(invocation.Context, string, []string) bool { return true }
	runtime.Out.(*bytes.Buffer).Reset()
	if err := RunStatus(runtime, false); err != nil {
		t.Fatal(err)
	}
	if got := output(runtime); !strings.Contains(got, "Ready") || !strings.Contains(got, "aigw check") {
		t.Fatalf("proved authentication status = %q", got)
	}
	runtime.Out.(*bytes.Buffer).Reset()
	if err := RunStatus(runtime, true); err != nil {
		t.Fatal(err)
	}
	if got := output(runtime); !strings.Contains(got, `"native_authentication": "present"`) {
		t.Fatalf("proved native authentication status = %q", got)
	}

	profile := cfg.Profiles["one"]
	profile.Client = configuration.ClientCodex
	delete(profile.Models, configuration.ClientClaude)
	profile.ModelProvider = "amazon-bedrock"
	cfg.Profiles["one"] = profile
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	probeCodexAuthentication = func(invocation.Context, string, []string) bool {
		t.Fatal("native Codex provider must not require AIGW authentication")
		return false
	}
	runtime.Out.(*bytes.Buffer).Reset()
	if err := RunStatus(runtime, true); err != nil {
		t.Fatal(err)
	}
	if got := output(runtime); !strings.Contains(got, `"native_authentication": "not_required"`) {
		t.Fatalf("native provider authentication status = %q", got)
	}
}

func TestRunCheckCoversClientResolutionAndProjectionFailures(t *testing.T) {
	t.Run("enabled client route mismatch", func(t *testing.T) {
		runtime, cfg := configuredReadinessRuntime(t)
		runtime.Version = "1.0.0"
		runtime.Problem = func(title, _, _, _ string, err error) error {
			return fmt.Errorf("%s: %w", title, err)
		}
		if err := runtime.Secrets.Set("one", "token"); err != nil {
			t.Fatal(err)
		}
		profile := cfg.Profiles["one"]
		profile.Client = configuration.ClientCodex
		delete(profile.Models, configuration.ClientClaude)
		cfg.Profiles["one"] = profile
		cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "/opt/claude"}
		if err := runtime.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		if err := RunCheck(&cobra.Command{}, runtime); err == nil || !strings.Contains(err.Error(), "route cannot be resolved") {
			t.Fatalf("RunCheck() error = %v", err)
		}
	})

	t.Run("Codex projection drift", func(t *testing.T) {
		runtime, cfg := configuredReadinessRuntime(t)
		runtime.Version = "1.0.0"
		if err := runtime.Secrets.Set("one", "token"); err != nil {
			t.Fatal(err)
		}
		profile := cfg.Profiles["one"]
		profile.Client = configuration.ClientCodex
		delete(profile.Models, configuration.ClientClaude)
		cfg.Profiles["one"] = profile
		cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{
			Enabled:    true,
			Executable: "/opt/codex",
			Targets:    []string{filepath.Join(t.TempDir(), "missing.toml")},
		}
		if err := runtime.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		if err := RunCheck(&cobra.Command{}, runtime); err == nil || !strings.Contains(err.Error(), "adapter not ready") {
			t.Fatalf("RunCheck() error = %v", err)
		}
	})
}

func TestRunCheckShowsEnabledProviderDiagnostics(t *testing.T) {
	runtime, cfg := configuredReadinessRuntime(t)
	runtime.Version = "1.0.0"
	if err := runtime.Secrets.Set("one", "token"); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Accounts.Set("one", account.Credential{SystemToken: "system", UserID: "user"}); err != nil {
		t.Fatal(err)
	}
	providerAccount := cfg.Accounts["one"]
	providerAccount.AccountProbe = &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://probe.example.test"}
	cfg.Accounts["one"] = providerAccount
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	runtime.HTTP = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Request: request}, nil
	})
	command := &cobra.Command{}
	command.SetContext(context.Background())
	if err := RunCheck(command, runtime); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Precise balance", "Enabled", "aigw balance one"} {
		if !strings.Contains(output(runtime), want) {
			t.Fatalf("health output lacks %q: %s", want, output(runtime))
		}
	}
}

func TestEndpointTestCommandCoversSuccessAndTransportFailures(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		runtime, _ := configuredReadinessRuntime(t)
		if err := runtime.Secrets.Set("one", "token"); err != nil {
			t.Fatal(err)
		}
		requests := 0
		runtime.HTTP = roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests++
			if strings.Contains(request.URL.Host, "claude") && request.Header.Get("X-Api-Key") != "token" {
				t.Fatalf("Claude credential header = %q", request.Header.Get("X-Api-Key"))
			}
			if strings.Contains(request.URL.Host, "codex") && request.Header.Get("Authorization") != "Bearer token" {
				t.Fatalf("Codex credential header = %q", request.Header.Get("Authorization"))
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok"))}, nil
		})
		command := NewTestCommand(runtime)
		if err := executeCommand(command); err != nil {
			t.Fatal(err)
		}
		if requests != 2 || !strings.Contains(output(runtime), "Connectivity test") {
			t.Fatalf("requests=%d output=%q", requests, output(runtime))
		}
	})

	tests := []struct {
		name         string
		status       int
		body         io.ReadCloser
		roundTripErr error
		want         string
	}{
		{name: "network", roundTripErr: errors.New("offline"), want: "unreachable"},
		{name: "authentication", status: http.StatusUnauthorized, body: io.NopCloser(strings.NewReader("denied")), want: "authentication was rejected"},
		{name: "unexpected status", status: http.StatusBadGateway, body: io.NopCloser(strings.NewReader("failed")), want: "HTTP 502"},
		{name: "read", status: http.StatusOK, body: io.NopCloser(failingReader{err: errors.New("read failed")}), want: "read Codex endpoint response"},
		{name: "close", status: http.StatusOK, body: closeErrorBody{Reader: strings.NewReader("ok")}, want: "close Codex endpoint response"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runtime, _ := configuredReadinessRuntime(t)
			if err := runtime.Secrets.Set("one", "token"); err != nil {
				t.Fatal(err)
			}
			runtime.HTTP = roundTripFunc(func(*http.Request) (*http.Response, error) {
				if test.roundTripErr != nil {
					return nil, test.roundTripErr
				}
				return &http.Response{StatusCode: test.status, Body: test.body}, nil
			})
			command := NewTestCommand(runtime)
			command.SetArgs([]string{"--for", "codex"})
			if err := executeCommand(command); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestEndpointTestCommandCoversInputAndResolutionFailures(t *testing.T) {
	t.Run("profile infers client", func(t *testing.T) {
		runtime, cfg := configuredReadinessRuntime(t)
		profile := cfg.Profiles["one"]
		profile.Client = configuration.ClientCodex
		delete(profile.Models, configuration.ClientClaude)
		cfg.Profiles["one"] = profile
		if err := runtime.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		if err := runtime.Secrets.Set("one", "token"); err != nil {
			t.Fatal(err)
		}
		var requests int
		runtime.HTTP = roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests++
			if request.URL.Path != "/v1/models" {
				t.Fatalf("request path = %q", request.URL.Path)
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}"))}, nil
		})
		command := NewTestCommand(runtime)
		command.SetArgs([]string{"--profile", "one"})
		if err := executeCommand(command); err != nil {
			t.Fatal(err)
		}
		if requests != 1 {
			t.Fatalf("requests = %d, want 1", requests)
		}
	})

	t.Run("unscoped profile requires client", func(t *testing.T) {
		runtime, _ := configuredReadinessRuntime(t)
		command := NewTestCommand(runtime)
		command.SetArgs([]string{"--profile", "one"})
		if err := executeCommand(command); err == nil || !strings.Contains(err.Error(), "--for") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("invalid client", func(t *testing.T) {
		runtime, _ := configuredReadinessRuntime(t)
		command := NewTestCommand(runtime)
		command.SetArgs([]string{"--for", "future"})
		if err := executeCommand(command); err == nil || !strings.Contains(err.Error(), "--for must be") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("not configured", func(t *testing.T) {
		problem := errors.New("structured problem")
		runtime := invocation.Context{Config: configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml")), Out: io.Discard, Problem: func(string, string, string, string, error) error { return problem }}
		if err := executeCommand(NewTestCommand(runtime)); !errors.Is(err, problem) {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("unknown profile", func(t *testing.T) {
		runtime, _ := configuredReadinessRuntime(t)
		command := NewTestCommand(runtime)
		command.SetArgs([]string{"--profile", "missing"})
		if err := executeCommand(command); err == nil || !strings.Contains(err.Error(), "unknown profile") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("missing token", func(t *testing.T) {
		runtime, _ := configuredReadinessRuntime(t)
		command := NewTestCommand(runtime)
		command.SetArgs([]string{"--for", "codex"})
		if err := executeCommand(command); err == nil || !strings.Contains(err.Error(), "Token for account") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("missing explicit endpoint", func(t *testing.T) {
		runtime, cfg := configuredReadinessRuntime(t)
		accountConfig := cfg.Accounts["one"]
		accountConfig.Endpoints.OpenAIResponses = ""
		cfg.Accounts["one"] = accountConfig
		if err := runtime.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		command := NewTestCommand(runtime)
		command.SetArgs([]string{"--for", "codex"})
		if err := executeCommand(command); err == nil || !strings.Contains(err.Error(), "no OpenAI Responses endpoint") {
			t.Fatalf("error = %v", err)
		}
	})

}

func TestReadinessSmallHelpers(t *testing.T) {
	for configuredClients, want := range map[int]string{
		0: "The current API route is unavailable.",
		1: "The configured AI client is unavailable.",
		2: "2 configured AI clients are unavailable.",
	} {
		if got := healthImpact(configuredClients); got != want {
			t.Fatalf("healthImpact(%d) = %q, want %q", configuredClients, got, want)
		}
	}
	if _, err := firstCheckRuntime(configuration.NewConfig()); err == nil || !strings.Contains(err.Error(), "no testable endpoint") {
		t.Fatalf("empty default route error = %v", err)
	}
	missingAccount := configuration.NewConfig()
	missingAccount.Profiles["missing"] = configuration.Profile{Account: "absent", Client: configuration.ClientCodex}
	missingAccount.Routes.Default = "missing"
	if _, err := firstCheckRuntime(missingAccount); err == nil || !strings.Contains(err.Error(), "no testable endpoint") {
		t.Fatalf("unresolvable default route error = %v", err)
	}
	if got := TransportStatus("%"); got.Kind != "" {
		t.Fatalf("invalid transport = %#v", got)
	}
	if got := TransportStatus("http://LOCALHOST:8791/v1"); got.Kind != "external_loopback" {
		t.Fatalf("loopback transport = %#v", got)
	}
	if got := TransportStatus("https://api.example.test/v1"); got.Kind != "" {
		t.Fatalf("remote transport = %#v", got)
	}
	if got := CodexModelsEndpoint("https://api.example.test/v1/"); got != "https://api.example.test/v1/models" {
		t.Fatalf("models endpoint = %q", got)
	}
	if got := CodexModelsEndpoint("https://api.example.test/models/"); got != "https://api.example.test/models" {
		t.Fatalf("existing models endpoint = %q", got)
	}
	request, err := http.NewRequest(http.MethodGet, "https://example.test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := authenticateRequest(request, configuration.ClientSpec{ID: "future", EndpointProtocol: configuration.EndpointProtocol("future")}, "token"); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("unsupported protocol error = %v", err)
	}
	buffer := &bytes.Buffer{}
	renderer := Renderer(invocation.Context{Out: buffer})
	renderer.Text("fallback writer")
	if !strings.Contains(buffer.String(), "fallback writer") {
		t.Fatalf("renderer output = %q", buffer.String())
	}
	command := NewStatusCommand(invocation.Context{Config: configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml")), Out: io.Discard, Secrets: secrets.NewMemoryStore(), Accounts: account.NewMemoryStore()})
	command.SetArgs([]string{"--json"})
	if err := executeCommand(command); err != nil {
		t.Fatal(err)
	}
}
