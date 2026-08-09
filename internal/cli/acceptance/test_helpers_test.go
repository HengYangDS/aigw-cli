package cli_test

import (
	"aigw-cli/internal/account"
	"aigw-cli/internal/cli"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/process"
	"aigw-cli/internal/prompt"
	"aigw-cli/internal/secrets"
	"aigw-cli/internal/upgrade"
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
)

type fakeRunner struct {
	plans            []process.Plan
	captureDeadlines []bool
	output           []byte
	capture          error
}

func (r *fakeRunner) Run(_ context.Context, plan process.Plan) error {
	r.plans = append(r.plans, plan)
	return nil
}

func (r *fakeRunner) RunCapture(ctx context.Context, plan process.Plan) ([]byte, error) {
	r.plans = append(r.plans, plan)
	_, hasDeadline := ctx.Deadline()
	r.captureDeadlines = append(r.captureDeadlines, hasDeadline)
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

// emptyDiscovery preserves the production dependency boundary in command
// tests: commands always receive a discovery service, while individual tests
// can still exercise an explicitly configured temporary Codex home.
type emptyDiscovery struct{}

func (emptyDiscovery) Discover() discovery.Result { return discovery.Result{} }

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

func (r *failingRunner) Run(_ context.Context, _ process.Plan) error {
	if r.remaining > 0 {
		r.remaining--
		return r.err
	}
	return nil
}

func (r *failingRunner) RunCapture(_ context.Context, _ process.Plan) ([]byte, error) {
	return []byte("AIGW_OK\n"), nil
}

type fakeHTTP struct {
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

type contextBoundReadCloser struct {
	ctx    context.Context
	reader *strings.Reader
}

func (body *contextBoundReadCloser) Read(data []byte) (int, error) {
	select {
	case <-body.ctx.Done():
		return 0, body.ctx.Err()
	default:
		return body.reader.Read(data)
	}
}

func (body *contextBoundReadCloser) Close() error { return nil }

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
		Version:            "0.1.0-test",
		Executable:         filepath.Join(t.TempDir(), "aigw"),
		InstallTarget:      filepath.Join(t.TempDir(), "bin", "aigw"),
		ClaudeSettingsPath: filepath.Join(t.TempDir(), ".claude", "settings.json"),
		Config:             configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml")),
		Secrets:            secretStore,
		Accounts:           account.NewMemoryStore(),
		Env:                []string{},
		In:                 strings.NewReader(stdin),
		Out:                out,
		Err:                out,
		Interactive:        false,
		Runner:             runner,
		HTTP:               httpClient,
		Discovery:          emptyDiscovery{},
	}
	return app, out, secretStore, runner
}

func addAccountProfile(cfg *configuration.Config, profileName, accountName, label string, endpoints configuration.Endpoints, client string, models configuration.Models) {
	if _, exists := cfg.Accounts[accountName]; !exists {
		cfg.Accounts[accountName] = configuration.Account{Label: label, Endpoints: endpoints}
	}
	cfg.Profiles[profileName] = configuration.Profile{Label: label, Account: accountName, Client: client, Models: models}
}

func execute(t *testing.T, app *cli.App, args ...string) error {
	t.Helper()
	return cli.Execute(app, args)
}

func executableFixture(t *testing.T, name string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("native client fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

type closeFailingBody struct {
	io.Reader
	err error
}

func (body closeFailingBody) Close() error { return body.err }

func saveCommandProfile(t *testing.T, app *cli.App, endpoints configuration.Endpoints, client string, models configuration.Models) {
	t.Helper()
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One", endpoints, client, models)
	cfg.Routes.Default = "one"
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
}

type failingAccountStore struct {
	account.Store
	setErr    error
	deleteErr error
}

func (store failingAccountStore) Set(string, account.Credential) error { return store.setErr }

func (store failingAccountStore) Delete(string) error { return store.deleteErr }

func saveProbeProfile(t *testing.T, appConfig configuration.Store) {
	t.Helper()
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{
		Label:        "DMXAPI",
		Endpoints:    configuration.Endpoints{OpenAIResponses: "https://dmx.test/v1"},
		AccountProbe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://www.dmxapi.cn"},
	}
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Account: "dmx", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-test"}}
	cfg.Routes.Default = "gpt"
	if err := appConfig.Save(cfg); err != nil {
		t.Fatal(err)
	}
}

type fakeUpdater struct {
	updateCalls       int
	candidateCalls    int
	rollbackCalls     int
	updateResult      string
	candidateResult   string
	rollbackResult    string
	updateErr         error
	candidateErr      error
	rollbackErr       error
	candidateReceived upgrade.CandidateArchive
}

func (u *fakeUpdater) Update(_ context.Context, _ string) (string, error) {
	u.updateCalls++
	return u.updateResult, u.updateErr
}

func (u *fakeUpdater) UpdateCandidate(_ context.Context, _ string, candidate upgrade.CandidateArchive) (string, error) {
	u.candidateCalls++
	u.candidateReceived = candidate
	return u.candidateResult, u.candidateErr
}

func (u *fakeUpdater) Rollback(_ context.Context) (string, error) {
	u.rollbackCalls++
	return u.rollbackResult, u.rollbackErr
}

func twoProfileConfig() configuration.Config {
	cfg := configuration.NewConfig()
	addAccountProfile(&cfg, "one", "one", "One Gateway", configuration.Endpoints{Anthropic: "https://one.test", OpenAIResponses: "https://one.test/v1"}, "", configuration.Models{})
	addAccountProfile(&cfg, "two", "two", "Two Gateway", configuration.Endpoints{Anthropic: "https://two.test", OpenAIResponses: "https://two.test/v1"}, "", configuration.Models{})
	cfg.Routes.Default = "one"
	return cfg
}

type recordingPrompt struct {
	calls int
}

func (p *recordingPrompt) Secret(string) (string, error) {
	p.calls++
	return "", errors.New("prompt forbidden during check")
}

func (p *recordingPrompt) Text(string) (string, error) {
	p.calls++
	return "", errors.New("prompt forbidden during check")
}

func (p *recordingPrompt) Select(string, []prompt.Choice) (string, error) {
	p.calls++
	return "", errors.New("prompt forbidden during check")
}
