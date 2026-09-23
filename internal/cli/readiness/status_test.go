package readiness

import (
	"aigw-cli/internal/cli/invocation"
	clientdomain "aigw-cli/internal/client"
	configuration "aigw-cli/internal/configuration"
	domainreadiness "aigw-cli/internal/readiness"
	"aigw-cli/internal/secrets"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strings"
	"testing"
)

func TestRunStatusCoversSelectionDiagnosticsAndReadyNextActions(t *testing.T) {
	runtime, cfg, buffer := configuredReadinessRuntime(t)
	originalInspect := inspectAdapter
	t.Cleanup(func() { inspectAdapter = originalInspect })
	inspectAdapter = func(context.Context, invocation.Context, configuration.Config, string, configuration.Runtime) clientdomain.Status {
		return clientdomain.Status{Ready: true}
	}
	if err := runtime.Secrets.Set("one", "token"); err != nil {
		t.Fatal(err)
	}
	for _, client := range []string{configuration.ClientClaude, configuration.ClientCodex} {
		cfg.SetClientActivation(client, true, "", nil)
	}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := RunStatus(runtime, false); err != nil {
		t.Fatal(err)
	}
	if got := buffer.String(); !strings.Contains(got, "aigw check") {
		t.Fatalf("ready status = %q", got)
	}

	accountConfig := cfg.Accounts["one"]
	accountConfig.AccountProbe = &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://probe.example.test"}
	cfg.Accounts["one"] = accountConfig
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	buffer.Reset()
	if err := RunStatus(runtime, false); err != nil {
		t.Fatal(err)
	}
	if got := buffer.String(); !strings.Contains(got, "aigw account diagnostics enable one") {
		t.Fatalf("missing diagnostic credential status = %q", got)
	}
	if err := runtime.Accounts.Set("one", secrets.DiagnosticCredential{SystemToken: "system", UserID: "user"}); err != nil {
		t.Fatal(err)
	}
	buffer.Reset()
	if err := RunStatus(runtime, false); err != nil {
		t.Fatal(err)
	}
	if got := buffer.String(); !strings.Contains(got, "Precise balance enabled") {
		t.Fatalf("enabled diagnostic status = %q", got)
	}

	cfg.Routes["codex-only"] = configuration.Route{
		Label: "Codex only", Purpose: "Selection", Account: "one", Model: "gpt-test",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}},
	}
	delete(cfg.Clients, configuration.ClientCodex)
	cfg.SetRecommendedRoute(configuration.ClientCodex, "codex-only")
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	buffer.Reset()
	if err := RunStatus(runtime, false); err != nil {
		t.Fatal(err)
	}
	if got := buffer.String(); !strings.Contains(got, "aigw use --for codex codex-only") {
		t.Fatalf("route selection status = %q", got)
	}
}

func TestStatusStatesClaudeDesktopQualifiedModesAndPlatforms(t *testing.T) {
	runtime, _, buffer := configuredReadinessRuntime(t)

	if err := RunStatus(runtime, true); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Clients map[string]struct {
			QualifiedModes     []string `json:"qualified_modes"`
			QualifiedPlatforms []string `json:"qualified_platforms"`
		} `json:"clients"`
	}
	if err := json.Unmarshal(buffer.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	desktop := result.Clients[configuration.ClientClaudeDesktop]
	if !slices.Equal(desktop.QualifiedModes, []string{"Chat", "Cowork", "Code"}) {
		t.Fatalf("qualified modes = %#v", desktop.QualifiedModes)
	}
	if !slices.Equal(desktop.QualifiedPlatforms, []string{"macOS"}) {
		t.Fatalf("qualified platforms = %#v", desktop.QualifiedPlatforms)
	}

	buffer.Reset()
	if err := RunStatus(runtime, false); err != nil {
		t.Fatal(err)
	}
	if got := buffer.String(); !strings.Contains(got, "Qualified: Chat, Cowork, Code · macOS") {
		t.Fatalf("human status = %q", got)
	}
}

func TestRunStatusDescribesTransportAndOptionalProviderDiagnostics(t *testing.T) {
	for _, test := range []struct {
		name       string
		probe      *configuration.AccountProbe
		credential bool
		want       string
	}{
		{name: "no probe", want: "Provider does not expose a balance probe"},
		{name: "unsupported probe", probe: &configuration.AccountProbe{Kind: "future", BaseURL: "https://probe.test"}, want: "Provider diagnostics unavailable in this version"},
		{name: "missing probe credential", probe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://probe.test"}, want: "Precise balance disabled"},
		{name: "available probe credential", probe: &configuration.AccountProbe{Kind: "dmxapi", BaseURL: "https://probe.test"}, credential: true, want: "Precise balance enabled"},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime, cfg, buffer := configuredReadinessRuntime(t)
			providerAccount := cfg.Accounts["one"]
			providerAccount.Endpoints.OpenAIResponses = "http://127.0.0.1:1234/v1"
			providerAccount.AccountProbe = test.probe
			cfg.Accounts["one"] = providerAccount
			if err := runtime.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := runtime.Secrets.Set("one", "token"); err != nil {
				t.Fatal(err)
			}
			if test.credential {
				if err := runtime.Accounts.Set("one", secrets.DiagnosticCredential{SystemToken: "system", UserID: "user"}); err != nil {
					t.Fatal(err)
				}
			}
			if err := RunStatus(runtime, false); err != nil {
				t.Fatal(err)
			}
			output := buffer.String()
			if !strings.Contains(output, test.want) || !strings.Contains(output, "Loopback endpoint") {
				t.Fatalf("output = %q", output)
			}
		})
	}
}

