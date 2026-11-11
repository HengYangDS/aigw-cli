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
	"sync"
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
	"aigw-cli/internal/terminal"
)

// verifiedPath is the verified snapshot beside the configuration at path:
// aigw.verified.json beside aigw.toml.
func verifiedPath(path string) string {
	return strings.TrimSuffix(path, filepath.Ext(path)) + ".verified.json"
}

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

func secretExists(t testing.TB, store secret.Store, account string) bool {
	t.Helper()
	present, err := store.Exists(account)
	if err != nil {
		t.Fatalf("observe credential for %q: %v", account, err)
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

// fakeHTTP answers requests, which catalog reads send at once, through its
// handler or with its status and body, and records them.
type fakeHTTP struct {
	mu      sync.Mutex
	calls   int
	status  int
	headers http.Header
	body    string
	handler func(*http.Request) (*http.Response, error)
}

type failingOutput struct{ err error }

func (w failingOutput) Write([]byte) (int, error) { return 0, w.err }

func (f *fakeHTTP) Do(req *http.Request) (*http.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
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
		Providers:     diagnostics,
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

// addProviderModel adds a provider with endpoints, unless cfg has it already,
// and its model on the protocol that client resolves there, and returns the
// model.
func addProviderModel(cfg *domain.Config, accountName, label string, endpoints domain.Endpoints, client, model string) domain.ModelRef {
	if _, exists := cfg.Providers[accountName]; !exists {
		cfg.Providers[accountName] = domain.Provider{Label: label, Endpoints: endpoints}
	}
	account := cfg.Providers[accountName]
	account.ID = accountName
	spec, admitted := clients.Admission().Spec(client)
	if !admitted {
		panic("test fixture uses an unadmitted client: " + client)
	}
	_, protocol, err := spec.ResolveEndpoint(account, "")
	if err != nil {
		panic("test fixture cannot resolve a wire interface: " + err.Error())
	}
	ref := domain.NewModelRef(accountName, model)
	cfg.Models[ref] = qualifiedModel(label, accountName, model, protocol)
	return ref
}

func qualifiedModel(label, account, model string, protocol domain.EndpointProtocol) domain.Model {
	return domain.Model{
		Label: label, Provider: account, Family: model, ID: model,
		Protocols: map[domain.EndpointProtocol][]domain.Capability{protocol: {}},
	}
}

func synchronizeClaudeProjection(t *testing.T, app *cli.App, cfg domain.Config) {
	t.Helper()
	runtime, err := cfg.ResolveRuntime(claude.ID, domain.ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := claude.ReconcileSettings(claudeSettingsPath(t, app.Host), false, runtime, app.Executable, runtime.UpstreamModel); err != nil {
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

// wantNext reports whether a document's next step is want: a command that a
// script can run as it stands is next_action, and anything else next_detail.
func wantNext(action, detail, want string) bool {
	wantAction, wantDetail := terminal.NextSteps(want)
	return action == wantAction && detail == wantDetail
}

func executableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// blockRemoval makes removing path fail until the test ends: Windows refuses to
// delete an open file, and POSIX refuses to unlink a file from a directory
// without write permission, unless the process runs as root.
func blockRemoval(t *testing.T, path string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = file.Close() })
		return
	}
	if os.Geteuid() == 0 {
		t.Skip("root removes files from a read-only directory")
	}
	directory := filepath.Dir(path)
	if err := os.Chmod(directory, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(directory, 0o700) })
}

func saveCommandModel(t *testing.T, app *cli.App, endpoints domain.Endpoints, client, model string) {
	t.Helper()
	cfg := domain.NewConfig(clients.Admission())
	ref := addProviderModel(&cfg, "one", "One", endpoints, client, model)
	cfg.SetSelectedModel(client, ref, "")
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
}

func twoModelConfig() domain.Config {
	cfg := domain.NewConfig(clients.Admission())
	one := addProviderModel(&cfg, "one", "One Gateway", domain.Endpoints{Anthropic: "https://one.test", Responses: "https://one.test/v1"}, codex.ID, "model-one")
	two := addProviderModel(&cfg, "two", "Two Gateway", domain.Endpoints{Anthropic: "https://two.test", Responses: "https://two.test/v1"}, codex.ID, "model-two")
	for _, ref := range []domain.ModelRef{one, two} {
		model := cfg.Models[ref]
		model.Protocols[domain.ProtocolAnthropic] = []domain.Capability{}
		cfg.Models[ref] = model
	}
	cfg.SetSelectedModel(codex.ID, one, "")
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

const configurationManifestFixture = `version = 8

[[providers]]
id = "aihubmix"
label = "AIHubMix"
endpoints = {anthropic = "https://aihubmix.test", responses = "https://aihubmix.test/v1"}

[[providers.models]]
id = "claude-test"
family = "claude-test"
label = "AIHubMix Claude"
protocols = {anthropic = []}

[[providers]]
id = "dmxapi"
label = "DMXAPI"
endpoints = {anthropic = "https://dmxapi.test", responses = "https://dmxapi.test/v1"}

[[providers.models]]
id = "claude-test"
family = "claude-test"
label = "DMXAPI Claude"
protocols = {anthropic = []}

[[providers.models]]
id = "gpt-test"
family = "gpt-test"
label = "DMXAPI GPT"
protocols = {responses = []}

[[families]]
id = "claude-test"
label = "Claude Test"

[[families]]
id = "gpt-test"
label = "GPT Test"

[recommendations.claude]
primary = {model = "aihubmix/claude-test"}
alternatives = [{model = "dmxapi/claude-test"}]

[recommendations.codex]
primary = {model = "dmxapi/gpt-test"}
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

func (d fakeDiscovery) Discover(...string) discovery.Result { return d.result }

func readyVerificationApp(t *testing.T) (*cli.App, *fakeRunner) {
	t.Helper()
	app, _, secretStore, runner, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["dmx"] = domain.Provider{Label: "DMX", Endpoints: domain.Endpoints{Responses: "https://example.test/v1", Anthropic: "https://example.test"}}
	cfg.Models[domain.NewModelRef("dmx", "gpt-test")] = qualifiedModel("GPT", "dmx", "gpt-test", domain.ProtocolResponses)
	cfg.Models[domain.NewModelRef("dmx", "claude-test")] = qualifiedModel("Claude", "dmx", "claude-test", domain.ProtocolAnthropic)
	cfg.SetSelectedModel(codex.ID, domain.NewModelRef("dmx", "gpt-test"), "")
	cfg.SetSelectedModel(claude.ID, domain.NewModelRef("dmx", "claude-test"), "")
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
