package cli_test

import (
	"aigw-cli/internal/presentation"
	"errors"
	"strings"
	"testing"
)

func TestHelpKeepsDailyCommandsObvious(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	if err := execute(t, app, "--help"); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"use", "rotate", "check", "repair", "update"} {
		if !strings.Contains(out.String(), command) {
			t.Fatalf("help lacks %s:\n%s", command, out.String())
		}
	}
	for _, unwanted := range []string{"Usage:", "Additional Commands:", "Flags:"} {
		if strings.Contains(out.String(), unwanted) {
			t.Fatalf("help contains legacy Cobra scaffold %q:\n%s", unwanted, out.String())
		}
	}
	for _, wanted := range []string{"Start with one path", "Usage", "Connect", "Use every day", "Recover", "Advanced", "Options", "show help", "show version"} {
		if !strings.Contains(out.String(), wanted) {
			t.Fatalf("help lacks expected section %q:\n%s", wanted, out.String())
		}
	}
}

func TestRootHelpOrganizesTasksBeforeAdministration(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	if err := execute(t, app, "--help"); err != nil {
		t.Fatal(err)
	}
	help := out.String()
	for _, want := range []string{"Connect", "Use every day", "Recover", "Advanced", "setup", "use", "check", "doctor", "repair"} {
		if !strings.Contains(help, want) {
			t.Fatalf("help lacks %q:\n%s", want, help)
		}
	}
	positions := []int{
		strings.Index(help, "Connect"),
		strings.Index(help, "Use every day"),
		strings.Index(help, "Recover"),
		strings.Index(help, "Advanced"),
	}
	for index, position := range positions {
		if position < 0 || (index > 0 && positions[index-1] >= position) {
			t.Fatalf("task headings are not ordered:\n%s", help)
		}
	}
}

func TestRootHelpUsesCompactRowsWhenColumnsAreNarrow(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	app.Env = []string{"COLUMNS=48"}
	if err := execute(t, app, "--help"); err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimRight(out.String(), "\n"), "\n") {
		if got := presentation.DisplayWidth(line); got > 48 {
			t.Fatalf("help line width = %d, want <= 48: %q\n%s", got, line, out.String())
		}
	}
}

func TestRootHelpPresentsAThreeStepJourney(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	if err := execute(t, app, "--help"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"aigw setup    # connect the first service", "aigw use      # choose the active service", "aigw check    # confirm readiness"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("help lacks %q:\n%s", want, out.String())
		}
	}
}

func TestCriticalCommandHelpUsesEnglishGuidance(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	cases := []struct {
		args []string
		want []string
	}{
		{args: []string{"setup", "--help"}, want: []string{"Account ID", "First profile ID", "Read one token line from standard input"}},
		{args: []string{"verify", "--help"}, want: []string{"Verify Claude, Codex, or all clients", "Verify a specified profile without changing routes"}},
		{args: []string{"rollback", "--help"}, want: []string{"Restore only the immediately previous configuration backup"}},
		{args: []string{"config", "import", "--help"}, want: []string{"Merge a secret-free configuration manifest", "Explicitly replace conflicting account metadata", "system tokens remain unchanged"}},
		{args: []string{"adapter", "auth", "--help"}, want: []string{"Bind the current account token to Codex"}},
	}
	for _, tc := range cases {
		out.Reset()
		if err := execute(t, app, tc.args...); err != nil {
			t.Fatalf("%v: %v", tc.args, err)
		}
		help := out.String()
		for _, want := range tc.want {
			if !strings.Contains(help, want) {
				t.Fatalf("%v help missing %q:\n%s", tc.args, want, help)
			}
		}
	}
}

func TestCommonCommandFailuresUseEnglishGuidance(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	for _, tc := range []struct {
		args []string
		want string
		fix  string
	}{
		{args: []string{"config"}, want: "Choose a config subcommand; run `aigw config --help`", fix: "aigw config --help"},
		{args: []string{"adapter", "auth", "claude"}, want: "Native credential binding is available only for codex", fix: "aigw adapter auth codex"},
		{args: []string{"use", "--for", "other", "one"}, want: "unknown option --for", fix: "aigw --help"},
	} {
		out.Reset()
		err := execute(t, app, tc.args...)
		if err == nil || !strings.Contains(out.String(), tc.want) || !strings.Contains(out.String(), tc.fix) {
			t.Fatalf("%v err=%v output=%s", tc.args, err, out.String())
		}
	}
}

func TestUnknownCommandSuggestsTopLevelHelp(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	err := execute(t, app, "not-a-command")
	if err == nil || !strings.Contains(out.String(), "unknown command") || !strings.Contains(out.String(), "aigw --help") {
		t.Fatalf("err=%v output=%s", err, out.String())
	}
}

func TestUnknownFlagSuggestsTopLevelHelp(t *testing.T) {
	app, out, _, _ := testApp(t, "")
	err := execute(t, app, "status", "--not-a-flag")
	if err == nil || !strings.Contains(out.String(), "unknown option") || !strings.Contains(out.String(), "aigw --help") {
		t.Fatalf("err=%v output=%s", err, out.String())
	}
}

func TestCoreValidationFailuresUseEnglishGuidance(t *testing.T) {
	app, out, _, _ := testApp(t, "")
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
		err := execute(t, app, tc.args...)
		if err == nil || !strings.Contains(out.String(), tc.want) {
			t.Fatalf("%v err=%v output=%s", tc.args, err, out.String())
		}
	}
}

func TestExecuteReturnsHumanOutputFailure(t *testing.T) {
	app, _, _, _ := testApp(t, "")
	want := errors.New("output is unavailable")
	app.Out = failingOutput{err: want}

	err := execute(t, app, "adapter", "discover")
	if !errors.Is(err, want) {
		t.Fatalf("Execute() error = %v, want %v", err, want)
	}
}
