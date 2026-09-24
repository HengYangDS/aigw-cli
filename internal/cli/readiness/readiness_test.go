package readiness

import (
	"aigw-cli/internal/claude"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
	domainreadiness "aigw-cli/internal/readiness"
	"aigw-cli/internal/secrets"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	goruntime "runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) Do(request *http.Request) (*http.Response, error) { return fn(request) }

func TestCheckCommandDescribesItsProductBoundary(t *testing.T) {
	command := NewCheckCommand(invocation.Context{})
	if command.Short != "Check client bindings, credentials, projections, and endpoints" {
		t.Fatalf("check summary = %q", command.Short)
	}
}

type failingReader struct{ err error }

func (reader failingReader) Read([]byte) (int, error) { return 0, reader.err }

type failingOutputWriter struct{ err error }

func (writer failingOutputWriter) Write([]byte) (int, error) { return 0, writer.err }

type closeErrorBody struct{ io.Reader }

func (closeErrorBody) Close() error { return errors.New("close failed") }

type presentFailingSecretStore struct{ err error }

func (store presentFailingSecretStore) Get(string) (string, error) { return "", store.err }
func (presentFailingSecretStore) Set(string, string) error         { return nil }
func (presentFailingSecretStore) Delete(string) error              { return nil }
func (presentFailingSecretStore) Exists(string) (bool, error)      { return true, nil }

type failingAccountObservationStore struct {
	err   error
	reads int
}

func (store *failingAccountObservationStore) Get(string) (secrets.DiagnosticCredential, error) {
	store.reads++
	return secrets.DiagnosticCredential{}, store.err
}
func (*failingAccountObservationStore) Set(string, secrets.DiagnosticCredential) error { return nil }
func (*failingAccountObservationStore) Delete(string) error                            { return nil }
func (store *failingAccountObservationStore) Exists(string) (bool, error) {
	store.reads++
	return false, store.err
}

type observingSecretStore struct {
	value       string
	getErr      error
	existsErr   error
	getCalls    int
	existsCalls int
}

func (store *observingSecretStore) Get(string) (string, error) {
	store.getCalls++
	if store.getErr != nil {
		return "", store.getErr
	}
	if store.value == "" {
		return "", secrets.ErrNotFound
	}
	return store.value, nil
}

func (*observingSecretStore) Set(string, string) error { return nil }
func (*observingSecretStore) Delete(string) error      { return nil }

func (store *observingSecretStore) Exists(string) (bool, error) {
	store.existsCalls++
	return store.value != "", store.existsErr
}

func configuredReadinessRuntime(t *testing.T) (invocation.Context, configuration.Config, *bytes.Buffer) {
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
	cfg.Routes["claude"] = configuration.Route{
		Label: "Claude", Account: "one", Model: "claude-test",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}},
	}
	cfg.Routes["codex"] = configuration.Route{
		Label: "Codex", Account: "one", Model: "gpt-test",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}},
	}
	cfg.SetSelectedRoute(configuration.ClientClaude, "claude")
	cfg.SetSelectedRoute(configuration.ClientCodex, "codex")
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	runtime := invocation.Context{
		Config: store, Secrets: secrets.NewMemoryStore(),
		Out: out, RenderOut: out, Width: 120,
	}
	var err error
	runtime.Accounts, err = secrets.NewDiagnosticCredentialStore(runtime.Secrets)
	if err != nil {
		t.Fatal(err)
	}
	return runtime, cfg, out
}

func configureClaudeExecutable(t *testing.T, runtime *invocation.Context, cfg *configuration.Config) {
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
	runtime.ClaudeSettingsPath = filepath.Join(root, "settings.json")
	cfg.SetClientActivation(configuration.ClientClaude, true, executable, nil)
}

