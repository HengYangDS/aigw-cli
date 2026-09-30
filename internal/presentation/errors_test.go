package presentation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

type rewrittenOutputError struct{ cause error }

type presentationProjectionError struct {
	restored bool
	cause    error
}

func (e presentationProjectionError) Error() string               { return e.cause.Error() }
func (e presentationProjectionError) Unwrap() error               { return e.cause }
func (e presentationProjectionError) ConfigurationRestored() bool { return e.restored }

func TestRenderErrorDoesNotExposeLocalFilePath(t *testing.T) {
	const privatePath = "/private/account-store/configuration.toml.bak"
	for _, cause := range []error{
		&os.PathError{Op: "read", Path: privatePath, Err: os.ErrPermission},
		&os.LinkError{Op: "rename", Old: privatePath, New: privatePath + ".new", Err: os.ErrPermission},
	} {
		for _, jsonMode := range []bool{false, true} {
			var out bytes.Buffer
			renderer := New(&out, false)
			RenderError(renderer, fmt.Errorf("capture current config: %w", cause), jsonMode)
			if renderer.Err() != nil || strings.Contains(out.String(), privatePath) ||
				!strings.Contains(out.String(), "Local file access failed") || !strings.Contains(out.String(), "aigw doctor") {
				t.Fatalf("json=%t local file diagnostic = %q, render error = %v", jsonMode, out.String(), renderer.Err())
			}
		}
	}
}

func TestRenderProjectionFailureReportsConfigurationRecoveryWithoutPrivatePath(t *testing.T) {
	const privatePath = "/private/member/hermes/.aigw-state.json"
	for _, tc := range []struct {
		name     string
		restored bool
		want     string
	}{
		{"restored", true, "Configuration was restored"},
		{"incomplete", false, "Configuration restoration was incomplete"},
	} {
		for _, jsonMode := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/json=%t", tc.name, jsonMode), func(t *testing.T) {
				cause := &os.PathError{Op: "write", Path: privatePath, Err: os.ErrPermission}
				err := fmt.Errorf("configuration manifest: %w", presentationProjectionError{restored: tc.restored, cause: cause})
				var out bytes.Buffer
				renderer := New(&out, false)
				RenderError(renderer, err, jsonMode)
				if renderer.Err() != nil || !strings.Contains(out.String(), tc.want) ||
					!strings.Contains(out.String(), "aigw doctor") || strings.Contains(out.String(), privatePath) {
					t.Fatalf("projection recovery result = %q, render error = %v", out.String(), renderer.Err())
				}
			})
		}
	}
}

func TestCredentialErrorUsesOnlySafeProblems(t *testing.T) {
	const canary = "private-backend-output"
	cause := errors.New(canary)
	for _, test := range []struct {
		name string
		err  error
		want string
	}{
		{"raw", cause, "aigw sync"},
		{"safe", ProblemError("Token unavailable", "", "No Token returned.", "Inspect selected backend.", cause), "Inspect selected backend."},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			renderer := New(&out, false)
			RenderCredentialError(renderer, test.err)
			if renderer.Err() != nil || !strings.Contains(out.String(), test.want) || strings.Contains(out.String(), canary) {
				t.Fatalf("unsafe credential diagnostic: output=%q error=%v", out.String(), renderer.Err())
			}
			if !errors.Is(test.err, cause) {
				t.Fatal("credential cause was lost")
			}
		})
	}
}

func TestNativeReaderPreflightReportsRecoveryWithoutPrivateCause(t *testing.T) {
	const privateCause = "private-token-fragment /Users/operator/keychain"
	err := fmt.Errorf("%w: %s", secrets.ErrNativeReaderUnverified, privateCause)
	for _, jsonMode := range []bool{false, true} {
		var out bytes.Buffer
		renderer := New(&out, false)
		RenderError(renderer, err, jsonMode)
		if renderer.Err() != nil {
			t.Fatalf("json=%t render error: %v", jsonMode, renderer.Err())
		}
		for _, expected := range []string{"native Account Token access", "aigw doctor", "aigw sync"} {
			if !strings.Contains(out.String(), expected) {
				t.Fatalf("json=%t output lacks %q: %s", jsonMode, expected, &out)
			}
		}
		if strings.Contains(out.String(), privateCause) || strings.Contains(out.String(), "aigw check") {
			t.Fatalf("json=%t output exposed a private cause or misleading action: %s", jsonMode, &out)
		}
	}
}

