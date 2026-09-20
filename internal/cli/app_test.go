package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"

	"github.com/charmbracelet/x/ansi"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func configuredApp(t *testing.T) *App {
	t.Helper()
	store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
	cfg := configuration.NewConfig()
	cfg.Accounts["one"] = configuration.Account{Label: "One", Endpoints: configuration.Endpoints{OpenAIResponses: "http://127.0.0.1:1234/v1", Anthropic: "https://one.test"}}
	cfg.Profiles["one"] = configuration.Profile{Label: "One", Purpose: "Primary", Account: "one", Client: configuration.ClientCodex, Model: "gpt"}
	cfg.Routes[configuration.ClientCodex] = "one"
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	secretStore := secrets.NewMemoryStore()
	diagnostics, err := secrets.NewDiagnosticCredentialStore(secretStore)
	if err != nil {
		t.Fatal(err)
	}
	return &App{Config: store, Secrets: secretStore, Accounts: diagnostics, Out: out, Err: out}
}

func TestNewDefaultBuildsAFunctioningApp(t *testing.T) {
	app, err := NewDefault()
	if err != nil {
		t.Fatalf("NewDefault() error = %v", err)
	}
	if app.GOOS == "" || app.DataDir == "" || app.Version == "" {
		t.Fatalf("NewDefault() produced an incomplete app: %#v", app)
	}
	if app.Config.Path() == "" {
		t.Fatal("NewDefault() did not wire a config path")
	}
	if filepath.Base(app.Config.Path()) != "config.toml" {
		t.Fatalf("NewDefault() config path = %q, want the stable config.toml contract", app.Config.Path())
	}
	if app.Secrets == nil || app.Accounts == nil || app.Runner == nil || app.HTTP == nil || app.Prompt == nil || app.Discovery == nil || app.Updater == nil {
		t.Fatalf("NewDefault() left a required dependency nil: %#v", app)
	}
	if _, ok := app.Runner.(process.Runner); !ok {
		t.Fatalf("NewDefault() runner = %T, want process.Runner", app.Runner)
	}
	if app.Now == nil {
		t.Fatal("NewDefault() did not wire a clock")
	}
}

func TestExecuteReturnsBusyLockErrorWhenAnotherMutationHoldsIt(t *testing.T) {
	store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	unlock, err := store.Lock(ctx)
	if err != nil {
		t.Fatalf("acquire external lock: %v", err)
	}
	t.Cleanup(func() {
		if err := unlock(); err != nil {
			t.Error(err)
		}
	})

	app := &App{Config: store, Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	err = Execute(app, []string{"add", "dmx", "--for", "claude", "--model", "claude-test", "--anthropic-url", "https://example.test", "--token-stdin"})
	if err == nil || !strings.Contains(err.Error(), "retry after the other command finishes") {
		t.Fatalf("Execute() error = %v, want a busy-lock error", err)
	}
}

type failingWriter struct{ err error }

func (writer failingWriter) Write([]byte) (int, error) { return 0, writer.err }

func TestExecuteCredentialFailureStaysMachineReadable(t *testing.T) {
	app := configuredApp(t)
	var stdout, stderr bytes.Buffer
	app.Out = &stdout
	app.Err = &stderr
	err := Execute(app, []string{"credential", "unsupported"})
	if err == nil || !strings.Contains(stderr.String(), "aigw sync") || stdout.Len() != 0 {
		t.Fatalf("error=%v stderr=%q", err, stderr.String())
	}
}

func TestExecuteCredentialFailurePreservesDiagnosticWriteError(t *testing.T) {
	app := configuredApp(t)
	writeErr := errors.New("diagnostic writer failed")
	app.Err = failingWriter{err: writeErr}
	if err := Execute(app, []string{"credential", "unsupported"}); !errors.Is(err, writeErr) {
		t.Fatalf("credential diagnostic lost write failure: %v", err)
	}
}

func TestExecuteCredentialStaleProjectionExplainsRecovery(t *testing.T) {
	app := configuredApp(t)
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "codex"}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	app.Out = &stdout
	app.Err = &stderr
	err = Execute(app, []string{"credential", configuration.ClientCodex, "stale-projection"})
	if err == nil || stdout.Len() != 0 {
		t.Fatalf("error=%v stdout=%q", err, app.Out)
	}
	for _, expected := range []string{"no longer matches", "aigw sync", "reload"} {
		if !strings.Contains(stderr.String(), expected) {
			t.Fatalf("credential diagnostic lacks %q: %q", expected, stderr.String())
		}
	}
}