func synchronizeClaudeSettings(t *testing.T, runtime invocation.Context, cfg configuration.Config) {
	t.Helper()
	clientRuntime, err := cfg.ResolveRuntime(configuration.ClientClaude, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := claude.ReconcileSettings(runtime.ClaudeSettingsPath, false, clientRuntime, runtime.Executable, clientRuntime.Model); err != nil {
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
		var status statusOutput
		if err := json.Unmarshal(buffer.Bytes(), &status); err != nil {
			t.Fatal(err)
		}
		client := status.Clients[configuration.ClientClaude]
		if client.ProjectionReady != synchronized || secretStore.getCalls != secretReads {
			t.Errorf("synchronized=%t client=%+v secret reads=%d", synchronized, client, secretStore.getCalls-secretReads)
		}
		err := RunCheck(command, runtime)
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

func TestCheckEvaluationClientLookupDistinguishesMissingClient(t *testing.T) {
	evaluation := checkEvaluation{clients: []evaluatedClient{{client: configuration.ClientClaude}}}
	if client, ok := evaluation.client(configuration.ClientClaude); !ok || client.client != configuration.ClientClaude {
		t.Fatalf("Claude client = %#v, %v", client, ok)
	}
	if client, ok := evaluation.client(configuration.ClientCodex); ok || client != (evaluatedClient{}) {
		t.Fatalf("missing Codex client = %#v, %v", client, ok)
	}
}

func TestCheckReadsEachEnabledRouteCredentialOnce(t *testing.T) {
	runtime, cfg, _ := configuredReadinessRuntime(t)
	store := &observingSecretStore{value: "token"}
	runtime.Secrets = store
	configureClaudeExecutable(t, &runtime, &cfg)
	synchronizeClaudeSettings(t, runtime, cfg)
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.SetClientActivation(configuration.ClientCodex, true, "codex", []string{target})
	clientRuntime, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	clientRuntime.CredentialCommand = runtime.Executable
	if err := codex.SyncConfig(target, clientRuntime); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	runtime.HTTP = roundTripFunc(successfulReadinessResponse)
	command := NewCheckCommand(runtime)
	command.SetContext(t.Context())
	if evaluation := evaluateCheck(command, runtime, cfg); !evaluation.ok() {
		t.Fatalf("ready clients failed evaluation: %+v", evaluation.clients)
	}
	if store.existsCalls != 0 || store.getCalls != len([]string{configuration.ClientClaude, configuration.ClientCodex}) {
		t.Fatalf("exists calls=%d get calls=%d", store.existsCalls, store.getCalls)
	}
}

func TestCheckAdmitsProjectionBeforeReadingCredentials(t *testing.T) {
	for _, client := range []string{configuration.ClientClaude, configuration.ClientCodex} {
		for _, jsonMode := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/json=%t", client, jsonMode), func(t *testing.T) {
				runtime, cfg, output := configuredReadinessRuntime(t)
				runtime.Version = "1.0.0"
				problemDetail, problemAction := "", ""
				runtime.Problem = func(_, evidence, _, action string, cause error) error {
					problemDetail, problemAction = evidence, action
					return cause
				}
				cfg.SetClientActivation(client, true, "", nil)
				if err := runtime.Config.Save(cfg); err != nil {
					t.Fatal(err)
				}
				store := &observingSecretStore{getErr: errors.New("secret read before projection admission")}
				runtime.Secrets = store
				runtime.HTTP = roundTripFunc(func(*http.Request) (*http.Response, error) {
					t.Fatal("invalid projection must not authenticate an endpoint")
					return nil, nil
				})
				command := NewCheckCommand(runtime)
				if jsonMode {
					command.SetArgs([]string{"--json"})
				} else {
					command.SetArgs([]string{})
				}
				if err := executeCommand(command); err == nil {
					t.Fatal("invalid projection was accepted")
				}
				if store.getCalls != 0 {
					t.Fatalf("invalid projection read secret values %d times", store.getCalls)
				}
				if jsonMode {
					var result checkJSON
					if err := json.Unmarshal(output.Bytes(), &result); err != nil {
						t.Fatal(err)
					}
					problemDetail, problemAction = result.Clients[client].Detail, result.Clients[client].NextAction
					if state := result.Clients[client]; state.State != domainreadiness.Invalid || state.NextAction != problemAction || state.Detail != problemDetail {
						t.Fatalf("client status = %+v", state)
					}
				}
				if !strings.Contains(problemDetail, "executable is not configured") || problemAction != "aigw repair" {
					t.Fatalf("projection recovery was lost: detail=%q action=%q", problemDetail, problemAction)
				}
			})
		}
	}
}

