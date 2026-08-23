// Package cli composes the AIGW command tree and owns root-level workflows.
// Cohesive command groups live in semantic subpackages below this package.
package cli

import (
	"context"
	"fmt"
	"github.com/spf13/pflag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"aigw-cli/internal/account"
	accountcli "aigw-cli/internal/cli/account"
	"aigw-cli/internal/cli/adapter"
	"aigw-cli/internal/cli/catalog"
	credentialcli "aigw-cli/internal/cli/credential"
	"aigw-cli/internal/cli/doctor"
	installcli "aigw-cli/internal/cli/install"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/cli/manifest"
	"aigw-cli/internal/cli/onboarding"
	"aigw-cli/internal/cli/profile"
	"aigw-cli/internal/cli/readiness"
	"aigw-cli/internal/cli/recovery"
	"aigw-cli/internal/cli/route"
	updatecli "aigw-cli/internal/cli/update"
	"aigw-cli/internal/cli/verification"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/console"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/platform"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/process"
	"aigw-cli/internal/prompt"
	"aigw-cli/internal/renaming"
	"aigw-cli/internal/secrets"
	"aigw-cli/internal/synchronization"
	"aigw-cli/internal/upgrade"
	"github.com/spf13/cobra"
)

type Runner interface {
	Run(context.Context, process.Plan) error
}

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type Prompter interface {
	Secret(label string) (string, error)
	Text(label string) (string, error)
	Select(label string, choices []prompt.Choice) (string, error)
}

type Updater interface {
	Update(context.Context, string) (string, error)
	UpdateCandidate(context.Context, string, upgrade.CandidateArchive) (string, error)
	Rollback(context.Context) (string, error)
}

type App struct {
	GOOS               string
	DataDir            string
	Now                func() time.Time
	Version            string
	Executable         string
	InstallTarget      string
	ClaudeSettingsPath string
	Config             configuration.Store
	Secrets            secrets.Store
	Accounts           account.Store
	Env                []string
	In                 io.Reader
	Out                io.Writer
	Err                io.Writer
	Interactive        bool
	Color              bool
	Runner             Runner
	HTTP               HTTPDoer
	Prompt             Prompter
	Discovery          discovery.Discoverer
	Updater            Updater
	renderErr          error
}

// synchronizer is the CLI composition boundary for the synchronization
// domain. It assembles dependencies only; synchronization behavior remains in
// internal/synchronization.
func (a *App) synchronizer() synchronization.Synchronizer {
	return synchronization.Synchronizer{
		Config: a.Config, Secrets: a.Secrets, Runner: a.Runner, Discovery: a.Discovery,
		ClaudeSettingsPath: a.ClaudeSettingsPath,
		AIGWExecutable:     a.Executable,
	}
}

type renderErrorWriter struct {
	writer io.Writer
	err    *error
}

func (w renderErrorWriter) Write(data []byte) (int, error) {
	count, writeErr := w.writer.Write(data)
	if writeErr != nil && *w.err == nil {
		*w.err = writeErr
	}
	return count, writeErr
}

func (a *App) Renderer() *presentation.Renderer {
	return presentation.NewWithWidth(renderErrorWriter{writer: a.Out, err: &a.renderErr}, a.Color, console.PresentationWidth(a.Out, environmentMap(a.Env)))
}

func renderer(app *App) *presentation.Renderer { return app.Renderer() }

func Execute(app *App, args []string) error {
	app.renderErr = nil
	var unlock func() error
	if mutationCommand(app, args) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		locked, err := app.Config.Lock(ctx)
		if err != nil {
			return fmt.Errorf("%w; retry after the other command finishes", err)
		}
		unlock = locked
	}
	root := NewRoot(app)
	root.SetArgs(args)
	err := root.Execute()
	if err != nil && credentialInvocation(args) {
		return err
	}
	if err == nil && app.renderErr != nil {
		err = app.renderErr
	}
	err = finishExecution(err, unlock)
	if err != nil {
		presentation.RenderError(app.Renderer(), err)
		return presentation.Presented(err)
	}
	return nil
}