func TestExecuteReturnsRendererFailureAfterSuccessfulCommand(t *testing.T) {
	want := errors.New("output unavailable")
	app := configuredApp(t)
	app.Out = failingWriter{err: want}
	app.Err = io.Discard
	if err := Execute(app, []string{"status"}); err == nil || !errors.Is(err, want) {
		t.Fatalf("renderer error = %v", err)
	}
}

func TestFinishExecutionPreservesCommandAndUnlockFailures(t *testing.T) {
	commandErr := errors.New("command failed")
	unlockErr := errors.New("unlock failed")
	if err := finishExecution(commandErr, func() error { return unlockErr }); err == nil || !errors.Is(err, commandErr) || !strings.Contains(err.Error(), "release config lock") {
		t.Fatalf("combined error = %v", err)
	}
	if err := finishExecution(nil, func() error { return unlockErr }); err == nil || !errors.Is(err, unlockErr) {
		t.Fatalf("unlock error = %v", err)
	}
	if err := finishExecution(commandErr, nil); !errors.Is(err, commandErr) {
		t.Fatalf("command error = %v", err)
	}
}

func TestExecuteReturnsConfigurationLoadFailure(t *testing.T) {
	out := new(bytes.Buffer)
	app := &App{Config: configuration.NewStore(t.TempDir()), Out: out, Err: out}
	if err := Execute(app, nil); err == nil {
		t.Fatal("expected configuration load error")
	}
}

func TestCommandHelpIsProjectedFromCobraMetadata(t *testing.T) {
	out := new(bytes.Buffer)
	app := &App{Out: out, Err: out}
	command := &cobra.Command{Use: "semantic-root", Short: "source-owned summary"}
	command.AddCommand(
		&cobra.Command{Use: "zeta", Short: "source-owned Z", Run: func(*cobra.Command, []string) {}},
		&cobra.Command{Use: "alpha", Short: "source-owned A", Run: func(*cobra.Command, []string) {}},
		&cobra.Command{Use: "internal", Hidden: true, Run: func(*cobra.Command, []string) {}},
	)
	command.Flags().String("visible", "", "visible option")
	command.Flags().String("internal", "", "internal option")
	if err := command.Flags().MarkHidden("internal"); err != nil {
		t.Fatal(err)
	}

	renderCommandHelp(app, command)
	help := out.String()
	for _, want := range []string{
		"source-owned summary",
		"semantic-root [command]",
		"alpha",
		"source-owned A",
		"zeta",
		"source-owned Z",
		"Commands",
		"visible option",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("help does not project Cobra metadata %q: %q", want, help)
		}
	}
	if strings.Index(help, "alpha") > strings.Index(help, "zeta") {
		t.Fatalf("extension commands are not ordered: %q", help)
	}
	if strings.Contains(help, "internal option") || strings.Contains(help, "internal  ") {
		t.Fatalf("help exposes an internal surface: %q", help)
	}
	originalSorting := cobra.EnableCommandSorting
	cobra.EnableCommandSorting = false
	t.Cleanup(func() { cobra.EnableCommandSorting = originalSorting })
	command.AddCommand(&cobra.Command{Use: "beta", Short: "source-owned B", Run: func(*cobra.Command, []string) {}})
	out.Reset()
	renderCommandHelp(app, command)
	help = out.String()
	if strings.Index(help, "zeta") > strings.Index(help, "beta") {
		t.Fatalf("help overrides Cobra's explicit command ordering: %q", help)
	}
}

