package terminal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"syscall"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"

	"github.com/pelletier/go-toml/v2"
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
	for _, test := range []struct {
		cause error
		want  string
	}{
		{&os.PathError{Op: "read", Path: privatePath, Err: os.ErrPermission}, "Local file access failed (read: permission denied)"},
		{&os.LinkError{Op: "rename", Old: privatePath, New: privatePath + ".new", Err: os.ErrPermission}, "Local file access failed (rename: permission denied)"},
		{&os.PathError{Op: "open", Path: privatePath, Err: os.ErrNotExist}, "Local file access failed (open: no such file or directory)"},
		{&os.PathError{Op: "stat", Path: privatePath, Err: syscall.ENOENT}, "Local file access failed (stat: " + syscall.ENOENT.Error() + ")"},
		{&os.PathError{Op: "mkdir", Path: privatePath, Err: fs.ErrExist}, "Local file access failed (mkdir: file exists)"},
		{&os.PathError{Op: "mkdir", Path: privatePath, Err: errors.New(privatePath + " is a private detail")}, "Local file access failed (mkdir: unexpected error)"},
	} {
		for _, jsonMode := range []bool{false, true} {
			var out bytes.Buffer
			renderer := NewRenderer(&out, false)
			RenderError(renderer, fmt.Errorf("capture current config: %w", test.cause), jsonMode)
			if renderer.Err() != nil || strings.Contains(out.String(), privatePath) ||
				!strings.Contains(out.String(), test.want) || !strings.Contains(out.String(), "aigw doctor") {
				t.Fatalf("json=%t local file diagnostic = %q, want %q, render error = %v", jsonMode, out.String(), test.want, renderer.Err())
			}
		}
	}
}

func TestConfigurationLoadFailuresNameTheirStage(t *testing.T) {
	const content = "secret-looking configuration content"
	decodeErr := toml.Unmarshal([]byte("[routes]\nbroken = \n"), &struct{}{})
	syntax, ok := errors.AsType[*toml.DecodeError](decodeErr)
	if !ok {
		t.Fatalf("fixture did not produce a TOML decode error: %v", decodeErr)
	}
	line, column := syntax.Position()
	for name, test := range map[string]struct {
		err  error
		want []string
	}{
		"read": {
			err:  &store.LoadError{Phase: store.LoadPhaseRead, Err: &os.PathError{Op: "open", Path: "/private/config.toml", Err: os.ErrPermission}},
			want: []string{"Cannot read local configuration", "read stage (open: permission denied)", "Restore read access", "aigw config path"},
		},
		"parse": {
			err:  &store.LoadError{Phase: store.LoadPhaseParse, Err: fmt.Errorf("%s: %w", content, syntax)},
			want: []string{"Cannot parse local configuration", fmt.Sprintf("parse stage at line %d, column %d", line, column), "parse position", "aigw config path"},
		},
		"validate": {
			err:  &store.LoadError{Phase: store.LoadPhaseValidate, Err: errors.New(content)},
			want: []string{"Cannot validate local configuration", "AIGW 9.8.7 stopped at the configuration validate stage", "newer AIGW", "fail validation", "aigw config path"},
		},
		"unknown stage": {
			err:  &store.LoadError{Phase: "replay", Err: errors.New(content)},
			want: []string{"Cannot load local configuration", "stopped at the configuration replay stage", "aigw config path"},
		},
	} {
		for _, jsonMode := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/json=%t", name, jsonMode), func(t *testing.T) {
				var out bytes.Buffer
				renderer := NewRenderer(&out, false)
				RenderError(renderer, CommandFailure("9.8.7", "aigw status", fmt.Errorf("load: %w", test.err)), jsonMode)
				rendered := out.String()
				if renderer.Err() != nil || strings.Contains(rendered, content) || strings.Contains(rendered, "/private/config.toml") {
					t.Fatalf("configuration diagnostic exposed private detail: %q, %v", rendered, renderer.Err())
				}
				for _, required := range test.want {
					if !strings.Contains(rendered, required) {
						t.Fatalf("configuration diagnostic omits %q: %q", required, rendered)
					}
				}
				// A command that loads the configuration would fail the same
				// way, and a load failure states no impact of its own.
				for _, forbidden := range []string{"aigw doctor", "aigw check", "aigw status", "Impact", "impact"} {
					if strings.Contains(rendered, forbidden) {
						t.Fatalf("configuration diagnostic names %q: %q", forbidden, rendered)
					}
				}
			})
		}
	}
}