func TestStatusDescribesEachLoopbackRouteWithoutInferringServiceIdentity(t *testing.T) {
	runtime, cfg, buffer := configuredReadinessRuntime(t)
	account := cfg.Accounts["one"]
	account.Endpoints = configuration.Endpoints{Anthropic: "http://127.0.0.2:4567", OpenAIResponses: "http://[::ffff:127.0.0.2]:4568/v1"}
	cfg.Accounts["one"] = account
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(runtime.Config.Path())
	if err != nil {
		t.Fatal(err)
	}
	store := &observingSecretStore{value: "token"}
	runtime.Secrets = store
	if err := RunStatus(runtime, false); err != nil {
		t.Fatal(err)
	}
	_, transport, found := strings.Cut(buffer.String(), "Transport\n")
	if !found {
		t.Fatalf("transport section missing: %s", buffer.String())
	}
	transport, _, _ = strings.Cut(transport, "Optional diagnostics\n")
	for _, want := range []string{"Claude", "Codex", "Endpoint runtime identity and availability are not inferred from the address"} {
		if !strings.Contains(transport, want) {
			t.Errorf("transport lacks %q: %s", want, transport)
		}
	}
	if strings.Count(transport, "Loopback endpoint") != 2 {
		t.Errorf("expected both endpoint observations: %s", transport)
	}
	buffer.Reset()
	if err := RunStatus(runtime, true); err != nil {
		t.Fatal(err)
	}
	var status statusOutput
	if err := json.Unmarshal(buffer.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	for _, client := range []string{configuration.ClientClaude, configuration.ClientCodex} {
		if status.Clients[client].Transport != "external_loopback" {
			t.Errorf("client %s transport = %q", client, status.Clients[client].Transport)
		}
	}
	after, err := os.ReadFile(runtime.Config.Path())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) || store.getCalls != 0 {
		t.Fatalf("status changed configuration or read secrets: reads=%d", store.getCalls)
	}
}