func TestCheckHonorsClientNativeAuthenticationOwnership(t *testing.T) {
	runtime, cfg, buffer := configuredReadinessRuntime(t)
	runtime.Version = "1.0.0"
	binding := cfg.Clients[configuration.ClientCodex]
	binding.ModelProvider = "amazon-bedrock"
	binding.Authentication = configuration.AuthenticationClientNative
	delete(cfg.Clients, configuration.ClientClaude)
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	binding.Enabled = true
	binding.Executable = "codex"
	binding.Targets = []string{target}
	cfg.Clients[configuration.ClientCodex] = binding
	clientRuntime, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := codex.SyncConfig(target, clientRuntime); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	store := &observingSecretStore{getErr: errors.New("client-native credential access")}
	runtime.Secrets = store
	requests := 0
	runtime.HTTP = roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		return nil, errors.New("client-native endpoint probe")
	})
	command := &cobra.Command{}
	command.SetContext(context.Background())

	if err := RunCheck(command, runtime); err != nil {
		t.Fatalf("client-native check error = %v", err)
	}
	if store.getCalls != 0 || store.existsCalls != 0 || requests != 0 {
		t.Fatalf("client-native check used AIGW authentication capabilities: get=%d exists=%d HTTP=%d", store.getCalls, store.existsCalls, requests)
	}
	human := buffer.String()
	for _, want := range []string{"Local projection checked", "Client-owned authentication", "aigw verify --for codex", "All enabled client checks passed", "Model inference was not verified", "Real-client execution was not verified"} {
		if !strings.Contains(strings.ToLower(human), strings.ToLower(want)) {
			t.Fatalf("client-native check output = %q, want %q", human, want)
		}
	}
	for _, forbidden := range []string{"Every enabled client route is healthy", "remote authentication succeeded", "model request succeeded"} {
		if strings.Contains(human, forbidden) {
			t.Fatalf("client-native check output = %q, contains %q", human, forbidden)
		}
	}

	buffer.Reset()
	if err := runJSONCheck(command, runtime); err != nil {
		t.Fatalf("client-native check --json error = %v", err)
	}
	var machine struct {
		OK      bool                                  `json:"ok"`
		Clients map[string]map[string]json.RawMessage `json:"clients"`
	}
	if err := json.Unmarshal(buffer.Bytes(), &machine); err != nil {
		t.Fatal(err)
	}
	want := map[string]json.RawMessage{
		"state":               json.RawMessage(`"configured"`),
		"route":               json.RawMessage(`"codex"`),
		"account":             json.RawMessage(`"one"`),
		"detail":              json.RawMessage(`"Projection ready; client-owned authentication is not proven"`),
		"authentication":      json.RawMessage(`"client-native"`),
		"endpoint_configured": json.RawMessage(`true`),
		"projection_ready":    json.RawMessage(`true`),
		"check_passed":        json.RawMessage(`true`),
		"next_action":         json.RawMessage(`"aigw verify --for codex"`),
	}
	if !machine.OK || !reflect.DeepEqual(machine.Clients[configuration.ClientCodex], want) {
		t.Fatalf("client-native JSON check = %#v", machine)
	}
	if store.getCalls != 0 || store.existsCalls != 0 || requests != 0 {
		t.Fatalf("client-native JSON check used AIGW authentication capabilities: get=%d exists=%d HTTP=%d", store.getCalls, store.existsCalls, requests)
	}
}

