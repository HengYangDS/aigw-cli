package check

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	claudeclient "aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/app"
	"aigw-cli/internal/app/diagnostics"
	domainreadiness "aigw-cli/internal/app/readiness"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/cli/status"
	"aigw-cli/internal/domain"
)

func configuredCodexScopedCheck(t *testing.T) (invocation.Context, *bytes.Buffer) {
	t.Helper()
	runtime, cfg, output := configuredReadinessRuntime(t)
	delete(cfg.Clients, claudeclient.ID)
	runtime.Executable = filepath.Join(t.TempDir(), "aigw")
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.SetClientActivation(codex.ID, true, "codex", []string{target})
	selected, err := cfg.ResolveRuntime(codex.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	selected.CredentialCommand = runtime.Executable
	if err := codex.SyncConfig(target, selected); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Secrets.Set("one", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	return runtime, output
}

func TestCheckForLimitsCredentialsAndInferenceToOneEnabledClient(t *testing.T) {
	runtime, output := configuredCodexScopedCheck(t)
	cfg, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Accounts["other"] = domain.Account{Label: "Other", Endpoints: domain.Endpoints{Anthropic: "https://other.example.test"}}
	claude := cfg.Routes["claude"]
	claude.Account = "other"
	cfg.Routes["claude"] = claude
	cfg.SetSelectedRoute(claudeclient.ID, "claude", "")
	cfg.SetClientActivation(claudeclient.ID, true, filepath.Join(t.TempDir(), "missing-claude"), nil)
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	store := &observingSecretStore{value: "fixture-token"}
	runtime.Secrets = store
	requests := 0
	runtime.HTTP = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		if request.URL.Host != "codex.example.test" || request.URL.Path != "/v1/responses" {
			t.Fatalf("scoped check contacted %s", request.URL)
		}
		return successfulReadinessResponse(request)
	})
	command := NewCommand(runtime)
	command.SetArgs([]string{"--for", "codex", "--json"})
	if err := executeCommand(command); err != nil {
		t.Fatal(err)
	}
	var result checkDocument
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	checked := result.Clients[codex.ID]
	if !result.OK || result.EnabledClients != 1 || len(result.Clients) != 1 || checked.CheckPassed == nil || !*checked.CheckPassed || checked.DiagnosticScope != diagnostics.ScopeInference || requests != 1 {
		t.Fatalf("scoped result = %+v; requests=%d", result, requests)
	}
	if !slices.Equal(store.existsAccounts, []string{"one"}) || !slices.Equal(store.getAccounts, []string{"one"}) {
		t.Fatalf("scoped check observed unrelated credentials: exists=%v get=%v", store.existsAccounts, store.getAccounts)
	}

	output.Reset()
	command = NewCommand(runtime)
	command.SetArgs([]string{"--for", "codex"})
	if err := executeCommand(command); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Selected client check passed") || strings.Contains(output.String(), "Claude") {
		t.Fatalf("human scoped check = %q", output.String())
	}
}

func TestCheckForRejectsUnknownOrDisabledClientBeforeObservingCredentials(t *testing.T) {
	runtime, output := configuredCodexScopedCheck(t)
	store := &observingSecretStore{value: "fixture-token"}
	runtime.Secrets = store
	runtime.HTTP = roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("rejected scope must not contact an endpoint")
		return nil, nil
	})
	unknown := NewCommand(runtime)
	unknown.SetArgs([]string{"--for", "unknown", "--json"})
	if err := executeCommand(unknown); err == nil || !strings.Contains(err.Error(), "--for") {
		t.Fatalf("unknown client error = %v", err)
	}
	blank := NewCommand(runtime)
	blank.SetArgs([]string{"--for", " "})
	if err := executeCommand(blank); err == nil || !strings.Contains(err.Error(), "--for requires a non-empty client") {
		t.Fatalf("blank client error = %v", err)
	}
	output.Reset()
	disabled := NewCommand(runtime)
	disabled.SetArgs([]string{"--for", "claude", "--json"})
	if err := executeCommand(disabled); err == nil {
		t.Fatal("disabled client was checked")
	}
	var result checkDocument
	if err := json.Unmarshal(output.Bytes(), &result); err != nil || result.OK || !strings.Contains(result.Problem, "not enabled") || result.NextDetail != "aigw use --for claude <route>" || result.NextAction != "" {
		t.Fatalf("disabled client result = %+v, %v", result, err)
	}
	if store.getCalls != 0 || store.existsCalls != 0 {
		t.Fatalf("invalid scopes read credentials: get=%d exists=%d", store.getCalls, store.existsCalls)
	}
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
	providerAccount.AccountProbe = &domain.AccountProbe{Kind: "dmxapi", BaseURL: "https://probe.example.test"}
	cfg.Accounts["one"] = providerAccount
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	runtime.HTTP = roundTripFunc(successfulReadinessResponse)
	for _, mode := range []string{"human", "json"} {
		t.Run(mode, func(t *testing.T) {
			store.reads = 0
			buffer.Reset()
			command := NewCommand(runtime)
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
			var result checkDocument
			if err := json.Unmarshal(buffer.Bytes(), &result); err != nil || !result.OK {
				t.Fatalf("JSON readiness verdict: %#v, %v", result, err)
			}
		})
	}
}

