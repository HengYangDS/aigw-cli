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
	if command.Short != "Check routes, credentials, clients, and endpoints" {
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
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude", Account: "one", Client: configuration.ClientClaude, Model: "claude-test"}
	cfg.Profiles["codex"] = configuration.Profile{Label: "Codex", Account: "one", Client: configuration.ClientCodex, Model: "gpt-test"}
	cfg.Routes[configuration.ClientClaude] = "claude"
	cfg.Routes[configuration.ClientCodex] = "codex"
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
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: executable}
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
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Request: request}, nil
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
		route := status.Routes[configuration.ClientClaude]
		if route.AdapterReady != synchronized || secretStore.getCalls != secretReads {
			t.Errorf("synchronized=%t route=%+v secret reads=%d", synchronized, route, secretStore.getCalls-secretReads)
		}
		err := RunCheck(command, runtime)
		if synchronized {
			if err != nil || requests == 0 {
				t.Errorf("synchronized check: error=%v requests=%d", err, requests)
			}
		} else if err == nil || requests != 0 || !strings.Contains(buffer.String(), "aigw sync") {
			t.Errorf("unsynchronized check: error=%v requests=%d output=%s", err, requests, buffer.String())
		}
	}
}

func executeCommand(command *cobra.Command) error {
	command.SetErr(io.Discard)
	command.SilenceErrors = true
	command.SilenceUsage = true
	return command.Execute()
}

func TestCheckEvaluationRouteLookupDistinguishesMissingClient(t *testing.T) {
	evaluation := checkEvaluation{routes: []evaluatedRoute{{client: configuration.ClientClaude}}}
	if route, ok := evaluation.route(configuration.ClientClaude); !ok || route.client != configuration.ClientClaude {
		t.Fatalf("Claude route = %#v, %v", route, ok)
	}
	if route, ok := evaluation.route(configuration.ClientCodex); ok || route != (evaluatedRoute{}) {
		t.Fatalf("missing Codex route = %#v, %v", route, ok)
	}
}

func TestCheckReadsEachEnabledRouteCredentialOnce(t *testing.T) {
	runtime, cfg, _ := configuredReadinessRuntime(t)
	store := &observingSecretStore{value: "token"}
	runtime.Secrets = store
	for _, client := range configuration.AdmittedClientIDs() {
		cfg.Adapters[client] = configuration.AdapterConfig{Enabled: true}
	}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	command := NewCheckCommand(runtime)
	_ = evaluateCheck(command, runtime, cfg)
	if store.existsCalls != 0 || store.getCalls != len(configuration.AdmittedClientIDs()) {
		t.Fatalf("exists calls=%d get calls=%d", store.existsCalls, store.getCalls)
	}
}

func TestCheckHonorsClientNativeAuthenticationOwnership(t *testing.T) {
	runtime, cfg, buffer := configuredReadinessRuntime(t)
	runtime.Version = "1.0.0"
	profile := cfg.Profiles["codex"]
	profile.ModelProvider = "amazon-bedrock"
	profile.Authentication = configuration.AuthenticationClientNative
	cfg.Profiles["codex"] = profile
	delete(cfg.Routes, configuration.ClientClaude)
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{
		Enabled: true, Executable: "codex", Targets: []string{target},
	}
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
	for _, want := range []string{"Local projection checked", "Client-owned authentication", "aigw verify --for codex", "All enabled route checks passed", "Model inference and real-client execution were not verified"} {
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
		Routes  map[string]map[string]json.RawMessage `json:"routes"`
		Clients map[string]domainreadiness.Client     `json:"clients"`
	}
	if err := json.Unmarshal(buffer.Bytes(), &machine); err != nil {
		t.Fatal(err)
	}
	want := map[string]json.RawMessage{
		"client":              json.RawMessage(`"codex"`),
		"profile":             json.RawMessage(`"codex"`),
		"account":             json.RawMessage(`"one"`),
		"authentication":      json.RawMessage(`"client-native"`),
		"endpoint_configured": json.RawMessage(`true`),
		"adapter_ready":       json.RawMessage(`true`),
		"check_passed":        json.RawMessage(`true`),
		"next_action":         json.RawMessage(`"aigw verify --for codex"`),
	}
	if !machine.OK || !reflect.DeepEqual(machine.Routes[configuration.ClientCodex], want) || machine.Clients[configuration.ClientCodex].State != domainreadiness.Configured {
		t.Fatalf("client-native JSON check = %#v", machine)
	}
	if store.getCalls != 0 || store.existsCalls != 0 || requests != 0 {
		t.Fatalf("client-native JSON check used AIGW authentication capabilities: get=%d exists=%d HTTP=%d", store.getCalls, store.existsCalls, requests)
	}
}

func TestCheckJSONReportsOutputFailure(t *testing.T) {
	runtime, cfg, _ := configuredReadinessRuntime(t)
	runtime.Version = "1.0.0"
	cfg.Adapters = map[string]configuration.AdapterConfig{}
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
		runtime, _, buffer := configuredReadinessRuntime(t)
		runtime.Version = "1.0.0"
		if err := RunCheck(&cobra.Command{}, runtime); err != nil {
			t.Fatal(err)
		}
		if got := buffer.String(); !strings.Contains(got, "no clients are enabled") {
			t.Fatalf("RunCheck() output = %q", got)
		}
	})

	t.Run("enabled client route mismatch", func(t *testing.T) {
		runtime, cfg, _ := configuredReadinessRuntime(t)
		runtime.Version = "1.0.0"
		delete(cfg.Routes, configuration.ClientClaude)
		cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true}
		if err := runtime.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		if err := RunCheck(&cobra.Command{}, runtime); err == nil || !strings.Contains(err.Error(), `no route selected for client "claude"`) {
			t.Fatalf("RunCheck() error = %v", err)
		}
	})

	t.Run("token lookup failure", func(t *testing.T) {
		runtime, cfg, _ := configuredReadinessRuntime(t)
		runtime.Version = "1.0.0"
		want := errors.New("credential backend unavailable")
		runtime.Secrets = presentFailingSecretStore{err: want}
		cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true}
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
	runtime.HTTP = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Request: request}, nil
	})
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
				if !strings.Contains(buffer.String(), "All enabled route checks passed") {
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
