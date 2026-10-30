package status

import (
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/cli/check"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

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

func TestClaudeReadinessFollowsSettingsConvergence(t *testing.T) {
	runtime, cfg, buffer := configuredReadinessRuntime(t)
	runtime.Version = "1.0.0"
	configureClaudeExecutable(t, &runtime, &cfg)
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	secretStore := &observingSecretStore{value: "token"}
	runtime.Secrets = secretStore
	requests := 0
	runtime.HTTP = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		return successfulReadinessResponse(request)
	})
	command := &cobra.Command{}
	command.SetContext(context.Background())
	for _, synchronized := range []bool{false, true} {
		if synchronized {
			synchronizeClaudeSettings(t, runtime, cfg)
		}
		buffer.Reset()
		secretReads := secretStore.getCalls
		if err := RunStatus(runtime, true); err != nil {
			t.Fatal(err)
		}
		var status statusReport
		if err := json.Unmarshal(buffer.Bytes(), &status); err != nil {
			t.Fatal(err)
		}
		client := status.Clients[claude.ID]
		if client.ProjectionReady != synchronized || secretStore.getCalls != secretReads {
			t.Errorf("synchronized=%t client=%+v secret reads=%d", synchronized, client, secretStore.getCalls-secretReads)
		}
		err := check.RunCheck(command, runtime)
		if synchronized {
			if err != nil || requests == 0 {
				t.Errorf("synchronized check: error=%v requests=%d", err, requests)
			}
		} else if err == nil || requests != 0 || secretStore.getCalls != secretReads || !strings.Contains(buffer.String(), "aigw sync") {
			t.Errorf("unsynchronized check: error=%v requests=%d secret reads=%d output=%s", err, requests, secretStore.getCalls-secretReads, buffer.String())
		}
	}
}

func executeCommand(command *cobra.Command) error {
	command.SetErr(io.Discard)
	command.SilenceErrors = true
	command.SilenceUsage = true
	return command.Execute()
}