func TestCommandHelpIncludesNativeFlagMetadataAndExamples(t *testing.T) {
	for _, columns := range []string{"0", "48"} {
		t.Run(columns, func(t *testing.T) {
			var out bytes.Buffer
			app := &App{Out: &out, Err: &out, Env: []string{"COLUMNS=" + columns}}
			root := &cobra.Command{Use: "service"}
			root.PersistentFlags().String("region", "team", "Choose the service region")
			command := &cobra.Command{
				Use:     "connect",
				Short:   "Connect a service",
				Long:    "Choose a service without changing credentials.\nExisting selections remain available.",
				Example: "service connect --target local\nservice connect --region team",
			}
			root.AddCommand(command)
			command.Flags().String("target", "local", "Use the selected `path`")
			command.Flags().IntP("attempts", "a", 3, "Bound the connection attempts")
			command.Flags().Duration("timeout", 3*time.Second, "Bound the connection duration")
			command.Flags().Bool("verify", true, "Verify the selected endpoint")
			if err := command.Flags().Set("attempts", "7"); err != nil {
				t.Fatal(err)
			}
			renderCommandHelp(app, command)
			text := strings.Join(strings.Fields(out.String()), " ")
			for _, want := range []string{
				command.Long, command.Example, "Examples", "Inherited options",
				command.NonInheritedFlags().FlagUsages(), command.InheritedFlags().FlagUsages(),
			} {
				if !strings.Contains(text, strings.Join(strings.Fields(want), " ")) {
					t.Fatalf("help missing native metadata %q:\n%s", want, &out)
				}
			}
			if columns == "48" {
				for line := range strings.SplitSeq(strings.TrimRight(command.NonInheritedFlags().FlagUsagesWrapped(46), "\n"), "\n") {
					if !strings.Contains(out.String(), "  "+strings.TrimRight(line, " \t")+"\n") {
						t.Fatalf("help changed native narrow option layout: %q\n%s", line, &out)
					}
				}
				for line := range strings.SplitSeq(out.String(), "\n") {
					if presentation.DisplayWidth(line) > 48 {
						t.Fatalf("help line exceeds available width: %q", line)
					}
				}
			}
			out.Reset()
			command.Example = "service connect --target 'a-long-path-with-two  spaces'\nservice connect --region 'tab\tvalue'"
			renderCommandHelp(app, command)
			want := "  " + strings.ReplaceAll(command.Example, "\n", "\n  ") + "\n"
			if !strings.Contains(out.String(), want) {
				t.Fatalf("help rewrote copyable example bytes:\n%q", out.String())
			}
		})
	}
}

func TestGroupedHelpUsesDeclaredOrderAndTitles(t *testing.T) {
	out := new(bytes.Buffer)
	app := &App{Out: out, Err: out}
	command := &cobra.Command{Use: "workflow"}
	command.AddGroup(
		&cobra.Group{ID: "second", Title: "Prepare the workspace"},
		&cobra.Group{ID: "first", Title: "Run the application"},
	)
	command.AddCommand(
		&cobra.Command{Use: "launch", Short: "Run the chosen service", GroupID: "first", Run: func(*cobra.Command, []string) {}},
		&cobra.Command{Use: "prepare", Short: "Connect the chosen account", GroupID: "second", Run: func(*cobra.Command, []string) {}},
		&cobra.Command{Use: "inspect", Short: "Inspect without changing state", Run: func(*cobra.Command, []string) {}},
	)

	renderCommandHelp(app, command)
	remaining := out.String()
	for _, want := range []string{"Prepare the workspace", "prepare", "Connect the chosen account", "Run the application", "launch", "Run the chosen service", "Commands", "inspect", "Inspect without changing state"} {
		_, after, found := strings.Cut(remaining, want)
		if !found {
			t.Fatalf("help does not project declared group sequence at %q:\n%s", want, out.String())
		}
		remaining = after
	}
}

func TestRootHelpPresentsTheOrderedUserJourney(t *testing.T) {
	out := new(bytes.Buffer)
	app := &App{Out: out, Err: out}
	if err := Execute(app, []string{"--help"}); err != nil {
		t.Fatal(err)
	}
	remaining := out.String()
	for _, want := range []string{
		"Start with one path",
		"aigw setup", "Connect the first account",
		"aigw use <profile>", "Select this profile for its client",
		"aigw check", "Confirm readiness",
		"Usage", "aigw [command]",
		"Connect", "setup",
		"Use every day", "check", "rotate", "status", "use",
		"Recover", "doctor", "install", "repair", "rollback", "sync", "uninstall", "update",
		"Advanced", "account", "adapter", "add", "balance", "catalog", "completion", "config", "models", "profile", "route", "test", "verify",
		"Options", "show help", "show version",
	} {
		_, after, found := strings.Cut(remaining, want)
		if !found {
			t.Fatalf("help journey is missing or misorders %q:\n%s", want, out.String())
		}
		remaining = after
	}
}