func (e rewrittenOutputError) Error() string { return "completely rewritten outer error" }
func (e rewrittenOutputError) Unwrap() error { return e.cause }

func TestOutputLocalizationMalformedAndSpecificBranches(t *testing.T) {
	for input, want := range map[string]string{
		`unknown command "oops" for "aigw"`: `unknown command "oops"`,
		"unknown flag: --oops":              "unknown option --oops",
	} {
		if got := localizeCobraError(input); got != want {
			t.Errorf("localizeCobraError(%q) = %q, want %q", input, got, want)
		}
	}
	if _, ok := cobraUnknownCommand(`unknown command "" for "aigw"`); ok {
		t.Fatal("empty unknown command accepted")
	}
	if _, ok := cobraUnknownFlag("unknown flag: --two words"); ok {
		t.Fatal("spaced flag accepted")
	}
	if got := mentionedAIGWCommand("run `aigw repair without terminator"); got != "" {
		t.Fatalf("unterminated command = %q", got)
	}
}

func TestRenderErrorLeavesTransactionOutcomeToItsOwner(t *testing.T) {
	var out bytes.Buffer
	RenderError(New(&out, false), errors.New("output is unavailable"), false)
	if !strings.Contains(out.String(), "The command could not finish; inspect current state before retrying.") {
		t.Fatalf("generic failure did not preserve transaction uncertainty: %s", &out)
	}
}

func TestRenderErrorAndSuggestedFixBranches(t *testing.T) {
	cause := errors.New("cause")
	problem := ProblemError("Problem title", "Evidence", "Impact", "aigw repair", cause)
	if problem.Error() != "Problem title" || !errors.Is(problem, cause) {
		t.Fatalf("problem error = %v", problem)
	}
	presented := Presented(problem)
	if presented.Error() != "Problem title" || !errors.Is(presented, cause) {
		t.Fatalf("presented error = %v", presented)
	}

	for _, test := range []struct {
		err  error
		want string
	}{
		{problem, "Problem title"},
		{errors.New("unknown command \"oops\" for \"aigw\""), "aigw --help"},
		{errors.New("failed; run `aigw repair`"), "aigw repair"},
		{errors.New("plain failure"), "aigw check"},
	} {
		var out bytes.Buffer
		RenderError(New(&out, false), test.err, false)
		if !strings.Contains(out.String(), test.want) {
			t.Fatalf("output = %q, want %q", out.String(), test.want)
		}
	}
	var out bytes.Buffer
	RenderError(New(&out, false), Presented(errors.New("already shown")), false)
	if out.Len() != 0 {
		t.Fatalf("presented error rendered twice: %q", out.String())
	}
}

func TestJSONErrorPreservesProblemAndOutputFailure(t *testing.T) {
	cause := errors.New("internal cause")
	problem := ProblemError("Cannot continue", "Observed failure", "Operation stopped", "aigw doctor", cause)
	var out bytes.Buffer
	renderer := New(&out, false)
	RenderError(renderer, problem, true)
	var result struct {
		OK bool `json:"ok"`
		Problem
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	want := Problem{Title: "Cannot continue", Evidence: "Observed failure", Impact: "Operation stopped", Fix: "aigw doctor"}
	if result.OK || result.Problem != want || renderer.Err() != nil {
		t.Fatalf("result=%+v error=%v", result, renderer.Err())
	}
	out.Reset()
	RenderError(renderer, Presented(problem), true)
	if out.Len() != 0 {
		t.Fatalf("presented result emitted again: %s", &out)
	}
	reader, writer := io.Pipe()
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := writer.Close(); err != nil {
			t.Error(err)
		}
	})
	renderer = New(writer, false)
	RenderError(renderer, problem, true)
	if !errors.Is(renderer.Err(), io.ErrClosedPipe) {
		t.Fatalf("renderer lost output failure: %v", renderer.Err())
	}
}

