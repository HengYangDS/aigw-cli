package readiness

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"aigw-cli/internal/account"
	"aigw-cli/internal/cli/invocation"
	clientdomain "aigw-cli/internal/client"
	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
	domainreadiness "aigw-cli/internal/readiness"
	"aigw-cli/internal/secrets"

	"github.com/spf13/cobra"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) Do(request *http.Request) (*http.Response, error) { return fn(request) }

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

type failingAccountObservationStore struct{ err error }

func (store failingAccountObservationStore) Get(string) (account.Credential, error) {
	return account.Credential{}, store.err
}
func (failingAccountObservationStore) Set(string, account.Credential) error { return nil }
func (failingAccountObservationStore) Delete(string) error                  { return nil }
func (store failingAccountObservationStore) Exists(string) (bool, error)    { return false, store.err }

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

func configuredReadinessRuntime(t *testing.T) (invocation.Context, configuration.Config) {
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
		Config: store, Secrets: secrets.NewMemoryStore(), Accounts: account.NewMemoryStore(),
		Out: out, RenderOut: out, Width: 120,
	}
	return runtime, cfg
}

func output(runtime invocation.Context) string { return runtime.Out.(*bytes.Buffer).String() }

func executeCommand(command *cobra.Command) error {
	command.SetErr(io.Discard)
	command.SilenceErrors = true
	command.SilenceUsage = true
	return command.Execute()
}

func TestRunStatusCoversSelectionDiagnosticsAndReadyNextActions(t *testing.T) {
	runtime, cfg := configuredReadinessRuntime(t)
	originalInspect := inspectAdapter
	t.Cleanup(func() { inspectAdapter = originalInspect })
	inspectAdapter = func(context.Context, invocation.Context, configuration.Config, string, configuration.Runtime, clientdomain.InspectionOptions) clientdomain.Status {
		return clientdomain.Status{Ready: true, NativeAuthentication: "present"}
	}
	if err := runtime.Secrets.Set("one", "token"); err != nil {
		t.Fatal(err)
	}
	for _, client := range configuration.AdmittedClientIDs() {
		cfg.Adapters[client] = configuration.AdapterConfig{Enabled: true}
	}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := RunStatus(runtime, false); err != nil {
		t.Fatal(err)
	}
	if got := output(runtime); !strings.Contains(got, "aigw check") {
		t.Fatalf("ready status = %q", got)
	}

	accountConfig := cfg.Accounts["one"]
	accountConfig.AccountProbe = &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://probe.example.test"}
	cfg.Accounts["one"] = accountConfig
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	runtime.Out.(*bytes.Buffer).Reset()
	if err := RunStatus(runtime, false); err != nil {
		t.Fatal(err)
	}
	if got := output(runtime); !strings.Contains(got, "aigw account connect one") {
		t.Fatalf("missing diagnostic credential status = %q", got)
	}
	if err := runtime.Accounts.Set("one", account.Credential{SystemToken: "system", UserID: "user"}); err != nil {
		t.Fatal(err)
	}
	runtime.Out.(*bytes.Buffer).Reset()
	if err := RunStatus(runtime, false); err != nil {
		t.Fatal(err)
	}
	if got := output(runtime); !strings.Contains(got, "Precise balance enabled") {
		t.Fatalf("enabled diagnostic status = %q", got)
	}

	cfg.Profiles["codex-only"] = configuration.Profile{Label: "Codex only", Purpose: "Selection", Account: "one", Client: configuration.ClientCodex, Model: "gpt-test"}
	delete(cfg.Routes, configuration.ClientCodex)
	cfg.Profiles["claude"] = configuration.Profile{Label: "Claude only", Account: "one", Client: configuration.ClientClaude, Model: "claude-test"}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	runtime.Out.(*bytes.Buffer).Reset()
	if err := RunStatus(runtime, false); err != nil {
		t.Fatal(err)
	}
	if got := output(runtime); !strings.Contains(got, "aigw use codex") {
		t.Fatalf("route selection status = %q", got)
	}
}

