package readiness

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/diagnostics"
	domainreadiness "aigw-cli/internal/readiness"
)

func configuredCodexScopedCheck(t *testing.T) (invocation.Context, *bytes.Buffer) {
	t.Helper()
	runtime, cfg, output := configuredReadinessRuntime(t)
	delete(cfg.Clients, configuration.ClientClaude)
	runtime.Executable = filepath.Join(t.TempDir(), "aigw")
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.SetClientActivation(configuration.ClientCodex, true, "codex", []string{target})
	selected, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
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
			command := NewCheckCommand(runtime)
			command.SetArgs(test.args)
			command.SetContext(t.Context())
			if err := executeCommand(command); err != nil {
				t.Fatal(err)
			}
			var result checkJSON
			if err := json.Unmarshal(output.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			observed := result.Clients[configuration.ClientCodex]
			if calls != 1 || !result.OK || observed.State != test.state || observed.DiagnosticScope != test.scope ||
				observed.CheckPassed == nil || !*observed.CheckPassed {
				t.Fatalf("calls=%d result=%+v", calls, result)
			}
		})
	}
}

func TestCheckPreservesClaudeNativeOverrideAndUsesEndpointScope(t *testing.T) {
	runtime, cfg, output := configuredReadinessRuntime(t)
	delete(cfg.Clients, configuration.ClientCodex)
	configureClaudeExecutable(t, &runtime, &cfg)
	synchronizeClaudeSettings(t, runtime, cfg)
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Secrets.Set("one", "fixture-token"); err != nil {
		t.Fatal(err)
	}

	original, err := os.ReadFile(runtime.ClaudeSettingsPath)
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
	if err := os.WriteFile(runtime.ClaudeSettingsPath, changed, 0o600); err != nil {
		t.Fatal(err)
	}
	sidecarPath := runtime.ClaudeSettingsPath + ".aigw-state.json"
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

	command := NewCheckCommand(runtime)
	command.SetArgs([]string{"--json"})
	command.SetContext(t.Context())
	if err := executeCommand(command); err != nil {
		t.Fatal(err)
	}
	var result checkJSON
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	observed := result.Clients[configuration.ClientClaude]
	if calls != 1 || !result.OK || !observed.NativeModelOverride ||
		observed.State != domainreadiness.EndpointChecked || observed.DiagnosticScope != diagnostics.ScopeEndpoint ||
		observed.NextAction != "aigw verify --for claude" {
		t.Fatalf("calls=%d client=%+v", calls, observed)
	}
	afterSettings, _ := os.ReadFile(runtime.ClaudeSettingsPath)
	afterSidecar, _ := os.ReadFile(sidecarPath)
	if !bytes.Equal(changed, afterSettings) || !bytes.Equal(beforeSidecar, afterSidecar) {
		t.Fatal("check changed Claude settings or sidecar")
	}
	assertClaudeNativeOverrideOutput(t, runtime, output)
}

func assertClaudeNativeOverrideOutput(t *testing.T, runtime invocation.Context, output *bytes.Buffer) {
	t.Helper()
	output.Reset()
	if err := RunStatus(runtime, true); err != nil {
		t.Fatal(err)
	}
	var status statusOutput
	if err := json.Unmarshal(output.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	local := status.Clients[configuration.ClientClaude]
	if local.State != domainreadiness.Configured || !local.NativeModelOverride ||
		local.NextAction != "aigw verify --for claude" {
		t.Fatalf("local Claude state = %+v", local)
	}

	output.Reset()
	human := NewCheckCommand(runtime)
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
			command := NewCheckCommand(runtime)
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
