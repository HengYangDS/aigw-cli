package cli_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aigw-cli/internal/claude"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"
)

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
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
	plans   []process.Plan
	output  []byte
	capture error
}

func (r *fakeRunner) RunCapture(_ context.Context, plan process.Plan) ([]byte, error) {
	r.plans = append(r.plans, plan)
	if len(plan.Args) == 1 && plan.Args[0] == "--version" {
		return []byte("codex-cli 0.0.0-test\n"), nil
	}
	if r.capture != nil {
		return append([]byte(nil), r.output...), r.capture
	}
	if outputPath := planArgumentValue(plan.Args, "--output-last-message"); outputPath != "" {
		output := r.output
		if output == nil {
			output = []byte("AIGW_OK\n")
		}
		if err := os.WriteFile(outputPath, output, 0o600); err != nil {
			return nil, err
		}
		return []byte("non-authoritative diagnostic output\n"), nil
	}
	if r.output == nil {
		return []byte("AIGW_OK\n"), nil
	}
	return append([]byte(nil), r.output...), nil
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

func secretExists(t testing.TB, store secrets.Store, account string) bool {
	t.Helper()
	present, err := store.Exists(account)
	if err != nil {
		t.Fatalf("observe credential for %q: %v", account, err)
	}
	return present
}

func accountCredentialExists(t testing.TB, store secrets.DiagnosticCredentialStore, accountID string) bool {
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
		body = "{}"
	}
	return &http.Response{StatusCode: f.status, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
}

func testApp(t *testing.T, stdin string) (*cli.App, *bytes.Buffer, secrets.Store, *fakeRunner, *fakeHTTP) {
	t.Helper()
	out := new(bytes.Buffer)
	secretStore := secrets.NewMemoryStore()
	diagnostics, err := secrets.NewDiagnosticCredentialStore(secretStore)
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}
	httpClient := &fakeHTTP{status: 200}
	app := &cli.App{
		Version:            "0.1.0-test",
		Executable:         filepath.Join(t.TempDir(), "aigw"),
		InstallTarget:      filepath.Join(t.TempDir(), "bin", "aigw"),
		ClaudeSettingsPath: filepath.Join(t.TempDir(), ".claude", "settings.json"),
		Config:             configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml")),
		Secrets:            secretStore,
		Accounts:           diagnostics,
		Env:                []string{},
		In:                 strings.NewReader(stdin),
		Out:                out,
		Err:                out,
		Interactive:        false,
		Runner:             runner,
		HTTP:               httpClient,
		Discovery:          fakeDiscovery{},
	}
	return app, out, secretStore, runner, httpClient
}

func addAccountProfile(cfg *configuration.Config, profileName, accountName, label string, endpoints configuration.Endpoints, client, model string) {
	if _, exists := cfg.Accounts[accountName]; !exists {
		cfg.Accounts[accountName] = configuration.Account{Label: label, Endpoints: endpoints}
	}
	cfg.Profiles[profileName] = configuration.Profile{Label: label, Account: accountName, Client: client, Model: model}
}