func TestRunStatusJSONNotConfiguredAndLoadErrors(t *testing.T) {
	emptyPath := filepath.Join(t.TempDir(), "configuration.toml")
	emptyOut := &bytes.Buffer{}
	emptyRuntime := invocation.Context{Config: configuration.NewStore(emptyPath), Out: emptyOut, RenderOut: emptyOut, Secrets: secrets.NewMemoryStore(), Accounts: account.NewMemoryStore()}
	if err := RunStatus(emptyRuntime, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(emptyOut.String(), "aigw setup") {
		t.Fatalf("empty status = %q", emptyOut.String())
	}
	emptyOut.Reset()
	if err := RunStatus(emptyRuntime, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(emptyOut.String(), `"profiles": 0`) {
		t.Fatalf("json status = %q", emptyOut.String())
	}

	badPath := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(badPath, []byte("not = [toml"), 0o600); err != nil {
		t.Fatal(err)
	}
	badRuntime := invocation.Context{Config: configuration.NewStore(badPath), Out: io.Discard}
	if err := RunStatus(badRuntime, false); err == nil {
		t.Fatal("malformed configuration was accepted")
	}
}

func TestStatusJSONExposesAutomaticCredentialBackendWithoutPersistingSelection(t *testing.T) {
	runtime, _ := configuredReadinessRuntime(t)
	root := filepath.Join(t.TempDir(), "secrets")
	store, err := secrets.Select(secrets.Selection{
		GOOS:         goruntime.GOOS,
		Root:         root,
		KeyringProbe: func(secrets.Store) error { return errors.New("service unavailable") },
	})
	if err != nil {
		t.Fatal(err)
	}
	runtime.Secrets = store

	if err := RunStatus(runtime, true); err != nil {
		t.Fatal(err)
	}
	var result struct {
		CredentialBackend secrets.BackendSelection `json:"credential_backend"`
	}
	if err := json.Unmarshal([]byte(output(runtime)), &result); err != nil {
		t.Fatal(err)
	}
	want := secrets.BackendSelection{
		Kind:         "file",
		Availability: "available",
		Mutability:   "read_write",
		Persistence:  "deferred",
	}
	if result.CredentialBackend != want {
		t.Fatalf("credential backend = %#v, want %#v", result.CredentialBackend, want)
	}
	if _, err := os.Stat(filepath.Join(root, "backend")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("status persisted automatic selection: %v", err)
	}
}

func TestStatusJSONReportsCredentialBackendInspectionFailure(t *testing.T) {
	runtime, _ := configuredReadinessRuntime(t)
	root := filepath.Join(t.TempDir(), "secrets")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "backend"), []byte("retired\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := secrets.Select(secrets.Selection{GOOS: goruntime.GOOS, Root: root})
	if err != nil {
		t.Fatal(err)
	}
	runtime.Secrets = store

	if err := RunStatus(runtime, true); err != nil {
		t.Fatal(err)
	}
	var result struct {
		CredentialBackend secrets.BackendSelection `json:"credential_backend"`
	}
	if err := json.Unmarshal([]byte(output(runtime)), &result); err != nil {
		t.Fatal(err)
	}
	want := secrets.BackendSelection{
		Kind:           "unknown",
		Availability:   "unavailable",
		Mutability:     "unknown",
		Persistence:    "unknown",
		RecoveryAction: domainreadiness.CredentialBackendRecovery,
	}
	if result.CredentialBackend != want {
		t.Fatalf("credential backend = %#v, want %#v", result.CredentialBackend, want)
	}
}

func TestStatusFallsBackToRepairForUnclassifiedAttention(t *testing.T) {
	out := &bytes.Buffer{}
	runtime := invocation.Context{Out: out, RenderOut: out, Width: 120, Accounts: account.NewMemoryStore()}
	cfg := configuration.NewConfig()
	cfg.Profiles["available"] = configuration.Profile{Label: "Available"}
	routes := map[string]routeStatus{}
	for _, client := range configuration.AdmittedClientIDs() {
		routes[client] = routeStatus{Client: domainreadiness.Client{State: domainreadiness.Invalid}}
	}

	renderStatus(runtime, cfg, statusOutput{Routes: routes})
	got := out.String()
	if !strings.Contains(got, "aigw repair") || !strings.Contains(got, "No selected account") {
		t.Fatalf("status fallback = %q", got)
	}
}

func TestRenderClientStatusCoversCanonicalStates(t *testing.T) {
	runtime := invocation.Context{Out: &bytes.Buffer{}, Width: 120}
	for _, state := range []domainreadiness.State{
		domainreadiness.Ready,
		domainreadiness.Configured,
		domainreadiness.Deferred,
		domainreadiness.Invalid,
	} {
		clientID := string(state)
		attention, _ := renderClientStatus(
			Renderer(runtime),
			statusOutput{Routes: map[string]routeStatus{clientID: {Client: domainreadiness.Client{State: state}}}},
			[]string{clientID},
		)
		if attention != (state == domainreadiness.Invalid) {
			t.Fatalf("state %s attention = %v", state, attention)
		}
	}
}

func TestStatusReportsSelectedUnknownProfile(t *testing.T) {
	runtime, cfg := configuredReadinessRuntime(t)
	cfg.Routes[configuration.ClientClaude] = "missing"
	state := inspectStatusClients(runtime, cfg)[configuration.ClientClaude]
	if state.State != domainreadiness.Invalid || state.Profile != "missing" || !strings.Contains(state.Detail, `unknown profile "missing"`) {
		t.Fatalf("Claude status = %#v", state)
	}
}

func TestStatusObservesCredentialsWithoutReadingValues(t *testing.T) {
	runtime, cfg := configuredReadinessRuntime(t)
	store := &observingSecretStore{value: "must-not-be-read"}
	runtime.Secrets = store
	originalInspect := inspectAdapter
	t.Cleanup(func() { inspectAdapter = originalInspect })
	inspectAdapter = func(context.Context, invocation.Context, configuration.Config, string, configuration.Runtime, clientdomain.InspectionOptions) clientdomain.Status {
		return clientdomain.Status{Ready: true}
	}

	collectStatus(runtime, cfg)
	if store.existsCalls != len(configuration.AdmittedClientIDs()) || store.getCalls != 0 {
		t.Fatalf("exists calls=%d get calls=%d", store.existsCalls, store.getCalls)
	}
}

func TestStatusHonorsClientNativeAuthenticationOwnership(t *testing.T) {
	runtime, cfg := configuredReadinessRuntime(t)
	profile := cfg.Profiles["codex"]
	profile.ModelProvider = "amazon-bedrock"
	profile.Authentication = configuration.AuthenticationClientNative
	cfg.Profiles["codex"] = profile
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	store := &observingSecretStore{value: "account-token"}
	runtime.Secrets = store
	originalInspect := inspectAdapter
	t.Cleanup(func() { inspectAdapter = originalInspect })
	inspectAdapter = func(context.Context, invocation.Context, configuration.Config, string, configuration.Runtime, clientdomain.InspectionOptions) clientdomain.Status {
		return clientdomain.Status{Ready: true, NativeAuthentication: "not_required"}
	}

	if err := RunStatus(runtime, false); err != nil {
		t.Fatal(err)
	}
	human := output(runtime)
	if store.existsCalls != 1 || store.getCalls != 0 {
		t.Fatalf("status credential observations = exists %d, get %d; want only the Claude Account-Token lookup", store.existsCalls, store.getCalls)
	}
	for _, want := range []string{"Client-owned authentication", "aigw verify --for codex"} {
		if !strings.Contains(strings.ToLower(human), strings.ToLower(want)) {
			t.Fatalf("client-native status = %q, want %q", human, want)
		}
	}
	for _, unwanted := range []string{"Token", "aigw rotate", "aigw adapter auth"} {
		if strings.Contains(human, unwanted) {
			t.Fatalf("client-native status = %q, contains %q", human, unwanted)
		}
	}

	runtime.Out.(*bytes.Buffer).Reset()
	if err := RunStatus(runtime, true); err != nil {
		t.Fatal(err)
	}
	machine := output(runtime)
	if !strings.Contains(machine, `"authentication": "client-native"`) || strings.Contains(machine, `"secret_available"`) {
		t.Fatalf("client-native JSON status = %q", machine)
	}
}

func TestStatusClassifiesCredentialObservationFailureWithoutReadingValues(t *testing.T) {
	runtime, cfg := configuredReadinessRuntime(t)
	want := errors.New("credential metadata unavailable")
	store := &observingSecretStore{existsErr: want}
	runtime.Secrets = store
	result := collectStatus(runtime, cfg)
	for _, client := range configuration.AdmittedClientIDs() {
		state := result.Clients[client]
		if state.State != domainreadiness.Unavailable || state.NextAction != "aigw doctor" || !strings.Contains(strings.ToLower(state.Detail), "credential metadata") {
			t.Fatalf("%s state = %#v", client, state)
		}
	}
	if store.getCalls != 0 {
		t.Fatalf("status read credential %d times after observation failed", store.getCalls)
	}
	if err := RunStatus(runtime, false); err != nil {
		t.Fatalf("RunStatus error = %v", err)
	}
	if !strings.Contains(output(runtime), "Unavailable") || !strings.Contains(output(runtime), "aigw doctor") {
		t.Fatalf("RunStatus output = %q", output(runtime))
	}
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
	runtime, cfg := configuredReadinessRuntime(t)
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
	runtime, cfg := configuredReadinessRuntime(t)
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
	human := output(runtime)
	for _, want := range []string{"Local readiness", "Client-owned authentication", "aigw verify --for codex"} {
		if !strings.Contains(strings.ToLower(human), strings.ToLower(want)) {
			t.Fatalf("client-native check output = %q, want %q", human, want)
		}
	}
	for _, forbidden := range []string{"Every enabled client route is healthy", "remote authentication succeeded", "model request succeeded"} {
		if strings.Contains(human, forbidden) {
			t.Fatalf("client-native check output = %q, contains %q", human, forbidden)
		}
	}

	runtime.Out.(*bytes.Buffer).Reset()
	if err := runJSONCheck(command, runtime); err != nil {
		t.Fatalf("client-native check --json error = %v", err)
	}
	var machine struct {
		OK     bool `json:"ok"`
		Routes map[string]struct {
			Authentication configuration.Authentication `json:"authentication"`
			Ready          bool                         `json:"ready"`
			Fix            string                       `json:"fix"`
			DiagnosticKind string                       `json:"diagnostic_kind"`
		} `json:"routes"`
	}
	if err := json.Unmarshal(runtime.Out.(*bytes.Buffer).Bytes(), &machine); err != nil {
		t.Fatal(err)
	}
	route := machine.Routes[configuration.ClientCodex]
	if !machine.OK || !route.Ready || route.Authentication != configuration.AuthenticationClientNative || route.Fix != "aigw verify --for codex" || route.DiagnosticKind != "" {
		t.Fatalf("client-native JSON check = %#v", machine)
	}
	if store.getCalls != 0 || store.existsCalls != 0 || requests != 0 {
		t.Fatalf("client-native JSON check used AIGW authentication capabilities: get=%d exists=%d HTTP=%d", store.getCalls, store.existsCalls, requests)
	}
}

func TestCheckJSONReportsOutputFailure(t *testing.T) {
	runtime, cfg := configuredReadinessRuntime(t)
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

func TestStatusSeparatesCodexProjectionFromNativeAuthentication(t *testing.T) {
	runtime, cfg := configuredReadinessRuntime(t)
	if err := runtime.Secrets.Set("one", "token"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "config.toml")
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{
		Enabled: true, Executable: "codex", Targets: []string{target},
	}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	originalInspect := inspectAdapter
	t.Cleanup(func() { inspectAdapter = originalInspect })
	inspectAdapter = func(_ context.Context, _ invocation.Context, _ configuration.Config, clientID string, _ configuration.Runtime, _ clientdomain.InspectionOptions) clientdomain.Status {
		status := clientdomain.Status{Ready: true}
		if clientID == configuration.ClientCodex {
			status.NativeAuthentication = "not_proven"
		}
		return status
	}

	if err := RunStatus(runtime, false); err != nil {
		t.Fatal(err)
	}
	got := output(runtime)
	if !strings.Contains(got, "Projection ready") || !strings.Contains(got, "Native authentication not proven") {
		t.Fatalf("status does not separate readiness facts: %q", got)
	}
	if !strings.Contains(got, "aigw adapter auth codex") || strings.Contains(got, "aigw repair") {
		t.Fatalf("status next action = %q", got)
	}

	runtime.Out.(*bytes.Buffer).Reset()
	if err := RunStatus(runtime, true); err != nil {
		t.Fatal(err)
	}
	jsonStatus := output(runtime)
	if !strings.Contains(jsonStatus, `"adapter_ready": true`) ||
		!strings.Contains(jsonStatus, `"native_authentication": "not_proven"`) {
		t.Fatalf("JSON readiness facts = %q", jsonStatus)
	}

	inspectAdapter = func(_ context.Context, _ invocation.Context, _ configuration.Config, clientID string, _ configuration.Runtime, _ clientdomain.InspectionOptions) clientdomain.Status {
		status := clientdomain.Status{Ready: true}
		if clientID == configuration.ClientCodex {
			status.NativeAuthentication = "present"
		}
		return status
	}
	runtime.Out.(*bytes.Buffer).Reset()
	if err := RunStatus(runtime, false); err != nil {
		t.Fatal(err)
	}
	if got := output(runtime); !strings.Contains(got, "Ready") || !strings.Contains(got, "aigw check") {
		t.Fatalf("proved authentication status = %q", got)
	}
	runtime.Out.(*bytes.Buffer).Reset()
	if err := RunStatus(runtime, true); err != nil {
		t.Fatal(err)
	}
	if got := output(runtime); !strings.Contains(got, `"native_authentication": "present"`) {
		t.Fatalf("proved native authentication status = %q", got)
	}

	profile := cfg.Profiles["codex"]
	profile.ModelProvider = "amazon-bedrock"
	cfg.Profiles["codex"] = profile
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	inspectAdapter = func(_ context.Context, _ invocation.Context, _ configuration.Config, clientID string, _ configuration.Runtime, _ clientdomain.InspectionOptions) clientdomain.Status {
		status := clientdomain.Status{Ready: true}
		if clientID == configuration.ClientCodex {
			status.NativeAuthentication = "not_required"
		}
		return status
	}
	runtime.Out.(*bytes.Buffer).Reset()
	if err := RunStatus(runtime, true); err != nil {
		t.Fatal(err)
	}
	if got := output(runtime); !strings.Contains(got, `"native_authentication": "not_required"`) {
		t.Fatalf("native provider authentication status = %q", got)
	}
}

func TestRunCheckCoversClientResolutionAndProjectionFailures(t *testing.T) {
	t.Run("no enabled clients", func(t *testing.T) {
		runtime, _ := configuredReadinessRuntime(t)
		runtime.Version = "1.0.0"
		if err := RunCheck(&cobra.Command{}, runtime); err != nil {
			t.Fatal(err)
		}
		if got := output(runtime); !strings.Contains(got, "no clients are enabled") {
			t.Fatalf("RunCheck() output = %q", got)
		}
	})

	t.Run("enabled client route mismatch", func(t *testing.T) {
		runtime, cfg := configuredReadinessRuntime(t)
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
		runtime, cfg := configuredReadinessRuntime(t)
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
		runtime, cfg := configuredReadinessRuntime(t)
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

func TestRunCheckShowsEnabledProviderDiagnostics(t *testing.T) {
	runtime, cfg := configuredReadinessRuntime(t)
	runtime.Version = "1.0.0"
	if err := runtime.Secrets.Set("one", "token"); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Accounts.Set("one", account.Credential{SystemToken: "system", UserID: "user"}); err != nil {
		t.Fatal(err)
	}
	claudeExecutable := filepath.Join(t.TempDir(), "claude")
	if goruntime.GOOS == "windows" {
		claudeExecutable += ".exe"
	}
	if err := os.WriteFile(claudeExecutable, []byte("fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: claudeExecutable}
	providerAccount := cfg.Accounts["one"]
	providerAccount.AccountProbe = &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://probe.example.test"}
	cfg.Accounts["one"] = providerAccount
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	runtime.HTTP = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Request: request}, nil
	})
	command := &cobra.Command{}
	command.SetContext(context.Background())
	if err := RunCheck(command, runtime); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"precise balance enabled"} {
		if !strings.Contains(strings.ToLower(output(runtime)), want) {
			t.Fatalf("health output lacks %q: %s", want, output(runtime))
		}
	}
}

func TestRunCheckReportsProviderDiagnosticCredentialObservationFailure(t *testing.T) {
	runtime, cfg := configuredReadinessRuntime(t)
	runtime.Version = "1.0.0"
	if err := runtime.Secrets.Set("one", "token"); err != nil {
		t.Fatal(err)
	}
	want := errors.New("credential metadata unavailable")
	runtime.Accounts = failingAccountObservationStore{err: want}
	claudeExecutable := filepath.Join(t.TempDir(), "claude")
	if goruntime.GOOS == "windows" {
		claudeExecutable += ".exe"
	}
	if err := os.WriteFile(claudeExecutable, []byte("fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: claudeExecutable}
	providerAccount := cfg.Accounts["one"]
	providerAccount.AccountProbe = &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://probe.example.test"}
	cfg.Accounts["one"] = providerAccount
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	runtime.HTTP = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Request: request}, nil
	})
	command := &cobra.Command{}
	command.SetContext(context.Background())
	if err := RunCheck(command, runtime); !errors.Is(err, want) {
		t.Fatalf("RunCheck() error = %v, want %v", err, want)
	}
}

func TestEndpointTestCommandCoversSuccessAndTransportFailures(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		runtime, _ := configuredReadinessRuntime(t)
		if err := runtime.Secrets.Set("one", "token"); err != nil {
			t.Fatal(err)
		}
		requests := 0
		runtime.HTTP = roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests++
			if strings.Contains(request.URL.Host, "claude") && request.Header.Get("X-Api-Key") != "token" {
				t.Fatalf("Claude credential header = %q", request.Header.Get("X-Api-Key"))
			}
			if strings.Contains(request.URL.Host, "codex") && request.Header.Get("Authorization") != "Bearer token" {
				t.Fatalf("Codex credential header = %q", request.Header.Get("Authorization"))
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok"))}, nil
		})
		command := NewTestCommand(runtime)
		if err := executeCommand(command); err != nil {
			t.Fatal(err)
		}
		if requests != 2 || !strings.Contains(output(runtime), "Connectivity test") {
			t.Fatalf("requests=%d output=%q", requests, output(runtime))
		}
	})

	tests := []struct {
		name         string
		status       int
		body         io.ReadCloser
		roundTripErr error
		want         string
	}{
		{name: "network", roundTripErr: errors.New("offline"), want: "unreachable"},
		{name: "authentication", status: http.StatusUnauthorized, body: io.NopCloser(strings.NewReader("denied")), want: "authentication was rejected"},
		{name: "unexpected status", status: http.StatusBadGateway, body: io.NopCloser(strings.NewReader("failed")), want: "HTTP 502"},
		{name: "read", status: http.StatusOK, body: io.NopCloser(failingReader{err: errors.New("read failed")}), want: "read Codex endpoint response"},
		{name: "close", status: http.StatusOK, body: closeErrorBody{Reader: strings.NewReader("ok")}, want: "close Codex endpoint response"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runtime, _ := configuredReadinessRuntime(t)
			if err := runtime.Secrets.Set("one", "token"); err != nil {
				t.Fatal(err)
			}
			runtime.HTTP = roundTripFunc(func(*http.Request) (*http.Response, error) {
				if test.roundTripErr != nil {
					return nil, test.roundTripErr
				}
				return &http.Response{StatusCode: test.status, Body: test.body}, nil
			})
			command := NewTestCommand(runtime)
			command.SetArgs([]string{"--for", "codex"})
			if err := executeCommand(command); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestEndpointTestRejectsClientNativeBeforeCredentialOrNetworkAccess(t *testing.T) {
	runtime, cfg := configuredReadinessRuntime(t)
	profile := cfg.Profiles["codex"]
	profile.ModelProvider = "amazon-bedrock"
	profile.Authentication = configuration.AuthenticationClientNative
	cfg.Profiles["codex"] = profile
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
	command := NewTestCommand(runtime)
	command.SetArgs([]string{"--for", configuration.ClientCodex})

	err := executeCommand(command)
	if err == nil || !strings.Contains(err.Error(), "aigw verify --for codex") {
		t.Fatalf("client-native endpoint test error = %v", err)
	}
	if store.getCalls != 0 || store.existsCalls != 0 || requests != 0 {
		t.Fatalf("client-native endpoint test used AIGW authentication capabilities: get=%d exists=%d HTTP=%d", store.getCalls, store.existsCalls, requests)
	}
}

func TestEndpointTestCommandCoversInputAndResolutionFailures(t *testing.T) {
	t.Run("profile infers client", func(t *testing.T) {
		runtime, _ := configuredReadinessRuntime(t)
		if err := runtime.Secrets.Set("one", "token"); err != nil {
			t.Fatal(err)
		}
		var requests int
		runtime.HTTP = roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests++
			if request.URL.Path != "/v1/models" {
				t.Fatalf("request path = %q", request.URL.Path)
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}"))}, nil
		})
		command := NewTestCommand(runtime)
		command.SetArgs([]string{"--profile", "codex"})
		if err := executeCommand(command); err != nil {
			t.Fatal(err)
		}
		if requests != 1 {
			t.Fatalf("requests = %d, want 1", requests)
		}
	})

	t.Run("unknown profile is rejected", func(t *testing.T) {
		runtime, _ := configuredReadinessRuntime(t)
		command := NewTestCommand(runtime)
		command.SetArgs([]string{"--profile", "missing"})
		if err := executeCommand(command); err == nil || !strings.Contains(err.Error(), "unknown profile") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("invalid client", func(t *testing.T) {
		runtime, _ := configuredReadinessRuntime(t)
		command := NewTestCommand(runtime)
		command.SetArgs([]string{"--for", "future"})
		if err := executeCommand(command); err == nil || !strings.Contains(err.Error(), "--for must be") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("not configured", func(t *testing.T) {
		problem := errors.New("structured problem")
		runtime := invocation.Context{Config: configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml")), Out: io.Discard, Problem: func(string, string, string, string, error) error { return problem }}
		if err := executeCommand(NewTestCommand(runtime)); !errors.Is(err, problem) {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("unknown profile", func(t *testing.T) {
		runtime, _ := configuredReadinessRuntime(t)
		command := NewTestCommand(runtime)
		command.SetArgs([]string{"--profile", "missing"})
		if err := executeCommand(command); err == nil || !strings.Contains(err.Error(), "unknown profile") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("missing token", func(t *testing.T) {
		runtime, _ := configuredReadinessRuntime(t)
		command := NewTestCommand(runtime)
		command.SetArgs([]string{"--for", "codex"})
		if err := executeCommand(command); err == nil || !strings.Contains(err.Error(), "Token for account") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("missing explicit endpoint", func(t *testing.T) {
		runtime, cfg := configuredReadinessRuntime(t)
		accountConfig := cfg.Accounts["one"]
		accountConfig.Endpoints.OpenAIResponses = ""
		cfg.Accounts["one"] = accountConfig
		if err := runtime.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		command := NewTestCommand(runtime)
		command.SetArgs([]string{"--for", "codex"})
		if err := executeCommand(command); err == nil || !strings.Contains(err.Error(), "no OpenAI Responses endpoint") {
			t.Fatalf("error = %v", err)
		}
	})

}

func TestReadinessTransportHelpers(t *testing.T) {
	if got := TransportStatus("%"); got.Kind != "" {
		t.Fatalf("invalid transport = %#v", got)
	}
	if got := TransportStatus("http://LOCALHOST:8791/v1"); got.Kind != "external_loopback" {
		t.Fatalf("loopback transport = %#v", got)
	}
	if got := TransportStatus("https://api.example.test/v1"); got.Kind != "" {
		t.Fatalf("remote transport = %#v", got)
	}
	buffer := &bytes.Buffer{}
	renderer := Renderer(invocation.Context{Out: buffer})
	renderer.Text("fallback writer")
	if !strings.Contains(buffer.String(), "fallback writer") {
		t.Fatalf("renderer output = %q", buffer.String())
	}
	command := NewStatusCommand(invocation.Context{Config: configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml")), Out: io.Discard, Secrets: secrets.NewMemoryStore(), Accounts: account.NewMemoryStore()})
	command.SetArgs([]string{"--json"})
	if err := executeCommand(command); err != nil {
		t.Fatal(err)
	}
}
