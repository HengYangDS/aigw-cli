package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform/process"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
)

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// hostAt observes home as the user's home directory, so Claude Code's settings
// file is home/.claude/settings.json.
func hostAt(home string) discovery.Host {
	return discovery.Host{GOOS: runtime.GOOS, Home: home, Env: map[string]string{"HOME": home, "USERPROFILE": home}}
}

// claudeSettingsPath returns the Claude Code settings file on host.
func claudeSettingsPath(t testing.TB, host discovery.Host) string {
	t.Helper()
	path, err := claude.SettingsPath(host)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func writeFile(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}

type fakeRunner struct {
	challenge string

	plans   []process.Plan
	output  []byte
	stderr  []byte
	capture error
}

func (r *fakeRunner) RunCaptureStreams(ctx context.Context, plan process.Plan) ([]byte, []byte, error) {
	output, err := r.RunCapture(ctx, plan)
	if slices.Equal(plan.Args, []string{"--version"}) {
		return output, nil, err
	}
	if err != nil {
		return nil, output, err
	}
	return output, r.stderr, err
}

func (r *fakeRunner) RunCapture(_ context.Context, plan process.Plan) ([]byte, error) {
	r.plans = append(r.plans, plan)
	if len(plan.Args) == 1 && plan.Args[0] == "--version" {
		return []byte("codex-cli 0.0.0-test\n"), nil
	}
	if r.capture != nil {
		return append([]byte(nil), r.output...), r.capture
	}
	outputPath := planArgumentValue(plan.Args, "--output-last-message")
	if outputPath == "" {
		if r.output == nil {
			if planArgumentValue(plan.Args, "--output-format") == "stream-json" {
				return []byte("{\"type\":\"result\",\"is_error\":false,\"result\":\"AIGW_OK\"}\n"), nil
			}
			return []byte("AIGW_OK\n"), nil
		}
		return append([]byte(nil), r.output...), nil
	}
	if !slices.Contains(plan.Args, "resume") {
		challenge, err := os.ReadFile(filepath.Join(filepath.Dir(outputPath), "challenge.txt"))
		if err != nil {
			return nil, err
		}
		r.challenge = strings.TrimSpace(string(challenge))
	}
	output := r.output
	if output == nil {
		output = []byte(r.challenge + "\n")
	}
	if err := os.WriteFile(outputPath, output, 0o600); err != nil {
		return nil, err
	}
	events := []map[string]any{{"type": "thread.started", "thread_id": "00000000-0000-4000-8000-000000000001"}}
	if !slices.Contains(plan.Args, "resume") {
		events = append(events, map[string]any{"type": "item.completed", "item": map[string]any{
			"type": "command_execution", "status": "completed", "command": "cat challenge.txt", "exit_code": 0, "aggregated_output": r.challenge,
		}})
	}
	events = append(events, map[string]any{"type": "item.completed", "item": map[string]any{"type": "agent_message", "text": strings.TrimSpace(string(output))}}, map[string]any{"type": "turn.completed"})
	var captured bytes.Buffer
	for _, event := range events {
		if err := json.NewEncoder(&captured).Encode(event); err != nil {
			return nil, err
		}
	}
	return captured.Bytes(), nil
}

func planArgumentValue(arguments []string, name string) string {
	for index, argument := range arguments {
		if argument == name && index+1 < len(arguments) {
			return arguments[index+1]
		}
	}
	return ""
}

func planEnvironmentValue(environment []string, name string) string {
	prefix := name + "="
	for _, value := range environment {
		if after, ok := strings.CutPrefix(value, prefix); ok {
			return after
		}
	}
	return ""
}

func secretExists(t testing.TB, store secret.Store, account string) bool {
	t.Helper()
	present, err := store.Exists(account)
	if err != nil {
		t.Fatalf("observe credential for %q: %v", account, err)
	}
	return present
}

func accountCredentialExists(t testing.TB, store secret.DiagnosticCredentialStore, accountID string) bool {
	t.Helper()
	present, err := store.Exists(accountID)
	if err != nil {
		t.Fatalf("observe provider diagnostic credential for %q: %v", accountID, err)
	}
	return present
}

func assertSameExistingPath(t *testing.T, got, want string) {
	t.Helper()
	gotInfo, err := os.Stat(got)
	if err != nil {
		t.Fatalf("inspect rendered path %q: %v", got, err)
	}
	wantInfo, err := os.Stat(want)
	if err != nil {
		t.Fatalf("inspect expected path %q: %v", want, err)
	}
	if !os.SameFile(gotInfo, wantInfo) {
		t.Fatalf("paths identify different files: got %q, want %q", got, want)
	}
}

type fakeHTTP struct {
	calls   int
	status  int
	headers http.Header
	body    string
	handler func(*http.Request) (*http.Response, error)
}

type failingOutput struct{ err error }

func (w failingOutput) Write([]byte) (int, error) { return 0, w.err }

type failingReadCloser struct{ err error }

func (body failingReadCloser) Read([]byte) (int, error) { return 0, body.err }

func (body failingReadCloser) Close() error { return nil }

func (f *fakeHTTP) Do(req *http.Request) (*http.Response, error) {
	f.calls++
	f.headers = req.Header.Clone()
	if f.handler != nil {
		return f.handler(req)
	}
	body := f.body
	if body == "" {
		body = fixtureResponseBody(req)
	}
	return &http.Response{StatusCode: f.status, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
}

func fixtureResponseBody(req *http.Request) string {
	switch {
	case req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/responses"):
		return `{"status":"completed","output":[{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"pong"}]}]}`
	case req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/messages"):
		return `{"type":"message","role":"assistant","content":[{"type":"text","text":"pong"}],"stop_reason":"end_turn"}`
	case req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/chat/completions"):
		return `{"choices":[{"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}]}`
	default:
		return "{}"
	}
}

func testApp(t *testing.T, stdin string) (*cli.App, *bytes.Buffer, secret.Store, *fakeRunner, *fakeHTTP) {
	t.Helper()
	out := new(bytes.Buffer)
	secretStore := secret.NewMemoryStore()
	diagnostics, err := secret.NewDiagnosticCredentialStore(secretStore)
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}
	httpClient := &fakeHTTP{status: 200}
	app := &cli.App{Clients: clients.Registry(),
		Version:       "0.1.0-test",
		Executable:    filepath.Join(t.TempDir(), "aigw"),
		InstallTarget: filepath.Join(t.TempDir(), "bin", "aigw"),
		Host:          hostAt(t.TempDir()),
		Config:        store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission()),
		Secrets:       secretStore,
		Accounts:      diagnostics,
		Env:           []string{},
		In:            strings.NewReader(stdin),
		Out:           out,
		Err:           out,
		Interactive:   false,
		Runner:        runner,
		HTTP:          httpClient,
		Discovery:     fakeDiscovery{},
	}
	return app, out, secretStore, runner, httpClient
}