func finishExecution(commandErr error, unlock func() error) error {
	if unlock == nil {
		return commandErr
	}
	unlockErr := unlock()
	if unlockErr == nil {
		return commandErr
	}
	if commandErr == nil {
		return fmt.Errorf("release config lock: %w", unlockErr)
	}
	return fmt.Errorf("%w; release config lock: %v", commandErr, unlockErr)
}

func credentialInvocation(args []string) bool {
	return len(args) > 0 && args[0] == "credential"
}

func mutationCommand(app *App, args []string) bool {
	if len(args) == 0 {
		cfg, err := app.Config.Load()
		return err == nil && len(cfg.Profiles) == 0 && app.Interactive
	}
	switch args[0] {
	case "setup", "add", "use", "rotate", "rollback":
		return true
	case "sync":
		return !boolArgumentEnabled(args[1:], "--dry-run")
	case "repair":
		return !boolArgumentEnabled(args[1:], "--dry-run")
	case "update":
		return true
	case "account":
		if len(args) < 2 {
			return false
		}
		if args[1] == "rename" {
			return !boolArgumentEnabled(args[2:], "--dry-run")
		}
		return args[1] == "connect" || args[1] == "disconnect" || args[1] == "edit"
	case "profile":
		if len(args) < 2 {
			return false
		}
		if args[1] == "rename" {
			return !boolArgumentEnabled(args[2:], "--dry-run")
		}
		return args[1] == "add" || args[1] == "edit" || args[1] == "remove"
	case "route":
		return len(args) > 1 && args[1] == "reset"
	case "adapter":
		return len(args) > 1 && (args[1] == "enable" || args[1] == "auth" || args[1] == "disable")
	case "config":
		return len(args) > 1 && args[1] == "import"
	default:
		return false
	}
}

func boolArgumentEnabled(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
		if !strings.HasPrefix(value, want+"=") {
			continue
		}
		enabled, err := strconv.ParseBool(strings.TrimPrefix(value, want+"="))
		return err == nil && enabled
	}
	return false
}

func NewDefault() (*App, error) {
	env := environmentMap(os.Environ())
	paths, err := platform.PathsFor(runtime.GOOS, env)
	if err != nil {
		return nil, err
	}
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve AIGW executable: %w", err)
	}
	secretStore, err := secrets.Select(secrets.Selection{
		Backend: env["AIGW_SECRET_BACKEND"],
		GOOS:    runtime.GOOS,
		Root:    paths.Secrets,
		Getenv:  os.Getenv,
	})
	if err != nil {
		return nil, err
	}
	return &App{
		GOOS:               runtime.GOOS,
		DataDir:            paths.Data,
		Now:                time.Now,
		Version:            Version,
		Executable:         executable,
		InstallTarget:      filepath.Join(paths.InstallDir, paths.InstallName),
		ClaudeSettingsPath: paths.ClaudeSettings,
		Config:             configuration.NewStore(paths.Config),
		Secrets:            secretStore,
		Accounts:           account.NewKeyringStore(),
		Env:                os.Environ(),
		In:                 os.Stdin,
		Out:                os.Stdout,
		Err:                os.Stderr,
		Interactive:        console.Interactive(os.Stdin),
		Color:              console.ColorEnabled(runtime.GOOS, env, console.Interactive(os.Stdout), console.EnableVirtualTerminal),
		Runner:             process.Runner{},
		HTTP:               &http.Client{},
		Prompt:             prompt.New(os.Stdin, os.Stdout, env["NO_COLOR"] != ""),
		Discovery:          discovery.Current(),
		Updater:            upgrade.Current(executable),
	}, nil
}

func environmentMap(values []string) map[string]string {
	out := map[string]string{}
	for _, value := range values {
		key, v, ok := strings.Cut(value, "=")
		if ok {
			out[key] = v
		}
	}
	return out
}

