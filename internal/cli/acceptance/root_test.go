package cli_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
)

func TestJSONCommandFailuresRemainMachineReadable(t *testing.T) {
	for _, args := range [][]string{
		{"status", "--json"},
		{"sync", "--json"},
		{"repair", "--json"},
		{"catalog", "--json"},
		{"setup", "--json"},
		{"route", "rename", "old", "new", "--dry-run", "--json"},
		{"account", "rename", "old", "new", "--dry-run", "--json"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			app, out, _, _, _ := testApp(t, "")
			var stderr bytes.Buffer
			app.Err = &stderr
			if err := os.WriteFile(app.Config.Path(), []byte("invalid configuration ["), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := cli.Execute(app, args); err == nil {
				t.Fatal("invalid invocation succeeded")
			}
			var result struct {
				OK         bool   `json:"ok"`
				Error      string `json:"error"`
				NextAction string `json:"next_action"`
				NextDetail string `json:"next_detail"`
			}
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatalf("expected one JSON failure: %v\n%s", err, out)
			}
			if result.OK || result.Error == "" || result.NextAction+result.NextDetail == "" || stderr.Len() != 0 {
				t.Fatalf("result=%+v stderr=%q", result, &stderr)
			}
		})
	}
}

func TestFailureFormatUsesParsedJSONFlag(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	for _, test := range []struct {
		args []string
		json bool
	}{
		{args: []string{"repair", "--json=true"}, json: true},
		{args: []string{"repair", "--json=false"}},
		{args: []string{"repair", "--", "--json"}},
	} {
		out.Reset()
		if err := cli.Execute(app, test.args); err == nil {
			t.Fatalf("%v succeeded", test.args)
		}
		if json.Valid(out.Bytes()) != test.json {
			t.Fatalf("%v JSON=%t: %s", test.args, test.json, out)
		}
	}
}

type interruptedOutput struct {
	bytes.Buffer
	err error
}

func (out *interruptedOutput) Write(data []byte) (int, error) {
	if out.Len() == 0 {
		count, _ := out.Buffer.Write(data[:1])
		return count, out.err
	}
	return out.Buffer.Write(data)
}

func TestJSONOutputFailureIsNeverFollowedByAnotherResult(t *testing.T) {
	for _, failure := range []error{io.ErrClosedPipe, nil} {
		app, _, _, _, _ := testApp(t, "")
		out := &interruptedOutput{err: failure}
		app.Out = out
		want := failure
		if want == nil {
			want = io.ErrShortWrite
		}
		err := cli.Execute(app, []string{"status", "--json"})
		if !errors.Is(err, want) || out.String() != "{" {
			t.Fatalf("error=%v output=%q, want original write failure and one partial result", err, out)
		}
	}
}

func TestOutputFailureDoesNotLeakIntoTheNextInvocation(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	app.Out = &interruptedOutput{err: io.ErrClosedPipe}
	if err := cli.Execute(app, []string{"status", "--json"}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("first invocation error=%v", err)
	}
	app.Out = out
	if err := cli.Execute(app, []string{"status", "--json"}); err != nil || !json.Valid(out.Bytes()) || app.Out != out {
		t.Fatalf("second invocation error=%v output=%s writer=%T", err, out, app.Out)
	}
}

func TestErrorRenderingPreservesCommandAndWriterFailures(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	app.Out = failingOutput{err: io.ErrClosedPipe}
	err := cli.Execute(app, []string{"repair", "--json"})
	if !errors.Is(err, io.ErrClosedPipe) || !strings.Contains(strings.ToLower(err.Error()), "not configured") {
		t.Fatalf("lost command or rendering failure: %v", err)
	}
}

func TestUnconfiguredCommandsPointToSetupWithoutLoops(t *testing.T) {
	for _, command := range [][]string{{"status"}, {"check"}, {"repair"}, {"models"}, {"catalog"}} {
		app, out, _, _, _ := testApp(t, "")
		err := cli.Execute(app, command)
		if command[0] == "status" && err != nil {
			t.Fatalf("%v error = %v", command, err)
		}
		if command[0] != "status" && err == nil {
			t.Fatalf("%v succeeded without configuration", command)
		}
		text := out.String() + "\n"
		if err != nil {
			text += err.Error()
		}
		if !strings.Contains(text, "aigw setup") {
			t.Fatalf("%v should point to setup:\n%s", command, text)
		}
		if strings.Contains(text, "run `aigw`") || strings.Contains(text, "aigw repair") || strings.Contains(text, "aigw check") {
			t.Fatalf("%v retained a loop or ambiguous first-use action:\n%s", command, text)
		}
	}
}