func addAccountRoute(cfg *domain.Config, routeName, accountName, label string, endpoints domain.Endpoints, client, model string) {
	if _, exists := cfg.Accounts[accountName]; !exists {
		cfg.Accounts[accountName] = domain.Account{Label: label, Endpoints: endpoints}
	}
	account := cfg.Accounts[accountName]
	account.ID = accountName
	spec, admitted := clients.Admission().Spec(client)
	if !admitted {
		panic("test fixture uses an unadmitted client: " + client)
	}
	_, protocol, err := spec.ResolveEndpoint(account, "")
	if err != nil {
		panic("test fixture cannot resolve a wire interface: " + err.Error())
	}
	cfg.Routes[routeName] = qualifiedRoute(label, accountName, model, protocol)
}

func qualifiedRoute(label, account, model string, protocol domain.EndpointProtocol) domain.Route {
	return domain.Route{
		Label: label, Account: account, Model: model, UpstreamModel: model,
		Interfaces: map[domain.EndpointProtocol][]domain.Capability{protocol: {}},
	}
}

func synchronizeClaudeProjection(t *testing.T, app *cli.App, cfg domain.Config) {
	t.Helper()
	runtime, err := cfg.ResolveRuntime(claude.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := claude.ReconcileSettings(claudeSettingsPath(t, app.Host), false, runtime, app.Executable, runtime.Model); err != nil {
		t.Fatal(err)
	}
}

func executableFixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), executableName(name))
	if err := os.WriteFile(path, []byte("native client fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func executableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func saveCommandRoute(t *testing.T, app *cli.App, endpoints domain.Endpoints, client, model string) {
	t.Helper()
	cfg := domain.NewConfig(clients.Admission())
	addAccountRoute(&cfg, "one", "one", "One", endpoints, client, model)
	cfg.SetSelectedRoute(client, "one", "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
}

func saveProbeRoute(t *testing.T, appConfig store.Store) {
	t.Helper()
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["dmx"] = domain.Account{
		Label:        "DMXAPI",
		Endpoints:    domain.Endpoints{OpenAIResponses: "https://dmx.test/v1"},
		AccountProbe: &domain.AccountProbe{Kind: "dmxapi", BaseURL: "https://www.dmxapi.cn"},
	}
	cfg.Routes["gpt"] = domain.Route{Label: "GPT", Account: "dmx", Model: "gpt-test", Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	cfg.SetSelectedRoute(codex.ID, "gpt", "")
	if err := appConfig.Save(cfg); err != nil {
		t.Fatal(err)
	}
}

func twoRouteConfig() domain.Config {
	cfg := domain.NewConfig(clients.Admission())
	addAccountRoute(&cfg, "one", "one", "One Gateway", domain.Endpoints{Anthropic: "https://one.test", OpenAIResponses: "https://one.test/v1"}, codex.ID, "model-one")
	addAccountRoute(&cfg, "two", "two", "Two Gateway", domain.Endpoints{Anthropic: "https://two.test", OpenAIResponses: "https://two.test/v1"}, codex.ID, "model-two")
	for _, routeID := range []string{"one", "two"} {
		route := cfg.Routes[routeID]
		route.Interfaces[domain.ProtocolAnthropic] = []domain.Capability{}
		cfg.Routes[routeID] = route
	}
	cfg.SetSelectedRoute(codex.ID, "one", "")
	return cfg
}

func directoryNames(t *testing.T, path string) []string {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

const configurationManifestFixture = `version = 7
[recommendations.claude.primary]
route = "aihubmix-claude"

[[recommendations.claude.alternatives]]
route = "dmxapi-claude"

[recommendations.codex.primary]
route = "dmxapi-gpt"

[accounts.aihubmix]
label = "AIHubMix"
[accounts.aihubmix.endpoints]
openai_responses = "https://aihubmix.test/v1"
anthropic = "https://aihubmix.test"

[accounts.dmxapi]
label = "DMXAPI"
[accounts.dmxapi.endpoints]
openai_responses = "https://dmxapi.test/v1"
anthropic = "https://dmxapi.test"

[models.claude-test]
label = "Claude Test"

[models.gpt-test]
label = "GPT Test"

[routes.aihubmix-claude]
label = "AIHubMix Claude"
account = "aihubmix"
model = "claude-test"
upstream_model = "claude-test"
interfaces = { anthropic = [] }

[routes.dmxapi-claude]
label = "DMXAPI Claude"
account = "dmxapi"
model = "claude-test"
upstream_model = "claude-test"
interfaces = { anthropic = [] }

[routes.dmxapi-gpt]
label = "DMXAPI GPT"
account = "dmxapi"
model = "gpt-test"
upstream_model = "gpt-test"
interfaces = { openai_responses = [] }
`

func writeConfigurationManifest(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

type fakeDiscovery struct{ result discovery.Result }

func (d fakeDiscovery) Discover() discovery.Result { return d.result }

func readyVerificationApp(t *testing.T) (*cli.App, *fakeRunner) {
	t.Helper()
	app, _, secretStore, runner, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["dmx"] = domain.Account{Label: "DMX", Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1", Anthropic: "https://example.test"}}
	cfg.Routes["gpt"] = qualifiedRoute("GPT", "dmx", "gpt-test", domain.ProtocolOpenAIResponses)
	cfg.Routes["claude"] = qualifiedRoute("Claude", "dmx", "claude-test", domain.ProtocolAnthropic)
	cfg.SetSelectedRoute(codex.ID, "gpt", "")
	cfg.SetSelectedRoute(claude.ID, "claude", "")
	cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
	codexTarget := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(codexTarget, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.SetClientActivation(codex.ID, true, executableFixture(t, "codex"), []string{codexTarget})
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("dmx", "verify-token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"sync"}); err != nil {
		t.Fatal(err)
	}
	app.HTTP = &fakeHTTP{status: http.StatusOK, handler: func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request to %s", req.URL)
		return nil, nil
	}}
	return app, runner
}

type verificationCompletionRunner struct {
	*fakeRunner
	completed func()
}

func (runner verificationCompletionRunner) RunCapture(ctx context.Context, plan process.Plan) ([]byte, error) {
	output, err := runner.fakeRunner.RunCapture(ctx, plan)
	if err == nil && slices.Contains(plan.Args, "--output-last-message") {
		runner.completed()
	}
	return output, err
}

func (runner verificationCompletionRunner) RunCaptureStreams(ctx context.Context, plan process.Plan) ([]byte, []byte, error) {
	output, err := runner.RunCapture(ctx, plan)
	return output, nil, err
}