func TestRunStatusJSONNotConfiguredAndLoadErrors(t *testing.T) {
	emptyPath := filepath.Join(t.TempDir(), "configuration.toml")
	emptyOut := &bytes.Buffer{}
	emptyRuntime := invocation.Context{Config: configuration.NewStore(emptyPath), Out: emptyOut, RenderOut: emptyOut, Secrets: secrets.NewMemoryStore()}
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
	if !strings.Contains(emptyOut.String(), `"routes": 0`) {
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
	runtime, _, buffer := configuredReadinessRuntime(t)
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
	if err := json.Unmarshal(buffer.Bytes(), &result); err != nil {
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
	runtime, _, buffer := configuredReadinessRuntime(t)
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
	if err := json.Unmarshal(buffer.Bytes(), &result); err != nil {
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
	runtime := invocation.Context{Out: out, RenderOut: out, Width: 120}
	cfg := configuration.NewConfig()
	cfg.Routes["available"] = configuration.Route{Label: "Available"}
	clients := map[string]clientStatus{}
	for _, client := range []string{configuration.ClientClaude, configuration.ClientCodex} {
		state := domainreadiness.Client{State: domainreadiness.Invalid}
		clients[client] = clientStatus{Client: state}
	}

	renderStatus(runtime, cfg, statusOutput{Clients: clients})
	got := out.String()
	if !strings.Contains(got, "aigw repair") || !strings.Contains(got, "No selected account") {
		t.Fatalf("status fallback = %q", got)
	}
}

func TestRenderClientStatusCoversCanonicalStates(t *testing.T) {
	runtime := invocation.Context{Out: &bytes.Buffer{}, Width: 120}
	for _, state := range []domainreadiness.State{
		domainreadiness.EndpointChecked,
		domainreadiness.Configured,
		domainreadiness.Deferred,
		domainreadiness.Invalid,
	} {
		clientID := string(state)
		attention, _ := renderClientStatus(
			invocation.Renderer(runtime),
			statusOutput{Clients: map[string]clientStatus{clientID: {Client: domainreadiness.Client{State: state}}}},
			[]string{clientID},
		)
		if attention != (state == domainreadiness.Invalid) {
			t.Fatalf("state %s attention = %v", state, attention)
		}
	}
}

func TestStatusReportsSelectedUnknownRoute(t *testing.T) {
	runtime, cfg, _ := configuredReadinessRuntime(t)
	cfg.SetSelectedRoute(configuration.ClientClaude, "missing")
	state := inspectStatusClients(runtime, cfg)[configuration.ClientClaude]
	if state.State != domainreadiness.Invalid || state.Route != "missing" || !strings.Contains(state.Detail, `unknown route "missing"`) {
		t.Fatalf("Claude status = %#v", state)
	}
}

func TestStatusObservesCredentialsWithoutReadingValues(t *testing.T) {
	runtime, cfg, _ := configuredReadinessRuntime(t)
	store := &observingSecretStore{value: "must-not-be-read"}
	runtime.Secrets = store
	originalInspect := inspectAdapter
	t.Cleanup(func() { inspectAdapter = originalInspect })
	inspectAdapter = func(context.Context, invocation.Context, configuration.Config, string, configuration.Runtime) clientdomain.Status {
		return clientdomain.Status{Ready: true}
	}

	collectStatus(runtime, cfg)
	if store.existsCalls != len([]string{configuration.ClientClaude, configuration.ClientCodex}) || store.getCalls != 0 {
		t.Fatalf("exists calls=%d get calls=%d", store.existsCalls, store.getCalls)
	}
}

func TestStatusHonorsClientNativeAuthenticationOwnership(t *testing.T) {
	runtime, cfg, buffer := configuredReadinessRuntime(t)
	codexBinding := cfg.Clients[configuration.ClientCodex]
	codexBinding.Enabled = true
	codexBinding.ModelProvider = "amazon-bedrock"
	codexBinding.Authentication = configuration.AuthenticationClientNative
	cfg.Clients[configuration.ClientCodex] = codexBinding
	claudeBinding := cfg.Clients[configuration.ClientClaude]
	claudeBinding.Enabled = true
	cfg.Clients[configuration.ClientClaude] = claudeBinding
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	store := &observingSecretStore{value: "account-token"}
	runtime.Secrets = store
	originalInspect := inspectAdapter
	t.Cleanup(func() { inspectAdapter = originalInspect })
	inspectAdapter = func(context.Context, invocation.Context, configuration.Config, string, configuration.Runtime) clientdomain.Status {
		return clientdomain.Status{Ready: true}
	}

	if err := RunStatus(runtime, false); err != nil {
		t.Fatal(err)
	}
	human := buffer.String()
	if store.existsCalls != 1 || store.getCalls != 0 {
		t.Fatalf("status credential observations = exists %d, get %d; want only the Claude Account-Token lookup", store.existsCalls, store.getCalls)
	}
	for _, want := range []string{"Client-owned authentication", "aigw verify --for codex"} {
		if !strings.Contains(strings.ToLower(human), strings.ToLower(want)) {
			t.Fatalf("client-native status = %q, want %q", human, want)
		}
	}
	for _, unwanted := range []string{"Token", "aigw rotate", "aigw client auth"} {
		if strings.Contains(human, unwanted) {
			t.Fatalf("client-native status = %q, contains %q", human, unwanted)
		}
	}

	buffer.Reset()
	if err := RunStatus(runtime, true); err != nil {
		t.Fatal(err)
	}
	machine := buffer.String()
	if !strings.Contains(machine, `"authentication": "client-native"`) || strings.Contains(machine, `"secret_available"`) {
		t.Fatalf("client-native JSON status = %q", machine)
	}
}

func TestStatusClassifiesCredentialObservationFailureWithoutReadingValues(t *testing.T) {
	runtime, cfg, buffer := configuredReadinessRuntime(t)
	want := errors.New("credential metadata unavailable")
	store := &observingSecretStore{existsErr: want}
	runtime.Secrets = store
	result := collectStatus(runtime, cfg)
	for _, client := range []string{configuration.ClientClaude, configuration.ClientCodex} {
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
	if !strings.Contains(buffer.String(), "Unavailable") || !strings.Contains(buffer.String(), "aigw doctor") {
		t.Fatalf("RunStatus output = %q", buffer.String())
	}
}

func TestStatusReportsProjectionReadinessWithoutClaimingAuthentication(t *testing.T) {
	runtime, cfg, buffer := configuredReadinessRuntime(t)
	if err := runtime.Secrets.Set("one", "token"); err != nil {
		t.Fatal(err)
	}
	cfg.SetClientActivation(configuration.ClientCodex, true, "codex", nil)
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	originalInspect := inspectAdapter
	t.Cleanup(func() { inspectAdapter = originalInspect })
	inspectAdapter = func(context.Context, invocation.Context, configuration.Config, string, configuration.Runtime) clientdomain.Status {
		return clientdomain.Status{Ready: true}
	}
	if err := RunStatus(runtime, true); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Clients map[string]struct {
			ProjectionReady bool   `json:"projection_ready"`
			Authentication  string `json:"authentication"`
		} `json:"clients"`
	}
	if err := json.Unmarshal(buffer.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	observed := result.Clients[configuration.ClientCodex]
	if !observed.ProjectionReady || observed.Authentication != string(configuration.AuthenticationAccountToken) {
		t.Fatalf("local projection observation = %#v", observed)
	}
}