func TestCheckJSONReportsOutputFailure(t *testing.T) {
	runtime, cfg, _ := configuredReadinessRuntime(t)
	runtime.Version = "1.0.0"
	cfg.Clients = map[string]configuration.ClientBinding{}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	want := errors.New("output failed")
	runtime.Out = failingOutputWriter{err: want}
	command := NewCheckCommand(runtime)
	command.SetArgs([]string{"--json"})
	if err := executeCommand(command); !errors.Is(err, want) {
		t.Fatalf("check --json output error = %v", err)
	}
}

func TestRunCheckCoversClientResolutionAndProjectionFailures(t *testing.T) {
	t.Run("no enabled clients", func(t *testing.T) {
		runtime, _, _ := configuredReadinessRuntime(t)
		runtime.Version = "1.0.0"
		if err := RunCheck(&cobra.Command{}, runtime); err == nil || !strings.Contains(err.Error(), "no enabled Client Bindings") {
			t.Fatalf("RunCheck() error = %v", err)
		}
	})

	t.Run("token lookup failure", func(t *testing.T) {
		runtime, cfg, _ := configuredReadinessRuntime(t)
		runtime.Version = "1.0.0"
		want := errors.New("credential backend unavailable")
		runtime.Secrets = presentFailingSecretStore{err: want}
		configureClaudeExecutable(t, &runtime, &cfg)
		synchronizeClaudeSettings(t, runtime, cfg)
		if err := runtime.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		if err := RunCheck(&cobra.Command{}, runtime); !errors.Is(err, want) {
			t.Fatalf("RunCheck() error = %v, want %v", err, want)
		}
	})

	t.Run("Codex projection drift", func(t *testing.T) {
		runtime, cfg, _ := configuredReadinessRuntime(t)
		runtime.Version = "1.0.0"
		if err := runtime.Secrets.Set("one", "token"); err != nil {
			t.Fatal(err)
		}
		binding := cfg.Clients[configuration.ClientCodex]
		binding.Enabled = true
		binding.Executable = "/opt/codex"
		binding.Targets = []string{filepath.Join(t.TempDir(), "missing.toml")}
		cfg.Clients[configuration.ClientCodex] = binding
		if err := runtime.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		if err := RunCheck(&cobra.Command{}, runtime); err == nil || !strings.Contains(err.Error(), "projection not ready") {
			t.Fatalf("RunCheck() error = %v", err)
		}
	})
}

func TestCheckEndpointReadinessIsIndependentOfDiagnosticCredentials(t *testing.T) {
	runtime, cfg, buffer := configuredReadinessRuntime(t)
	runtime.Version = "1.0.0"
	if err := runtime.Secrets.Set("one", "token"); err != nil {
		t.Fatal(err)
	}
	store := &failingAccountObservationStore{err: errors.New("credential metadata unavailable")}
	runtime.Accounts = store
	configureClaudeExecutable(t, &runtime, &cfg)
	synchronizeClaudeSettings(t, runtime, cfg)
	providerAccount := cfg.Accounts["one"]
	providerAccount.AccountProbe = &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://probe.example.test"}
	cfg.Accounts["one"] = providerAccount
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	runtime.HTTP = roundTripFunc(successfulReadinessResponse)
	for _, mode := range []string{"human", "json"} {
		t.Run(mode, func(t *testing.T) {
			store.reads = 0
			buffer.Reset()
			command := NewCheckCommand(runtime)
			if mode == "json" {
				command.SetArgs([]string{"--json"})
			} else {
				command.SetArgs([]string{})
			}
			if err := executeCommand(command); err != nil {
				t.Fatalf("healthy endpoint depends on optional diagnostics: %v", err)
			}
			if store.reads != 0 {
				t.Fatalf("endpoint check read diagnostic credentials %d times", store.reads)
			}
			if mode == "human" {
				if !strings.Contains(buffer.String(), "All enabled client checks passed") {
					t.Fatalf("human readiness verdict: %s", buffer.String())
				}
				return
			}
			var result checkJSON
			if err := json.Unmarshal(buffer.Bytes(), &result); err != nil || !result.OK {
				t.Fatalf("JSON readiness verdict: %#v, %v", result, err)
			}
		})
	}
}
