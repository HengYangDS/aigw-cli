package presentation

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
)

type rewrittenOutputError struct{ cause error }

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
		RenderError(New(&out, false), test.err)
		if !strings.Contains(out.String(), test.want) {
			t.Fatalf("output = %q, want %q", out.String(), test.want)
		}
	}
	var out bytes.Buffer
	RenderError(New(&out, false), Presented(errors.New("already shown")))
	if out.Len() != 0 {
		t.Fatalf("presented error rendered twice: %q", out.String())
	}
}

func TestTypedErrorLocalizationDoesNotDependOnErrorText(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "profile client mismatch", err: &configuration.RuntimeProfileClientMismatchError{ProfileID: "one", ExpectedClient: configuration.ClientCodex, ActualClient: configuration.ClientClaude}, want: `profile "one" is for codex, not claude`},
		{name: "profile unknown account", err: &configuration.RuntimeProfileUnknownAccountError{ProfileID: "one", AccountID: "missing"}, want: `profile "one" references unknown account "missing"`},
		{name: "Anthropic endpoint", err: &configuration.RuntimeMissingEndpointError{AccountID: "one", Protocol: configuration.ProtocolAnthropic}, want: `account "one" has no Anthropic endpoint`},
		{name: "OpenAI Responses endpoint", err: &configuration.RuntimeMissingEndpointError{AccountID: "one", Protocol: configuration.ProtocolOpenAIResponses}, want: `account "one" has no OpenAI Responses endpoint`},
		{name: "unsupported version", err: &configuration.UnsupportedConfigVersionError{Version: 3, ExpectedVersion: 2}, want: "unsupported configuration version: found 3, expected 2"},
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
		`profile "one" is for codex, not claude`,
		`profile "one" references unknown account "missing"`,
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