func TestCommonCommandFailuresUseEnglishGuidance(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	for _, tc := range []struct {
		args []string
		want string
		fix  string
	}{
		{args: []string{"use", "--for", "other", "one"}, want: "--for must be claude, claude-desktop, codex, or hermes", fix: "aigw use --help"},
	} {
		out.Reset()
		err := cli.Execute(app, tc.args)
		if err == nil || !strings.Contains(out.String(), tc.want) || !strings.Contains(out.String(), tc.fix) {
			t.Fatalf("%v err=%v output=%s", tc.args, err, out.String())
		}
	}
}

func TestUnknownCommandSuggestsTopLevelHelp(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	err := cli.Execute(app, []string{"not-a-command"})
	if err == nil || !strings.Contains(out.String(), "unknown command") || !strings.Contains(out.String(), "aigw --help") {
		t.Fatalf("err=%v output=%s", err, out.String())
	}
	out.Reset()
	err = cli.Execute(app, []string{"not-a-command", "--json"})
	if err == nil || !strings.Contains(out.String(), `unknown command "not-a-command"`) || strings.Contains(out.String(), "unknown option") {
		t.Fatalf("unknown command with a flag: err=%v output=%s", err, out.String())
	}
}

// TestFlagErrorsSuggestTheCommandHelp shows that a flag error names the help
// of the command that refused the flag, not top-level help or a readiness check.
func TestFlagErrorsSuggestTheCommandHelp(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"status", "--not-a-flag"}, "unknown option --not-a-flag; run `aigw status --help`"},
		{[]string{"--not-a-flag"}, "unknown option --not-a-flag; run `aigw --help`"},
		{[]string{"install", "--path"}, "flag needs an argument: --path; run `aigw install --help`"},
	} {
		out.Reset()
		err := cli.Execute(app, tc.args)
		if err == nil || !strings.Contains(out.String(), tc.want) || strings.Contains(out.String(), "aigw check") {
			t.Fatalf("%v err=%v output=%s", tc.args, err, out.String())
		}
	}
}

func TestCoreValidationFailuresUseEnglishGuidance(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	for _, tc := range []struct {
		args []string
		want string
	}{
		{args: []string{"test", "--for", "other"}, want: "--for must be claude, claude-desktop, codex, or hermes"},
		{args: []string{"verify", "--for", "other"}, want: "--for must be claude, claude-desktop, codex, hermes, or all"},
		{args: []string{"setup", "--route", "new-route", "--for", "other"}, want: "--for must be claude, claude-desktop, codex, or hermes"},
		{args: []string{"route", "add", "new-route"}, want: "--account, --model, and --protocol are required"},
		{args: []string{"adapter"}, want: "unknown command \"adapter\""},
		{args: []string{"client", "enable", "other"}, want: "Client must be claude, claude-desktop, codex, or hermes"},
	} {
		out.Reset()
		err := cli.Execute(app, tc.args)
		if err == nil || !strings.Contains(out.String(), tc.want) {
			t.Fatalf("%v err=%v output=%s", tc.args, err, out.String())
		}
	}
}

func TestExecuteReturnsHumanOutputFailure(t *testing.T) {
	app, _, _, _, _ := testApp(t, "")
	want := errors.New("output is unavailable")
	app.Out = failingOutput{err: want}

	err := cli.Execute(app, []string{"client", "discover"})
	if !errors.Is(err, want) {
		t.Fatalf("Execute() error = %v, want %v", err, want)
	}
}

func TestFailureSuggestionUsesCommandNamedInEnglishGuidance(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	if err := app.Config.Save(twoRouteConfig()); err != nil {
		t.Fatal(err)
	}
	err := cli.Execute(app, []string{"setup", "--route", "new-route"})
	if err == nil || !strings.Contains(out.String(), "AIGW is already configured") || !strings.Contains(out.String(), "aigw add") {
		t.Fatalf("err=%v output=%s", err, out.String())
	}
}