func (a *App) doctorCommand() *cobra.Command {
	return doctor.NewCommand(doctor.Dependencies{
		Config: a.Config, Secrets: a.Secrets, Env: a.Env, Out: a.Out,
		RenderOut: renderErrorWriter{writer: a.Out, err: &a.renderErr},
		Color:     a.Color, Width: console.PresentationWidth(a.Out, environmentMap(a.Env)),
	})
}

func (a *App) catalogDependencies() catalog.Dependencies {
	return catalog.Dependencies{
		Config: a.Config, Secrets: a.Secrets, HTTP: a.HTTP, Out: a.Out,
		RenderOut: renderErrorWriter{writer: a.Out, err: &a.renderErr},
		Color:     a.Color, Width: console.PresentationWidth(a.Out, environmentMap(a.Env)),
	}
}

func (a *App) invocationContext() invocation.Context {
	return invocation.Context{
		Version: appVersion(a), Executable: a.Executable, InstallTarget: a.InstallTarget,
		ClaudeSettingsPath: a.ClaudeSettingsPath,
		Config:             a.Config, Secrets: a.Secrets, Accounts: a.Accounts, Out: a.Out,
		In:        a.In,
		RenderOut: renderErrorWriter{writer: a.Out, err: &a.renderErr},
		Color:     a.Color, Width: console.PresentationWidth(a.Out, environmentMap(a.Env)), Interactive: a.Interactive,
		Runner: a.Runner, HTTP: a.HTTP, Prompt: a.Prompt,
		Discovery: a.Discovery, Updater: a.Updater, Now: a.Now, Problem: presentation.ProblemError,
	}
}

func (a *App) renamingDependencies() renaming.Dependencies {
	return renaming.Dependencies{
		Config: a.Config, Secrets: a.Secrets, Accounts: a.Accounts,
		Out:   renderErrorWriter{writer: a.Out, err: &a.renderErr},
		Color: a.Color, Width: console.PresentationWidth(a.Out, environmentMap(a.Env)),
		Interactive: a.Interactive, Prompt: a.Prompt, HTTP: a.HTTP,
		Synchronizer: a.synchronizer(),
	}
}

var Version = "0.1.0-dev"

func NewRoot(app *App) *cobra.Command {
	root := &cobra.Command{
		Use:           "aigw",
		Short:         "Local AI provider configuration, routing, and diagnostics",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			if len(cfg.Profiles) == 0 && app.Interactive {
				return onboarding.RunWizard(cmd.Context(), app.invocationContext())
			}
			return readiness.RunStatus(app.invocationContext(), false)
		},
	}
	root.SetOut(app.Out)
	root.SetErr(app.Err)
	root.Version = appVersion(app)
	root.InitDefaultHelpFlag()
	if flag := root.Flags().Lookup("help"); flag != nil {
		flag.Usage = "show help"
	}
	root.InitDefaultVersionFlag()
	if flag := root.Flags().Lookup("version"); flag != nil {
		flag.Usage = "show version"
	}
	root.SetHelpFunc(func(command *cobra.Command, _ []string) { renderCommandHelp(app, command) })
	root.SetHelpCommand(&cobra.Command{Use: "help [command]", Short: "show command help", Hidden: true})
	root.CompletionOptions.DisableDefaultCmd = true
	root.AddGroup(
		&cobra.Group{ID: "connect", Title: "Connect"},
		&cobra.Group{ID: "daily", Title: "Use every day"},
		&cobra.Group{ID: "recover", Title: "Recover"},
		&cobra.Group{ID: "advanced", Title: "Advanced"},
	)
	runtime := app.invocationContext()
	connect := []*cobra.Command{onboarding.NewCommand(runtime)}
	daily := []*cobra.Command{readiness.NewStatusCommand(runtime), route.NewUseCommand(runtime), readiness.NewCheckCommand(runtime), accountcli.NewRotateCommand(runtime)}
	recover := []*cobra.Command{
		app.doctorCommand(), recovery.NewRepairCommand(runtime), recovery.NewSyncCommand(runtime),
		recovery.NewRollbackCommand(runtime), updatecli.NewCommand(runtime),
		installcli.NewInstallCommand(runtime), installcli.NewUninstallCommand(runtime),
	}
	advanced := []*cobra.Command{
		accountcli.NewAddCommand(runtime), accountcli.NewCommand(runtime, renaming.NewAccountCommand(app.renamingDependencies())),
		profile.NewCommand(runtime, renaming.NewProfileCommand(app.renamingDependencies())),
		route.NewCommand(runtime), adapter.NewCommand(runtime),
		manifest.NewCommand(runtime), readiness.NewTestCommand(runtime),
		verification.NewCommand(runtime), catalog.NewModelsCommand(app.catalogDependencies()),
		catalog.NewCatalogCommand(app.catalogDependencies()), accountcli.NewBalanceCommand(runtime),
	}
	for group, commands := range map[string][]*cobra.Command{
		"connect": connect, "daily": daily, "recover": recover, "advanced": advanced,
	} {
		for _, command := range commands {
			command.GroupID = group
			root.AddCommand(command)
		}
	}
	completion := newCompletionCommand(root)
	completion.GroupID = "advanced"
	root.AddCommand(completion, credentialcli.NewCommand(runtime))
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return fmt.Errorf("%w", err)
	})
	return root
}

