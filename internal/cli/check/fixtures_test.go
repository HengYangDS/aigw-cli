package check

import (
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/client/claude"
	"aigw-cli/internal/client/codex"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func successfulReadinessResponse(request *http.Request) (*http.Response, error) {
	body := `{"data":[]}`
	switch request.Method + " " + request.URL.Path {
	case "POST /v1/messages":
		body = `{"type":"message","role":"assistant","content":[{"type":"text","text":"pong"}],"stop_reason":"end_turn"}`
	case "POST /v1/responses":
		body = `{"status":"completed","output":[{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"pong"}]}]}`
	case "POST /v1/chat/completions":
		body = `{"choices":[{"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}]}`
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

// hostAt observes home as the user's home directory, so Claude Code's settings
// file is home/.claude/settings.json.
func hostAt(home string) discovery.Host {
	return discovery.Host{GOOS: goruntime.GOOS, Home: home, Env: map[string]string{"HOME": home, "USERPROFILE": home}}
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

func (fn roundTripFunc) Do(request *http.Request) (*http.Response, error) { return fn(request) }

type observingSecretStore struct {
	value          string
	getErr         error
	existsErr      error
	getCalls       int
	existsCalls    int
	getAccounts    []string
	existsAccounts []string
}

func (store *observingSecretStore) Get(account string) (string, error) {
	store.getCalls++
	store.getAccounts = append(store.getAccounts, account)
	if store.getErr != nil {
		return "", store.getErr
	}
	if store.value == "" {
		return "", secret.ErrNotFound
	}
	return store.value, nil
}

func (*observingSecretStore) Set(string, string) error { return nil }

func (*observingSecretStore) Delete(string) error { return nil }

func (store *observingSecretStore) Exists(account string) (bool, error) {
	store.existsCalls++
	store.existsAccounts = append(store.existsAccounts, account)
	return store.value != "", store.existsErr
}

func configuredReadinessRuntime(t *testing.T) (invocation.Context, domain.Config, *bytes.Buffer) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "configuration.toml")
	configStore := store.NewStore(configPath, clients.Admission())
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["one"] = domain.Account{
		Label: "One",
		Endpoints: domain.Endpoints{
			Anthropic:       "https://claude.example.test",
			OpenAIResponses: "https://codex.example.test/v1",
		},
	}
	cfg.Routes["claude"] = domain.Route{
		Label: "Claude", Account: "one", Model: "claude-test",
		Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}},
	}
	cfg.Routes["codex"] = domain.Route{
		Label: "Codex", Account: "one", Model: "gpt-test",
		Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}},
	}
	cfg.SetSelectedRoute(claude.ID, "claude", "")
	cfg.SetSelectedRoute(codex.ID, "codex", "")
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	runtime := invocation.Context{Clients: clients.Registry(),
		Config: configStore, Secrets: secret.NewMemoryStore(),
		Out: out, RenderOut: out, Width: 120,
	}
	var err error
	runtime.Accounts, err = secret.NewDiagnosticCredentialStore(runtime.Secrets)
	if err != nil {
		t.Fatal(err)
	}
	return runtime, cfg, out
}

func configureClaudeExecutable(t *testing.T, runtime *invocation.Context, cfg *domain.Config) {
	t.Helper()
	root := t.TempDir()
	executable := filepath.Join(root, "claude")
	if goruntime.GOOS == "windows" {
		executable += ".exe"
	}
	if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	runtime.Executable = filepath.Join(root, "aigw")
	runtime.Host = hostAt(root)
	cfg.SetClientActivation(claude.ID, true, executable, nil)
}

func synchronizeClaudeSettings(t *testing.T, runtime invocation.Context, cfg domain.Config) {
	t.Helper()
	clientRuntime, err := cfg.ResolveRuntime(claude.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := claude.ReconcileSettings(claudeSettingsPath(t, runtime.Host), false, clientRuntime, runtime.Executable, clientRuntime.Model); err != nil {
		t.Fatal(err)
	}
}

func executeCommand(command *cobra.Command) error {
	command.SetErr(io.Discard)
	command.SilenceErrors = true
	command.SilenceUsage = true
	return command.Execute()
}