func TestMixedInvalidAndDeferredClientsShareOneRecoveryAction(t *testing.T) {
	runtime, cfg, output := configuredReadinessRuntime(t)
	if err := runtime.Secrets.Set("one", "available-token"); err != nil {
		t.Fatal(err)
	}
	cfg.SetClientActivation(claudeclient.ID, true, "", nil)
	cfg.SetClientActivation(codex.ID, true, "/opt/codex", []string{filepath.Join(t.TempDir(), "missing.toml")})
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	command := NewCommand(runtime)
	command.SetArgs([]string{"--json"})
	if err := executeCommand(command); err == nil {
		t.Fatal("invalid Codex projection was accepted")
	}
	var result checkDocument
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Clients[claudeclient.ID].State != domainreadiness.Deferred || result.Clients[codex.ID].State != domainreadiness.Invalid || result.NextAction+result.NextDetail == "" || result.Problem == "" {
		t.Fatalf("mixed JSON check = %+v", result)
	}
	humanAction := ""
	runtime.Problem = func(_, _, _, action string, cause error) error {
		humanAction = action
		return cause
	}
	command = NewCommand(runtime)
	if err := executeCommand(command); err == nil {
		t.Fatal("human check accepted invalid Codex projection")
	}
	if humanAction != result.NextAction {
		t.Fatalf("human recovery %q differs from JSON action %q", humanAction, result.NextAction)
	}
}

func TestCheckReportsThePerformedInferenceOrEndpointScope(t *testing.T) {
	for _, test := range []struct {
		name   string
		args   []string
		method string
		path   string
		state  domainreadiness.State
		scope  diagnostics.Scope
	}{
		{name: "default inference", args: []string{"--json"}, method: http.MethodPost, path: "/v1/responses", state: domainreadiness.InferenceChecked, scope: diagnostics.ScopeInference},
		{name: "explicit endpoint only", args: []string{"--json", "--endpoint-only"}, method: http.MethodGet, path: "/v1/models", state: domainreadiness.EndpointChecked, scope: diagnostics.ScopeEndpoint},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime, output := configuredCodexScopedCheck(t)
			calls := 0
			runtime.HTTP = roundTripFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				if request.Method != test.method || request.URL.Path != test.path {
					t.Fatalf("request = %s %s, want %s %s", request.Method, request.URL.Path, test.method, test.path)
				}
				if test.scope == diagnostics.ScopeInference {
					var body map[string]any
					if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
						t.Fatal(err)
					}
					if body["model"] != "gpt-test" {
						t.Fatalf("wire model = %v", body["model"])
					}
				}
				return successfulReadinessResponse(request)
			})
			command := NewCommand(runtime)
			command.SetArgs(test.args)
			command.SetContext(t.Context())
			if err := executeCommand(command); err != nil {
				t.Fatal(err)
			}
			var result checkDocument
			if err := json.Unmarshal(output.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			observed := result.Clients[codex.ID]
			if calls != 1 || !result.OK || observed.State != test.state || observed.DiagnosticScope != test.scope ||
				observed.CheckPassed == nil || !*observed.CheckPassed {
				t.Fatalf("calls=%d result=%+v", calls, result)
			}
		})
	}
}

