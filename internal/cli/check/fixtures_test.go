package check

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
)

// answer is a provider that answers every request successfully: a model list
// for a request without a model, and a short answer for an inference request.
func answer(request *http.Request) (*http.Response, error) {
	body := `{"data":[]}`
	switch request.Method + " " + request.URL.Path {
	case "POST /v1/messages":
		body = `{"type":"message","role":"assistant","content":[{"type":"text","text":"pong"}],"stop_reason":"end_turn"}`
	case "POST /v1/responses":
		body = `{"status":"completed","output":[{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"pong"}]}]}`
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
}

// status is a provider that answers every request with code.
func status(code int) func(*http.Request) (*http.Response, error) {
	return func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(`{}`)), Request: request}, nil
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) Do(request *http.Request) (*http.Response, error) { return fn(request) }

// recorder answers as respond does and records each request as its method,
// host and path.
type recorder struct {
	respond  func(*http.Request) (*http.Response, error)
	requests []string
}

func (r *recorder) Do(request *http.Request) (*http.Response, error) {
	r.requests = append(r.requests, request.Method+" "+request.URL.Host+request.URL.Path)
	return r.respond(request)
}

// observingSecretStore records each Token read, each metadata read and each
// write, by provider: a Set with its value and a Delete, the only writes that
// a secret.Store has.
type observingSecretStore struct {
	tokens map[string]string
	// empty names the providers whose item holds an empty value, which Exists
	// reports present, as a store whose metadata cannot tell does, and Get
	// reports as ErrEmpty.
	empty           map[string]bool
	existsErr       error
	getProviders    []string
	existsProviders []string
	writes          []string
}

func (store *observingSecretStore) Get(provider string) (string, error) {
	store.getProviders = append(store.getProviders, provider)
	if store.empty[provider] {
		return "", secret.ErrEmpty
	}
	if token, found := store.tokens[provider]; found {
		return token, nil
	}
	return "", secret.ErrNotFound
}

func (store *observingSecretStore) Set(provider, value string) error {
	store.writes = append(store.writes, "set "+provider+" "+value)
	return nil
}

func (store *observingSecretStore) Delete(provider string) error {
	store.writes = append(store.writes, "delete "+provider)
	return nil
}

func (store *observingSecretStore) Exists(provider string) (bool, error) {
	store.existsProviders = append(store.existsProviders, provider)
	_, found := store.tokens[provider]
	return found || store.empty[provider], store.existsErr
}

// secrets is the credential store that configuredRuntime gives runtime.
func secrets(t *testing.T, runtime invocation.Context) *observingSecretStore {
	t.Helper()
	store, ok := runtime.Secrets.(*observingSecretStore)
	if !ok {
		t.Fatalf("the credential store is a %T", runtime.Secrets)
	}
	return store
}

// configuredRuntime is a home with provider one, whose Anthropic and
// Responses endpoints serve models claude and codex, which Claude Code and
// Codex select without being enabled. Provider one has a Token.
func configuredRuntime(t *testing.T) (invocation.Context, domain.Config, *bytes.Buffer) {
	t.Helper()
	root := t.TempDir()
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["one"] = domain.Provider{Label: "One", Endpoints: domain.Endpoints{Anthropic: "https://claude.example.test", OpenAIResponses: "https://codex.example.test/v1"}}
	cfg.Models["claude"] = domain.Model{Label: "Claude", Provider: "one", Family: "claude-test", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	cfg.Models["codex"] = domain.Model{Label: "Codex", Provider: "one", Family: "gpt-test", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
	cfg.SetSelectedModel(claude.ID, "claude", "")
	cfg.SetSelectedModel(codex.ID, "codex", "")
	out := &bytes.Buffer{}
	runtime := invocation.Context{
		Clients: clients.Registry(), Config: store.NewStore(filepath.Join(root, "configuration.toml"), clients.Admission()),
		Secrets: &observingSecretStore{tokens: map[string]string{"one": "fixture-token"}}, Out: out, Width: 120,
		HTTP: roundTripFunc(answer), Executable: filepath.Join(root, "aigw"),
		Host: discovery.Host{GOOS: goruntime.GOOS, Home: root, Env: map[string]string{"HOME": root, "USERPROFILE": root}},
	}
	save(t, runtime, cfg)
	return runtime, cfg, out
}

// runtimeFixture is a configured home: the invocation, the configuration
// that it saved, and the output.
type runtimeFixture struct {
	ctx invocation.Context
	cfg domain.Config
	out *bytes.Buffer
}

// configuration is the saved configuration's bytes.
func (f runtimeFixture) configuration(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(f.ctx.Config.Path())
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func save(t *testing.T, runtime invocation.Context, cfg domain.Config) {
	t.Helper()
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
}

// enable enables each client with an executable and writes the client files
// that match its selection, as sync writes them.
func enable(t *testing.T, runtime invocation.Context, cfg *domain.Config, ids ...string) {
	t.Helper()
	root := runtime.Host.Home
	for _, id := range ids {
		executable := filepath.Join(root, id)
		if goruntime.GOOS == "windows" {
			executable += ".exe"
		}
		if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
			t.Fatal(err)
		}
		switch id {
		case claude.ID:
			cfg.SetClientActivation(id, true, executable, nil)
			selected := resolve(t, *cfg, id)
			settings, err := claude.SettingsPath(runtime.Host)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := claude.ReconcileSettings(settings, false, selected, runtime.Executable, selected.UpstreamModel); err != nil {
				t.Fatal(err)
			}
		case codex.ID:
			target := filepath.Join(root, ".codex", "config.toml")
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			cfg.SetClientActivation(id, true, executable, []string{target})
			selected := resolve(t, *cfg, id)
			selected.CredentialCommand = runtime.Executable
			if _, err := codex.ReconcileConfigs(nil, []codex.TargetRef{codex.HomeTarget(target)}, selected, selected, false); err != nil {
				t.Fatal(err)
			}
		default:
			t.Fatalf("no client files for %s", id)
		}
	}
	save(t, runtime, *cfg)
}

func resolve(t *testing.T, cfg domain.Config, id string) domain.Runtime {
	t.Helper()
	selected, err := cfg.ResolveRuntime(id, "")
	if err != nil {
		t.Fatal(err)
	}
	return selected
}

// execute runs `aigw check` with args and returns its error.
func execute(runtime invocation.Context, args ...string) error {
	command := NewCommand(runtime)
	command.SetArgs(args)
	command.SetErr(io.Discard)
	command.SilenceErrors, command.SilenceUsage = true, true
	return command.Execute()
}

// checkDocument decodes check's document with its header.
type checkDocument struct {
	document
	SchemaVersion int  `json:"schema_version"`
	OK            bool `json:"ok"`
}

func decode(t *testing.T, out *bytes.Buffer) checkDocument {
	t.Helper()
	var doc checkDocument
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("decode check's document: %v\n%s", err, out)
	}
	return doc
}

// client returns the document's entry of id.
func (doc checkDocument) client(t *testing.T, id string) clientDocument {
	t.Helper()
	index := slices.IndexFunc(doc.Clients, func(client clientDocument) bool { return client.ID == id })
	if index < 0 {
		t.Fatalf("the document has no %s: %+v", id, doc.Clients)
	}
	return doc.Clients[index]
}