func synchronizeClaudeProjection(t *testing.T, app *cli.App, cfg configuration.Config) {
	t.Helper()
	runtime, err := cfg.ResolveRuntime(configuration.ClientClaude, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := claude.ReconcileSettings(app.ClaudeSettingsPath, false, runtime, app.Executable, runtime.Model); err != nil {
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

func saveCommandProfile(t *testing.T, app *cli.App, endpoints configuration.Endpoints, client, model string) {
	t.Helper()
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", endpoints, client, model)
	cfg.Routes[client] = "one"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
}

type recordingCredentialStore[T string | secrets.DiagnosticCredential] struct {
	backend interface {
		Get(string) (T, error)
		Set(string, T) error
		Delete(string) error
		Exists(string) (bool, error)
	}
	getCalls    []string
	existsCalls []string
	setCalls    []string
	deleteCalls []string
	existsErr   error
	getErr      error
	setErr      error
	deleteErr   error
}

func (store *recordingCredentialStore[T]) Get(account string) (T, error) {
	store.getCalls = append(store.getCalls, account)
	if store.getErr != nil {
		var value T
		return value, store.getErr
	}
	return store.backend.Get(account)
}

func (store *recordingCredentialStore[T]) Set(account string, value T) error {
	store.setCalls = append(store.setCalls, account)
	if store.setErr != nil {
		return store.setErr
	}
	return store.backend.Set(account, value)
}

func (store *recordingCredentialStore[T]) Delete(account string) error {
	store.deleteCalls = append(store.deleteCalls, account)
	if store.deleteErr != nil {
		return store.deleteErr
	}
	return store.backend.Delete(account)
}

func (store *recordingCredentialStore[T]) Exists(account string) (bool, error) {
	store.existsCalls = append(store.existsCalls, account)
	if store.existsErr != nil {
		return false, store.existsErr
	}
	return store.backend.Exists(account)
}

func TestCredentialFixturePreservesRealStateAcrossInjectedFailures(t *testing.T) {
	backend := secrets.NewMemoryStore()
	store := &recordingCredentialStore[string]{backend: backend}
	if err := store.Set("account", "original"); err != nil {
		t.Fatal(err)
	}
	want := errors.New("credential mutation failed")
	store.setErr, store.deleteErr = want, want
	if err := store.Set("account", "replacement"); !errors.Is(err, want) {
		t.Fatalf("failed write = %v", err)
	}
	if err := store.Delete("account"); !errors.Is(err, want) {
		t.Fatalf("failed deletion = %v", err)
	}
	if value, err := store.Get("account"); err != nil || value != "original" {
		t.Fatalf("failed mutation changed stored value: %q, %v", value, err)
	}
	store.setErr, store.deleteErr = nil, nil
	if err := store.Set("account", "replacement"); err != nil {
		t.Fatal(err)
	}
	if value, err := backend.Get("account"); err != nil || value != "replacement" {
		t.Fatalf("successful write did not reach the backend: %q, %v", value, err)
	}
	if err := store.Delete("account"); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.Get("account"); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("successful deletion retained credential: %v", err)
	}
	if len(store.setCalls) != 3 || len(store.deleteCalls) != 2 || len(store.getCalls) != 1 {
		t.Fatal("credential observations lost an attempted operation")
	}
}

func saveProbeProfile(t *testing.T, appConfig configuration.Store) {
	t.Helper()
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{
		Label:        "DMXAPI",
		Endpoints:    configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"},
		AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://www.dmxapi.cn"},
	}
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Account: "dmx", Client: configuration.ClientCodex, Model: "gpt-test"}
	cfg.Routes[configuration.ClientCodex] = "gpt"
	if err := appConfig.Save(cfg); err != nil {
		t.Fatal(err)
	}
}

func twoProfileConfig() configuration.Config {
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One Gateway", configuration.Endpoints{Anthropic: "https://one.test", OpenAIResponses: "https://one.test/v1"}, configuration.ClientCodex, "model-one")
	addAccountProfile(&cfg, "two", "two", "Two Gateway", configuration.Endpoints{Anthropic: "https://two.test", OpenAIResponses: "https://two.test/v1"}, configuration.ClientCodex, "model-two")
	cfg.Routes[configuration.ClientCodex] = "one"
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

const configurationManifestFixture = `version = 4
[recommended_routes]
claude = "aihubmix-claude"
codex = "dmxapi-gpt"

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

[profiles.aihubmix-claude]
label = "AIHubMix Claude"
account = "aihubmix"
client = "claude"
model = "claude-test"

[profiles.dmxapi-claude]
label = "DMXAPI Claude"
account = "dmxapi"
client = "claude"
model = "claude-test"

[profiles.dmxapi-gpt]
label = "DMXAPI GPT"
account = "dmxapi"
client = "codex"
model = "gpt-test"
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