func TestCheckPreservesClaudeNativeOverrideAndUsesEndpointScope(t *testing.T) {
	runtime, cfg, output := configuredReadinessRuntime(t)
	delete(cfg.Clients, codex.ID)
	configureClaudeExecutable(t, &runtime, &cfg)
	synchronizeClaudeSettings(t, runtime, cfg)
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Secrets.Set("one", "fixture-token"); err != nil {
		t.Fatal(err)
	}

	original, err := os.ReadFile(claudeSettingsPath(t, runtime.Host))
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(original, &document); err != nil {
		t.Fatal(err)
	}
	document["model"] = json.RawMessage(`"opus[1m]"`)
	changed, _ := json.MarshalIndent(document, "", "  ")
	changed = append(changed, '\n')
	if err := os.WriteFile(claudeSettingsPath(t, runtime.Host), changed, 0o600); err != nil {
		t.Fatal(err)
	}
	sidecarPath := claudeSettingsPath(t, runtime.Host) + ".aigw-state.json"
	beforeSidecar, err := os.ReadFile(sidecarPath)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	runtime.HTTP = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		if request.Method != http.MethodGet || request.URL.Path != "/v1/models" {
			t.Fatalf("native override sent a model-carrying AIGW request: %s %s", request.Method, request.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":[]}`)), Request: request}, nil
	})

	command := NewCommand(runtime)
	command.SetArgs([]string{"--json"})
	command.SetContext(t.Context())
	if err := executeCommand(command); err != nil {
		t.Fatal(err)
	}
	var result checkDocument
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	observed := result.Clients[claudeclient.ID]
	if calls != 1 || !result.OK || !observed.NativeModelOverride ||
		observed.State != domainreadiness.EndpointChecked || observed.DiagnosticScope != diagnostics.ScopeEndpoint ||
		observed.NextAction != "aigw verify --for claude" {
		t.Fatalf("calls=%d client=%+v", calls, observed)
	}
	afterSettings, _ := os.ReadFile(claudeSettingsPath(t, runtime.Host))
	afterSidecar, _ := os.ReadFile(sidecarPath)
	if !bytes.Equal(changed, afterSettings) || !bytes.Equal(beforeSidecar, afterSidecar) {
		t.Fatal("check changed Claude settings or sidecar")
	}
	assertClaudeNativeOverrideOutput(t, runtime, output)
}

func assertClaudeNativeOverrideOutput(t *testing.T, runtime invocation.Context, output *bytes.Buffer) {
	t.Helper()
	output.Reset()
	if err := status.RunStatus(runtime, true); err != nil {
		t.Fatal(err)
	}
	var status struct {
		Clients map[string]app.ClientStatus `json:"clients"`
	}
	if err := json.Unmarshal(output.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	local := status.Clients[claudeclient.ID]
	if local.State != domainreadiness.Configured || !local.NativeModelOverride ||
		local.NextAction != "aigw verify --for claude" {
		t.Fatalf("local Claude state = %+v", local)
	}

	output.Reset()
	human := NewCommand(runtime)
	human.SetContext(t.Context())
	if err := executeCommand(human); err != nil {
		t.Fatal(err)
	}
	message := strings.ToLower(output.String())
	if !strings.Contains(message, "endpoint checked") ||
		!strings.Contains(message, "native model preference") ||
		!strings.Contains(message, "aigw verify --for claude") {
		t.Fatalf("human Claude check = %q", output.String())
	}
}

func TestHumanCheckNamesTheScopeActuallyPerformed(t *testing.T) {
	for _, test := range []struct {
		name                string
		args                []string
		want                string
		inferenceUnverified bool
	}{
		{name: "default inference", want: "Inference checked"},
		{name: "endpoint only", args: []string{"--endpoint-only"}, want: "Endpoint checked", inferenceUnverified: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime, output := configuredCodexScopedCheck(t)
			runtime.HTTP = roundTripFunc(successfulReadinessResponse)
			command := NewCommand(runtime)
			command.SetArgs(test.args)
			command.SetContext(t.Context())
			if err := executeCommand(command); err != nil {
				t.Fatal(err)
			}
			rendered := output.String()
			if !strings.Contains(rendered, test.want) || !strings.Contains(rendered, "Real-client execution was not verified") {
				t.Fatalf("human check = %q", rendered)
			}
			if strings.Contains(rendered, "Model inference was not verified") != test.inferenceUnverified {
				t.Fatalf("inference evidence mismatch: %q", rendered)
			}
		})
	}
}