func appVersion(app *App) string {
	if version := strings.TrimSpace(app.Version); version != "" {
		return version
	}
	return Version
}

func renderCommandHelp(app *App, command *cobra.Command) {
	r := renderer(app)
	title := "Command help"
	if command.Parent() != nil {
		title = command.CommandPath()
	}
	r.ProductTitle(title)
	if command.Short != "" {
		r.Text(command.Short)
	}
	if command.Parent() == nil {
		r.Section("Start with one path")
		r.Command("aigw setup    # connect the first service")
		r.Command("aigw use      # choose the active service")
		r.Command("aigw check    # confirm readiness")
	}
	r.Section("Usage")
	usage := command.UseLine()
	if command.Parent() == nil && command.HasAvailableSubCommands() {
		usage = command.CommandPath() + " [command]"
	}
	r.Command(usage)
	groups := map[string][]*cobra.Command{}
	ungrouped := []*cobra.Command{}
	for _, child := range command.Commands() {
		if !child.IsAvailableCommand() || child.Hidden {
			continue
		}
		if child.GroupID == "" {
			ungrouped = append(ungrouped, child)
		} else {
			groups[child.GroupID] = append(groups[child.GroupID], child)
		}
	}
	for _, item := range []struct{ id, title string }{
		{"connect", "Connect"},
		{"daily", "Use every day"},
		{"recover", "Recover"},
		{"advanced", "Advanced"},
	} {
		if len(groups[item.id]) == 0 {
			continue
		}
		r.Section(item.title)
		for _, child := range groups[item.id] {
			r.Row(child.Name(), child.Short)
		}
	}
	if len(ungrouped) > 0 {
		sort.Slice(ungrouped, func(i, j int) bool { return ungrouped[i].Name() < ungrouped[j].Name() })
		r.Section("Commands")
		for _, child := range ungrouped {
			r.Row(child.Name(), child.Short)
		}
	}
	flags := command.NonInheritedFlags()
	if flags.HasAvailableFlags() {
		r.Section("Options")
		flags.VisitAll(func(flag *pflag.Flag) {
			if flag.Hidden {
				return
			}
			name := "--" + flag.Name
			if flag.Shorthand != "" {
				name = "-" + flag.Shorthand + ", " + name
			}
			usage := strings.TrimSpace(flag.Usage)
			if flag.Name == "help" {
				usage = "show help"
			}
			r.Row(name, usage)
		})
	}
}

func newCompletionCommand(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use: "completion <bash|zsh|fish|powershell>", Short: "generate shell completion", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return root.GenBashCompletion(root.OutOrStdout())
			case "zsh":
				return root.GenZshCompletion(root.OutOrStdout())
			case "fish":
				return root.GenFishCompletion(root.OutOrStdout(), true)
			case "powershell":
				return root.GenPowerShellCompletion(root.OutOrStdout())
			default:
				return fmt.Errorf("supported shells: bash, zsh, fish, powershell")
			}
		},
	}
}
