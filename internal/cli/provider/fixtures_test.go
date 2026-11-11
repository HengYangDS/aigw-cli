package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// teamToken is the team provider's Token, which the balance probe's account
// lists by its masked key.
const teamToken = "sk-team-token-0001"

// fakeHTTP answers the requests that the provider commands send: the
// model-free Token check of each host with the status set for it, or a
// transport failure, and the DMXAPI balance probe. It records each request.
type fakeHTTP struct {
	mu       sync.Mutex
	requests []string
	status   map[string]int
	failures map[string]error
	// tokens and user replace the balance probe's list of Tokens and its
	// account query's answer when set.
	tokens string
	user   string
}

func (f *fakeHTTP) Do(request *http.Request) (*http.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, request.Method+" "+request.URL.String())
	if err := f.failures[request.URL.Host]; err != nil {
		return nil, err
	}
	status, set := f.status[request.URL.Host]
	if !set {
		status = http.StatusOK
	}
	body := `{"data":[{"id":"team-model"}]}`
	switch request.URL.Path {
	case "/api/user/self":
		body = `{"success":true,"data":{"quota":5000000}}`
		if f.user != "" {
			body = f.user
		}
	case "/api/token/search":
		body = `{"success":true,"data":{"page_size":100,"items":[{"name":"team","key":"team**********0001","status":1,"used_quota":500000,"remain_quota":2500000,"remain_count":42}]}}`
		if f.tokens != "" {
			body = f.tokens
		}
	}
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
}

// sent returns the requests that the fake received.
func (f *fakeHTTP) sent() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string{}, f.requests...)
}

// prompter answers secret prompts with its values in order, and records the
// prompts; before runs first, as another program could while a person types.
type prompter struct {
	values []string
	asked  []string
	before func()
}

func (p *prompter) Secret(label string) (string, error) {
	p.asked = append(p.asked, label)
	if p.before != nil {
		p.before()
	}
	if len(p.values) == 0 {
		return "", errors.New("no answer")
	}
	value := p.values[0]
	p.values = p.values[1:]
	return value, nil
}

func (*prompter) Text(string) (string, error) { return "", errors.New("no text prompt") }

func (*prompter) Select(string, []terminal.Choice) (string, error) {
	return "", errors.New("no choice prompt")
}

// fixture is a configuration with two providers. team has an Anthropic and a
// Responses endpoint, a DMXAPI balance probe, a Token and balance credentials;
// Claude selects its team-model, whose Anthropic protocol declares tool
// search, and Codex selects its codex-model with forwarding and retry budgets.
// mixed has an Anthropic and a Chat Completions endpoint and no Token; Hermes
// recommends its chat-only model.
type fixture struct {
	runtime invocation.Context
	out     *bytes.Buffer
	http    *fakeHTTP
	path    string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "configuration.toml")
	configStore := store.NewStore(path, clients.Admission())
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["team"] = domain.Provider{
		Label: "Team", Endpoints: domain.Endpoints{Anthropic: "https://team.test", Responses: "https://team.test/v1"},
		BalanceProbe: &domain.BalanceProbe{Kind: "dmxapi", BaseURL: "https://balance.test"},
	}
	cfg.Providers["mixed"] = domain.Provider{Label: "Mixed", Endpoints: domain.Endpoints{Anthropic: "https://mixed.test", Chat: "https://mixed.test/v1"}}
	cfg.Models[domain.NewModelRef("team", "team-model")] = domain.Model{Provider: "team", Family: "team-model", ID: "team-model", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {domain.CapabilityToolSearch}}}
	cfg.Models[domain.NewModelRef("team", "codex-model")] = domain.Model{Provider: "team", Family: "codex-model", ID: "codex-model", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolResponses: {}}}
	cfg.Models[domain.NewModelRef("mixed", "chat-only")] = domain.Model{Provider: "mixed", Family: "chat-only", ID: "chat-only", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolChat: {}}}
	cfg.SetSelectedModel("claude", domain.NewModelRef("team", "team-model"), "")
	cfg.SetSelectedModel("codex", domain.NewModelRef("team", "codex-model"), "")
	if err := cfg.SetForwardingEndpoint("codex", "http://127.0.0.1:8792/v1"); err != nil {
		t.Fatal(err)
	}
	budget := uint64(3)
	binding := cfg.Clients["codex"]
	binding.RetryPolicy = &domain.RetryPolicy{RequestMaxRetries: &budget}
	cfg.Clients["codex"] = binding
	cfg.Recommendations["hermes"] = domain.ClientRecommendation{Primary: domain.ClientSelection{Model: domain.NewModelRef("mixed", "chat-only"), Protocol: domain.ProtocolChat}}
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	secrets := secret.NewMemoryStore()
	diagnostics, err := secret.NewDiagnosticCredentialStore(secrets)
	if err != nil {
		t.Fatal(err)
	}
	if err := secrets.Set("team", teamToken); err != nil {
		t.Fatal(err)
	}
	if err := diagnostics.Set("team", secret.DiagnosticCredential{SystemToken: "system-token", UserID: "42"}); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	fake := &fakeHTTP{status: map[string]int{}, failures: map[string]error{}}
	return &fixture{
		runtime: invocation.Context{Clients: clients.Registry(), Config: configStore, Secrets: secrets, Providers: diagnostics, HTTP: fake, Out: out, In: strings.NewReader("")},
		out:     out, http: fake, path: path,
	}
}

// emptyStore is a configuration store without a configuration file.
func emptyStore(t *testing.T) store.Store {
	t.Helper()
	return store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
}

// withEnvironment selects the read-only environment backend with variables.
func (f *fixture) withEnvironment(variables map[string]string) {
	backend := secret.NewEnvironmentStore(func(key string) string { return variables[key] })
	diagnostics, _ := secret.NewDiagnosticCredentialStore(backend)
	f.runtime.Secrets, f.runtime.Providers = backend, diagnostics
}

// run executes `aigw provider <args>` with stdin and returns its error.
func (f *fixture) run(stdin string, args ...string) error {
	f.out.Reset()
	f.runtime.In = strings.NewReader(stdin)
	root := &cobra.Command{Use: "aigw", SilenceErrors: true, SilenceUsage: true}
	root.AddCommand(NewCommand(f.runtime))
	root.SetOut(f.out)
	root.SetArgs(append([]string{"provider"}, args...))
	return root.Execute()
}

// load returns the stored configuration.
func (f *fixture) load(t *testing.T) domain.Config {
	t.Helper()
	cfg, err := f.runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// configuration returns the configuration file's bytes.
func (f *fixture) configuration(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(f.path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// decode decodes the JSON document that the last command wrote into target.
func (f *fixture) decode(t *testing.T, target any) {
	t.Helper()
	if err := json.Unmarshal(f.out.Bytes(), target); err != nil {
		t.Fatalf("output is not one JSON document: %v\n%s", err, f.out)
	}
}

// text is the last command's human output with runs of spaces joined.
func (f *fixture) text() string { return strings.Join(strings.Fields(f.out.String()), " ") }

// problem is the Problem and Fix that err presents.
func problem(err error) terminal.Problem {
	if err == nil {
		return terminal.Problem{}
	}
	return terminal.ProblemFor(err)
}

// token returns the stored Token of a provider, or "" without one.
func (f *fixture) token(id string) string {
	value, _ := f.runtime.Secrets.Get(id)
	return value
}