func TestJSONCommandsShareReadableDocumentLayout(t *testing.T) {
	for _, args := range [][]string{
		{"status", "--json"},
		{"check", "--json"},
		{"doctor", "--json"},
		{"catalog", "--json"},
		{"route", "show", "claude", "--json"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			app, out, _, _, _ := testApp(t, "")
			cfg := domain.NewConfig(clients.Admission())
			addAccountRoute(&cfg, "claude", "one", "One", domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test")
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := cli.Execute(app, args); (err != nil) != (args[0] == "check") {
				t.Fatalf("unexpected command outcome: %v\n%s", err, out)
			}
			var expected bytes.Buffer
			if err := json.Indent(&expected, bytes.TrimSpace(out.Bytes()), "", "  "); err != nil {
				t.Fatalf("expected one JSON document: %v\n%s", err, out)
			}
			expected.WriteByte('\n')
			if !bytes.Equal(out.Bytes(), expected.Bytes()) {
				t.Fatalf("JSON layout differs from the shared document format:\n%s", out)
			}
		})
	}
}

func TestPublicMutationFailureDoesNotExposePrivatePath(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	saveCommandRoute(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "m")
	privatePath := app.Config.Path() + ".bak"
	if err := os.Mkdir(privatePath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"route", "add", "two", "--account", "one", "--model", "m2", "--protocol", "anthropic"}); err == nil {
		t.Fatal("route add accepted an unreadable backup target")
	}
	if strings.Contains(out.String(), privatePath) || !strings.Contains(out.String(), "aigw doctor") {
		t.Fatalf("public command exposed the private failure target: %s", out.String())
	}
	cfg, err := app.Config.Load()
	if err != nil || len(cfg.Routes) != 1 {
		t.Fatalf("mutation changed configuration after failed preflight: %+v, %v", cfg.Routes, err)
	}
}

// TestCommandGroupsShowTheirHelp holds bare `aigw config` to the other command
// groups: it shows the group's help and succeeds, and `aigw help <command>`
// shows that command's help.
func TestCommandGroupsShowTheirHelp(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"config"}, "Import and export configuration"},
		{[]string{"route"}, "Manage Routes"},
		{[]string{"account", "diagnostics"}, "aigw account diagnostics [command]"},
		{[]string{"help", "sync"}, "aigw sync"},
	} {
		app, out, _, _, _ := testApp(t, "")
		if err := cli.Execute(app, test.args); err != nil || !strings.Contains(out.String(), test.want) || !strings.Contains(out.String(), "Usage") {
			t.Fatalf("%v: %v\n%s", test.args, err, out.String())
		}
		// A group's help names its subcommands, not a usage line of its own.
		if strings.Contains(out.String(), "aigw config [flags]") || strings.Contains(out.String(), "aigw route [flags]") {
			t.Fatalf("%v: a group's help shows a usage line of its own:\n%s", test.args, out.String())
		}
	}
}

// TestUnknownCommandsRefuse holds an unknown subcommand of a command group, at
// any depth, and an unknown help topic to a refusal that names the help to
// read, where Cobra would show a nested group's help and succeed.
func TestUnknownCommandsRefuse(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"config", "bogus"}, "run `aigw config --help`"},
		{[]string{"route", "bogus"}, "run `aigw route --help`"},
		{[]string{"account", "diagnostics", "bogus"}, "run `aigw account diagnostics --help`"},
		{[]string{"help", "bogus"}, "run `aigw --help`"},
		{[]string{"help", "route", "bogus"}, "run `aigw --help`"},
	} {
		app, out, _, _, _ := testApp(t, "")
		err := cli.Execute(app, test.args)
		if err == nil || !strings.Contains(out.String(), test.want) || !strings.Contains(out.String(), "bogus") {
			t.Fatalf("%v: %v\n%s", test.args, err, out.String())
		}
	}
	// A subcommand's name after a help flag still shows help.
	app, out, _, _, _ := testApp(t, "")
	if err := cli.Execute(app, []string{"route", "--help", "list"}); err != nil || !strings.Contains(out.String(), "Usage") {
		t.Fatalf("route --help list: %v\n%s", err, out.String())
	}
}