func TestTypedErrorLocalizationDoesNotDependOnErrorText(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "route client mismatch", err: &configuration.RuntimeRouteClientMismatchError{RouteID: "one", ExpectedClient: configuration.ClientCodex, ActualClient: configuration.ClientClaude}, want: `route "one" is for codex, not claude`},
		{name: "route unknown account", err: &configuration.RuntimeRouteUnknownAccountError{RouteID: "one", AccountID: "missing"}, want: `route "one" references unknown account "missing"`},
		{name: "Anthropic endpoint", err: &configuration.RuntimeMissingEndpointError{AccountID: "one", Protocol: configuration.ProtocolAnthropic}, want: `account "one" has no Anthropic endpoint`},
		{name: "OpenAI Responses endpoint", err: &configuration.RuntimeMissingEndpointError{AccountID: "one", Protocol: configuration.ProtocolOpenAIResponses}, want: `account "one" has no OpenAI Responses endpoint`},
		{name: "OpenAI Chat Completions endpoint", err: &configuration.RuntimeMissingEndpointError{AccountID: "one", Protocol: configuration.ProtocolOpenAIChatCompletions}, want: `account "one" has no OpenAI Chat Completions endpoint`},
		{name: "unsupported version", err: &configuration.UnsupportedConfigVersionError{Version: 3, ExpectedVersion: 2}, want: "unsupported configuration version: found 3, expected 2. AIGW does not reinterpret configuration schemas"},
		{name: "migratable version", err: &configuration.UnsupportedConfigVersionError{Version: configuration.LegacyConfigVersion, ExpectedVersion: configuration.ConfigVersion}, want: fmt.Sprintf("unsupported configuration version: found %d, expected %d. AIGW does not reinterpret configuration schemas; run `aigw config migrate --dry-run`", configuration.LegacyConfigVersion, configuration.ConfigVersion)},
		{name: "published migratable version", err: &configuration.UnsupportedConfigVersionError{Version: configuration.PublishedConfigVersion, ExpectedVersion: configuration.ConfigVersion}, want: fmt.Sprintf("unsupported configuration version: found %d, expected %d. AIGW does not reinterpret configuration schemas; run `aigw config migrate --dry-run`", configuration.PublishedConfigVersion, configuration.ConfigVersion)},
		{name: "config load", err: &configuration.LoadError{Phase: configuration.LoadPhaseRead, Err: errors.New("details changed")}, want: "Cannot read or validate local configuration; run `aigw doctor` to inspect or restore it"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wrapped := rewrittenOutputError{cause: test.err}
			got, ok := typedErrorMessage(wrapped)
			if !ok || got != test.want {
				t.Fatalf("typedErrorMessage() = %q, %v, want %q, true", got, ok, test.want)
			}
			if got := localizedErrorMessage(wrapped); got != test.want {
				t.Fatalf("localizedErrorMessage() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestTypedErrorLocalizationRejectsTextualLookalikes(t *testing.T) {
	for _, message := range []string{
		`route "one" is for codex, not claude`,
		`route "one" references unknown account "missing"`,
		`account "one" has no Anthropic endpoint`,
		`account "one" has no OpenAI Responses endpoint`,
		"validate config: unsupported config version 3; expected 2",
		"read config: details changed",
		"parse config: details changed",
		"validate config: details changed",
	} {
		err := errors.New(message)
		if got, ok := typedErrorMessage(err); ok {
			t.Errorf("typedErrorMessage(%q) = %q, true", message, got)
		}
		if got := localizedErrorMessage(err); got != message {
			t.Errorf("localizedErrorMessage(%q) = %q", message, got)
		}
	}
}
