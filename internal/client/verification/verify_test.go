package verification

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aigw-cli/internal/claude"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/process"
)

func TestProtocolTimeoutAllowsColdClientStartup(t *testing.T) {
	if ProtocolTimeout < time.Minute {
		t.Fatalf("ProtocolTimeout = %s, want at least %s", ProtocolTimeout, time.Minute)
	}
}

type captureRunner struct {
	output []byte
	stderr []byte
	err    error
}

func (runner captureRunner) RunCapture(context.Context, process.Plan) ([]byte, error) {
	return runner.output, runner.err
}

func (runner captureRunner) RunCaptureStreams(context.Context, process.Plan) ([]byte, []byte, error) {
	return runner.output, runner.stderr, runner.err
}

func TestClaudeWarningsDoNotQualifyVerification(t *testing.T) {
	selected := configuration.Runtime{RouteID: "claude", Model: "claude-test", CredentialCommand: filepath.Join(t.TempDir(), "aigw")}
	for _, diagnostic := range []string{"warning: native capability incomplete\n", "ERROR client could not initialize\n"} {
		runner := captureRunner{output: []byte("AIGW_OK\n"), stderr: []byte(diagnostic)}
		if err := VerifyClaudeRuntime(t.Context(), runner, "claude", filepath.Join(t.TempDir(), "settings.json"), selected, "token", ""); err == nil {
			t.Fatal("Claude diagnostics qualified a successful process")
		}
	}
}

func verificationConfig() configuration.Config {
	cfg := configuration.NewConfig()
	cfg.Accounts["one"] = configuration.Account{Label: "One", Endpoints: configuration.Endpoints{
		OpenAIResponses: "https://one.test/v1",
		Anthropic:       "https://one.test",
	}}
	cfg.Routes["codex"] = configuration.Route{Label: "Codex", Account: "one", Model: "gpt-test", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}}}
	cfg.Routes["claude"] = configuration.Route{Label: "Claude", Account: "one", Model: "claude-test", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}}}
	cfg.SetSelectedRoute(configuration.ClientCodex, "codex", "")
	cfg.SetSelectedRoute(configuration.ClientClaude, "claude", "")
	return cfg
}

func TestVerifyClaude(t *testing.T) {
	cfg := verificationConfig()
	runtime, err := cfg.ResolveRuntime(configuration.ClientClaude, "")
	if err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(t.TempDir(), "settings.json")
	runtime.CredentialCommand = filepath.Join(t.TempDir(), "aigw")
	if _, err := claude.ReconcileSettings(settings, false, runtime, runtime.CredentialCommand, runtime.Model); err != nil {
		t.Fatal(err)
	}
	if err := VerifyClaudeRuntime(t.Context(), nil, "claude", settings, configuration.Runtime{RouteID: "one"}, "token", ""); err == nil || !strings.Contains(err.Error(), "no Claude model") {
		t.Fatalf("model error = %v", err)
	}
	if err := VerifyClaudeRuntime(t.Context(), nil, "", settings, runtime, "token", ""); err == nil || !strings.Contains(err.Error(), "executable is not configured") {
		t.Fatalf("plan error = %v", err)
	}
	if err := VerifyClaudeRuntime(t.Context(), nil, "claude", settings, runtime, "token", ""); err == nil || !strings.Contains(err.Error(), "runner is unavailable") {
		t.Fatalf("runner error = %v", err)
	}
	cause := errors.New("launch /Users/operator/private/claude: exit status 1")
	compatibility := `{"type":"result","is_error":true,"result":"API Error: 400 context_management: Extra inputs are not permitted; token=must-not-leak; /Users/operator/private"}`
	if err := Failure("Codex", configuration.ClientCodex, "", []byte(compatibility), cause); strings.Contains(err.Error(), "CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS") {
		t.Fatalf("Codex failure advised a Claude-specific setting: %v", err)
	}
	for _, test := range []struct {
		name, output, diagnostic, want string
		cause                          error
	}{
		{"native result", "{\"type\":\"system\",\"subtype\":\"init\"}\n{\"type\":\"result\",\"is_error\":false,\"result\":\" AIGW_OK \\n\"}", "", "", nil},
		{"missing result", `{"type":"assistant","message":"AIGW_OK"}`, "", "expected AIGW_OK", nil},
		{"wrong result", `{"type":"result","is_error":false,"result":"wrong"}`, "", "expected AIGW_OK", nil},
		{"text is not a result", "AIGW_OK\n", "", "invalid native Claude result", nil},
		{"failed result", `{"type":"result","is_error":true,"result":"AIGW_OK"}`, "", "minimal verification request failed", nil},
		{"refusal fallback", "{\"type\":\"system\",\"subtype\":\"model_refusal_fallback\",\"trigger\":\"refusal\",\"scope\":\"session\",\"original_model\":\"claude-test\",\"fallback_model\":\"private-fallback must-not-leak\"}\n{\"type\":\"result\",\"is_error\":false,\"result\":\"AIGW_OK\"}", "", "selected model was not qualified", nil},
		{"private stderr", "", "/Users/operator/private", "minimal verification request failed", cause},
		{"model rejection", "[claude-code:unrecognized_model] claude-next token=must-not-leak request id=secret-request", "", "selected model is unavailable", cause},
		{"context rejection", compatibility, "", "CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1", cause},
		{"authorization before timeout", `{"type":"system","subtype":"api_retry","error_status":403}`, "", "timed out after endpoint HTTP 403", context.DeadlineExceeded},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := captureRunner{output: []byte(test.output), stderr: []byte(test.diagnostic), err: test.cause}
			err := VerifyClaudeRuntime(t.Context(), runner, "claude", settings, runtime, "must-not-leak", "")
			if test.want == "" && err != nil || test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)) {
				t.Fatalf("verification = %v, want %q", err, test.want)
			}
			if test.cause != nil && !errors.Is(err, test.cause) {
				t.Fatalf("native process cause was lost: %v", err)
			}
			if err != nil {
				for _, secret := range []string{"must-not-leak", "/Users/operator", "secret-request", "claude-next"} {
					if strings.Contains(err.Error(), secret) {
						t.Fatalf("private native output escaped: %v", err)
					}
				}
			}
		})
	}
}