func TestRootHelpSeparatesCommandsFromDescriptions(t *testing.T) {
	for _, width := range []int{32, 80, 120} {
		for _, color := range []bool{false, true} {
			var out bytes.Buffer
			app := &App{Out: &out, Err: &out, Color: color, Env: []string{"COLUMNS=" + strconv.Itoa(width)}}
			command := NewRoot(app)
			command.Use = "gateway"
			renderCommandHelp(app, command)
			_, help, found := strings.Cut(ansi.Strip(out.String()), "Start with one path\n")
			if !found {
				t.Fatal("root help omitted the starting journey")
			}
			help, _, _ = strings.Cut(help, "\nUsage")
			column := -1
			for _, row := range [][2]string{
				{"gateway setup", "Connect the first account"},
				{"gateway use <profile>", "Select this profile for its client"},
				{"gateway check", "Confirm readiness"},
			} {
				if !strings.Contains(help, row[0]) || !strings.Contains(strings.Join(strings.Fields(help), " "), row[1]) {
					t.Fatalf("width=%d color=%t lost command or description %q:\n%s", width, color, row, help)
				}
				for line := range strings.SplitSeq(help, "\n") {
					if prefix, _, present := strings.Cut(line, row[1]); present && width >= 80 {
						position := presentation.DisplayWidth(prefix)
						if column >= 0 && position != column {
							t.Fatalf("descriptions use different display columns %d/%d:\n%s", column, position, help)
						}
						column = position
					}
				}
			}
			if strings.Contains(help, "#") {
				t.Fatalf("help embeds explanations as executable shell comments: %q", help)
			}
		}
	}
}

func TestPublicCommandTreeCarriesOneCoherentMetadataContract(t *testing.T) {
	root := NewRoot(&App{Out: io.Discard, Err: io.Discard})
	identifier := regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	paths := make(map[string]struct{})

	var inspect func(*cobra.Command)
	inspect = func(command *cobra.Command) {
		if command.Hidden {
			return
		}
		path := command.CommandPath()
		if _, duplicate := paths[path]; duplicate {
			t.Fatalf("duplicate public command path %q", path)
		}
		paths[path] = struct{}{}
		if !identifier.MatchString(command.Name()) {
			t.Errorf("command %q has a non-canonical name", path)
		}
		if command != root {
			summary := strings.TrimSpace(command.Short)
			if summary == "" || !unicode.IsUpper([]rune(summary)[0]) {
				t.Errorf("command %q summary = %q, want a concise sentence beginning with an uppercase letter", path, command.Short)
			}
		}
		command.NonInheritedFlags().VisitAll(func(flag *pflag.Flag) {
			if flag.Hidden {
				return
			}
			if !identifier.MatchString(flag.Name) || strings.TrimSpace(flag.Usage) == "" {
				t.Errorf("command %q option --%s lacks canonical source-owned metadata", path, flag.Name)
			}
		})
		for _, child := range command.Commands() {
			inspect(child)
		}
	}
	inspect(root)

	add, _, err := root.Find([]string{"add"})
	if err != nil {
		t.Fatal(err)
	}
	if add.Use != "add <account>" || add.Short != "Add and connect one Account with its first Profile" {
		t.Fatalf("add metadata = %q / %q", add.Use, add.Short)
	}
}

func TestPublicCommandHelpFitsDeclaredTerminalWidths(t *testing.T) {
	for _, width := range []int{24, 48, 80, 120} {
		for _, color := range []bool{false, true} {
			var out bytes.Buffer
			app := &App{Out: &out, Err: &out, Color: color, Env: []string{"COLUMNS=" + strconv.Itoa(width)}}
			var visit func(*cobra.Command)
			visit = func(command *cobra.Command) {
				out.Reset()
				renderCommandHelp(app, command)
				var reference bytes.Buffer
				renderCommandHelp(&App{Out: &reference, Err: io.Discard}, command)
				actual := strings.ReplaceAll(ansi.Strip(out.String()), "─", "")
				original := strings.ReplaceAll(reference.String(), "─", "")
				if strings.Join(strings.Fields(actual), "") != strings.Join(strings.Fields(original), "") {
					t.Fatalf("%s width=%d color=%t changed help content", command.CommandPath(), width, color)
				}
				for line := range strings.SplitSeq(out.String(), "\n") {
					if got := presentation.DisplayWidth(line); got > width {
						t.Fatalf("%s width=%d color=%t overflow=%d: %q", command.CommandPath(), width, color, got, line)
					}
				}
				for _, child := range command.Commands() {
					if !child.Hidden {
						visit(child)
					}
				}
			}
			visit(NewRoot(app))
		}
	}
}

