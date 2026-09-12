package cli_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"aigw-cli/internal/cli"
)

func TestJSONCommandFailuresRemainMachineReadable(t *testing.T) {
	for _, args := range [][]string{
		{"status", "--json"},
		{"sync", "--json"},
		{"repair", "--json"},
		{"catalog", "--json"},
		{"setup", "--json"},
		{"profile", "rename", "old", "new", "--dry-run", "--json"},
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
			}
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatalf("expected one JSON failure: %v\n%s", err, out)
			}
			if result.OK || result.Error == "" || result.NextAction == "" || stderr.Len() != 0 {
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
		{args: []string{"config"}, want: "Choose a config subcommand; run `aigw config --help`", fix: "aigw config --help"},
		{args: []string{"use", "--for", "other", "one"}, want: "unknown option --for", fix: "aigw --help"},
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
}

func TestUnknownFlagSuggestsTopLevelHelp(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	err := cli.Execute(app, []string{"status", "--not-a-flag"})
	if err == nil || !strings.Contains(out.String(), "unknown option") || !strings.Contains(out.String(), "aigw --help") {
		t.Fatalf("err=%v output=%s", err, out.String())
	}
}

func TestCoreValidationFailuresUseEnglishGuidance(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	for _, tc := range []struct {
		args []string
		want string
	}{
		{args: []string{"test", "--for", "other"}, want: "--for must be claude or codex"},
		{args: []string{"verify", "--for", "other"}, want: "--for must be claude, codex, or all"},
		{args: []string{"setup", "--profile", "new-profile", "--for", "other"}, want: "--for must be claude or codex"},
		{args: []string{"profile", "add", "new-profile"}, want: "--account, --for, and --model are required"},
		{args: []string{"route", "reset", "other"}, want: "unknown command \"reset\""},
		{args: []string{"adapter", "enable", "other"}, want: "Client must be claude or codex"},
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

	err := cli.Execute(app, []string{"adapter", "discover"})
	if !errors.Is(err, want) {
		t.Fatalf("Execute() error = %v, want %v", err, want)
	}
}

func TestFailureSuggestionUsesCommandNamedInEnglishGuidance(t *testing.T) {
	app, out, _, _, _ := testApp(t, "")
	if err := app.Config.Save(twoProfileConfig()); err != nil {
		t.Fatal(err)
	}
	err := cli.Execute(app, []string{"setup", "--profile", "new-profile"})
	if err == nil || !strings.Contains(out.String(), "AIGW is already configured") || !strings.Contains(out.String(), "aigw add") {
		t.Fatalf("err=%v output=%s", err, out.String())
	}
}