func TestCommandFailureKeepsProblemsAndAttributesOtherErrors(t *testing.T) {
	if got := CommandFailure("1.2.3", "aigw status", nil); got != nil {
		t.Fatalf("CommandFailure(nil) = %v", got)
	}
	problem := ProblemError("Problem", "", "", "aigw sync", nil)
	if got := CommandFailure("1.2.3", "aigw status", problem); !errors.Is(got, problem) || !errors.Is(problem, got) {
		t.Fatalf("CommandFailure changed a structured problem into %#v", got)
	}
	recovery := presentationProjectionError{restored: true, cause: &os.PathError{Op: "write", Path: "/private/x", Err: os.ErrPermission}}
	unsupported := &store.LoadError{Phase: store.LoadPhaseValidate, Err: &domain.UnsupportedConfigVersionError{Version: 0, ExpectedVersion: domain.ConfigVersion}}
	for _, err := range []error{errors.New("plain failure"), recovery, unsupported} {
		if got := CommandFailure("1.2.3", "aigw status", err); !errors.Is(got, err) || got.Error() != err.Error() {
			t.Fatalf("CommandFailure lost %#v in %#v", err, got)
		}
	}
	for err, want := range map[error]string{
		errors.New("plain failure"): "aigw status --help",
		recovery:                    "Run `aigw doctor` before retrying.",
		unsupported:                 ConfigurationLoadFix(unsupported),
	} {
		if got := ProblemFor(CommandFailure("1.2.3", "aigw status", err)).Fix; got != want {
			t.Fatalf("fix of %v = %q, want %q", err, got, want)
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
				renderer := NewRenderer(&out, false)
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
			renderer := NewRenderer(&out, false)
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
	err := fmt.Errorf("%w: %s", secret.ErrNativeReaderUnverified, privateCause)
	for _, jsonMode := range []bool{false, true} {
		var out bytes.Buffer
		renderer := NewRenderer(&out, false)
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
		`unknown command "oops" for "aigw"`:              `unknown command "oops"`,
		"unknown flag: --oops":                           "unknown option --oops",
		"unknown flag: --oops; run `aigw status --help`": "unknown option --oops; run `aigw status --help`",
	} {
		if got := localizeCobraError(input); got != want {
			t.Errorf("localizeCobraError(%q) = %q, want %q", input, got, want)
		}
	}
	if _, ok := cobraUnknownCommand(`unknown command "" for "aigw"`); ok {
		t.Fatal("empty unknown command accepted")
	}
	if _, _, ok := cobraUnknownFlag("unknown flag: --two words"); ok {
		t.Fatal("spaced flag accepted")
	}
	if got := mentionedAIGWCommand("run `aigw repair without terminator"); got != "" {
		t.Fatalf("unterminated command = %q", got)
	}
}

// A generic failure states no impact: its owner, not the presentation, knows
// whether anything changed, so it neither claims nor denies a rollback.
func TestRenderErrorLeavesTransactionOutcomeToItsOwner(t *testing.T) {
	for _, jsonMode := range []bool{false, true} {
		var out bytes.Buffer
		RenderError(NewRenderer(&out, false), CommandFailure("1.2.3", "aigw sync", errors.New("output is unavailable")), jsonMode)
		rendered := out.String()
		if !strings.Contains(rendered, "output is unavailable") || !strings.Contains(rendered, "aigw sync --help") {
			t.Fatalf("json=%t generic failure lacks its problem or fix: %s", jsonMode, rendered)
		}
		for _, forbidden := range []string{"Impact", "impact", "roll", "could not finish"} {
			if strings.Contains(rendered, forbidden) {
				t.Fatalf("json=%t generic failure states %q: %s", jsonMode, forbidden, rendered)
			}
		}
	}
}

func TestRenderErrorAndFixBranches(t *testing.T) {
	cause := errors.New("cause")
	problem := ProblemError("Problem title", "Evidence", "Impact", "aigw repair", cause)
	if problem.Error() != "Problem title" || !errors.Is(problem, cause) {
		t.Fatalf("problem error = %v", problem)
	}
	presented := Presented(problem)
	if presented.Error() != "Problem title" || !errors.Is(presented, cause) {
		t.Fatalf("presented error = %v", presented)
	}
	decodeErr := toml.Unmarshal([]byte("broken = \n"), &struct{}{})

	for _, test := range []struct {
		err  error
		want string
	}{
		{problem, "aigw repair"},
		{errors.New("unknown command \"oops\" for \"aigw\""), "aigw --help"},
		{errors.New("failed; run `aigw repair`"), "aigw repair"},
		{errors.New("plain failure"), "aigw --help"},
		// An error that names no fix takes the help of the command that
		// returned it, including an argument that a leaf command refused.
		{CommandFailure("1.2.3", "aigw", errors.New("unknown command \"oops\" for \"aigw\"")), "aigw --help"},
		{CommandFailure("1.2.3", "aigw route list", errors.New("unknown command \"extra\" for \"aigw route list\"")), "aigw route list --help"},
		{CommandFailure("1.2.3", "aigw provider show", errors.New("accepts 1 arg(s), received 0")), "aigw provider show --help"},
		{CommandFailure("1.2.3", "aigw test", fmt.Errorf("resolve: %w", &domain.RuntimeBindingUnselectedError{Client: codex.ID})), "aigw use --for codex <route>"},
		{CommandFailure("1.2.3", "aigw config import", fmt.Errorf("parse manifest: %w", decodeErr)), "Correct the value at the reported document location and rerun the command."},
	} {
		var out bytes.Buffer
		RenderError(NewRenderer(&out, false), test.err, false)
		if fix := ProblemFor(test.err).Fix; fix != test.want || !strings.Contains(out.String(), "Fix\n  "+test.want) {
			t.Fatalf("fix = %q, output = %q, want %q", fix, out.String(), test.want)
		}
		if strings.Contains(out.String(), "aigw check") {
			t.Fatalf("output names an unrelated check: %q", out.String())
		}
	}
	var out bytes.Buffer
	RenderError(NewRenderer(&out, false), Presented(errors.New("already shown")), false)
	if out.Len() != 0 {
		t.Fatalf("presented error rendered twice: %q", out.String())
	}
}

func TestJSONErrorPreservesProblemAndOutputFailure(t *testing.T) {
	cause := errors.New("internal cause")
	problem := ProblemError("Cannot continue", "Observed failure", "Operation stopped", "aigw doctor", cause)
	var out bytes.Buffer
	renderer := NewRenderer(&out, false)
	RenderError(renderer, problem, true)
	var result struct {
		OK         bool   `json:"ok"`
		Error      string `json:"error"`
		Evidence   string `json:"evidence"`
		Impact     string `json:"impact"`
		NextAction string `json:"next_action"`
		NextDetail string `json:"next_detail"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.OK || result.Error != "Cannot continue" || result.Evidence != "Observed failure" || result.Impact != "Operation stopped" ||
		result.NextAction != "aigw doctor" || result.NextDetail != "" || renderer.Err() != nil {
		t.Fatalf("result=%+v error=%v", result, renderer.Err())
	}
	// A fix that a script cannot run as it stands is guidance.
	out.Reset()
	RenderError(renderer, ProblemError("Cannot continue", "", "", "Restore the file, then run `aigw sync`", cause), true)
	result.NextAction, result.NextDetail = "", ""
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.NextAction != "" || result.NextDetail != "Restore the file, then run `aigw sync`" {
		t.Fatalf("guidance result=%+v error=%v", result, err)
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
	renderer = NewRenderer(writer, false)
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
		{name: "route client mismatch", err: &domain.RuntimeModelClientMismatchError{ModelID: "one", ExpectedClient: codex.ID, ActualClient: claude.ID}, want: `route "one" is for codex, not claude`},
		{name: "route unknown account", err: &domain.RuntimeModelUnknownProviderError{ModelID: "one", ProviderID: "missing"}, want: `route "one" references unknown account "missing"`},
		{name: "Anthropic endpoint", err: &domain.RuntimeMissingEndpointError{ProviderID: "one", Protocol: domain.ProtocolAnthropic}, want: `account "one" has no Anthropic Messages endpoint`},
		{name: "OpenAI Responses endpoint", err: &domain.RuntimeMissingEndpointError{ProviderID: "one", Protocol: domain.ProtocolOpenAIResponses}, want: `account "one" has no OpenAI Responses endpoint`},
		{name: "OpenAI Chat Completions endpoint", err: &domain.RuntimeMissingEndpointError{ProviderID: "one", Protocol: domain.ProtocolOpenAIChatCompletions}, want: `account "one" has no OpenAI Chat Completions endpoint`},
		{name: "unsupported version", err: &domain.UnsupportedConfigVersionError{Version: 3, ExpectedVersion: 2}, want: "unsupported configuration version: found 3, expected 2. AIGW does not reinterpret configuration schemas"},
		{name: "migratable version", err: &domain.UnsupportedConfigVersionError{Version: domain.LegacyConfigVersion, ExpectedVersion: domain.ConfigVersion}, want: fmt.Sprintf("unsupported configuration version: found %d, expected %d. AIGW does not reinterpret configuration schemas; run `aigw config migrate --dry-run`", domain.LegacyConfigVersion, domain.ConfigVersion)},
		{name: "published migratable version", err: &domain.UnsupportedConfigVersionError{Version: domain.PublishedConfigVersion, ExpectedVersion: domain.ConfigVersion}, want: fmt.Sprintf("unsupported configuration version: found %d, expected %d. AIGW does not reinterpret configuration schemas; run `aigw config migrate --dry-run`", domain.PublishedConfigVersion, domain.ConfigVersion)},
		{name: "config load", err: &store.LoadError{Phase: store.LoadPhaseRead, Err: errors.New("details changed")}, want: "Cannot read local configuration"},
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
		`account "one" has no Anthropic Messages endpoint`,
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