func TestCriticalCommandHelpUsesEnglishGuidance(t *testing.T) {
	out := new(bytes.Buffer)
	app := &App{Out: out, Err: out}
	cases := []struct {
		args []string
		want []string
	}{
		{args: []string{"setup", "--help"}, want: []string{"Account ID; uses the first Profile ID when omitted", "First profile ID", "Read one token line from standard input"}},
		{args: []string{"test", "--help"}, want: []string{"Test selected endpoints", "Test the selected Route for Claude, Codex, or Hermes"}},
		{args: []string{"models", "--help"}, want: []string{"Compare configured model IDs with provider catalogs", "does not test inference"}},
		{args: []string{"verify", "--help"}, want: []string{"Verify the selected Route for Claude, Codex, Hermes, or all", "Verify one Profile using its declared client without changing Routes"}},
		{args: []string{"rotate", "--help"}, want: []string{"Update one Account Token"}},
		{args: []string{"completion", "--help"}, want: []string{"Generate shell completion"}},
		{args: []string{"rollback", "--help"}, want: []string{"Restore only the immediately previous configuration backup"}},
		{args: []string{"config", "import", "--help"}, want: []string{"Merge a secret-free configuration manifest", "Explicitly replace conflicting account metadata", "system tokens remain unchanged"}},
	}
	for _, tc := range cases {
		out.Reset()
		if err := Execute(app, tc.args); err != nil {
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

func TestCommandHelpLeavesConfigurationStorageAbsent(t *testing.T) {
	var visit func(*cobra.Command, []string)
	visit = func(command *cobra.Command, path []string) {
		t.Run(command.CommandPath(), func(t *testing.T) {
			configRoot := filepath.Join(t.TempDir(), "configuration")
			var output bytes.Buffer
			app := &App{
				Config: configuration.NewStore(filepath.Join(configRoot, "aigw.toml")),
				Out:    &output,
				Err:    io.Discard,
			}
			if err := Execute(app, append(append([]string{}, path...), "--help")); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(configRoot); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("help created configuration storage: %v", err)
			}
			text := strings.Join(strings.Fields(output.String()), " ")
			if command.Runnable() && !strings.Contains(text, command.UseLine()) {
				t.Fatalf("help omitted runnable usage %q:\n%s", command.UseLine(), &output)
			}
			if command.HasAvailableSubCommands() && !strings.Contains(text, command.CommandPath()+" [command]") {
				t.Fatalf("help omitted subcommand usage for %s:\n%s", command.CommandPath(), &output)
			}
			command.InitDefaultHelpFlag()
			for _, flags := range []*pflag.FlagSet{command.NonInheritedFlags(), command.InheritedFlags()} {
				if !strings.Contains(text, strings.Join(strings.Fields(flags.FlagUsages()), " ")) {
					t.Fatalf("help omitted source-owned options:\n%s", &output)
				}
			}
		})
		for _, child := range command.Commands() {
			visit(child, append(append([]string{}, path...), child.Name()))
		}
	}
	visit(NewRoot(&App{Out: io.Discard, Err: io.Discard}), nil)
}

func TestCompletionSupportsDocumentedShells(t *testing.T) {
	out := new(bytes.Buffer)
	app := &App{Out: out, Err: out}
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		out.Reset()
		if err := Execute(app, []string{"completion", shell}); err != nil {
			t.Fatalf("%s completion: %v", shell, err)
		}
		if out.Len() == 0 {
			t.Fatalf("%s completion produced no output", shell)
		}
	}
	if err := Execute(app, []string{"completion", "unsupported"}); err == nil {
		t.Fatal("expected unsupported shell error")
	}
}
