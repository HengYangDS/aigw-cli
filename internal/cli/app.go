// Package cli composes the AIGW command tree and owns root-level workflows.
// Cohesive command groups live in semantic subpackages below this package.
package cli

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/spf13/pflag"

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
	"aigw-cli/internal/cli/renaming"
	"aigw-cli/internal/cli/route"
	updatecli "aigw-cli/internal/cli/update"
	"aigw-cli/internal/cli/verification"
	"aigw-cli/internal/client"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/console"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/platform"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/process"
	"aigw-cli/internal/prompt"
	domainreadiness "aigw-cli/internal/readiness"
	"aigw-cli/internal/secrets"
	"aigw-cli/internal/upgrade"

	"github.com/spf13/cobra"
)

// App owns the dependencies and presentation state for one AIGW invocation.
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
	Accounts           secrets.DiagnosticCredentialStore
	Env                []string
	In                 io.Reader
	Out                io.Writer
	Err                io.Writer
	Interactive        bool
	Color              bool
	Runner             process.CaptureRunner
	HTTP               invocation.HTTPDoer
	Prompt             invocation.Prompter
	Discovery          discovery.Discoverer
	Updater            invocation.Updater
	output             *commandOutput
}

// commandOutput owns write progress and the first failure for one invocation.
type commandOutput struct {
	writer  io.Writer
	err     error
	started bool
}

func (w *commandOutput) Write(data []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	count, writeErr := w.writer.Write(data)
	w.started = w.started || count > 0
	if writeErr == nil && count != len(data) {
		writeErr = io.ErrShortWrite
	}
	w.err = writeErr
	return count, writeErr
}

func (a *App) outputWriter() io.Writer {
	if a.output != nil {
		return a.output
	}
	return a.Out
}

// Renderer uses invocation output while measuring the original terminal writer.
func (a *App) Renderer() *presentation.Renderer {
	return presentation.NewWithWidth(a.outputWriter(), a.Color, console.PresentationWidth(a.Out, environmentMap(a.Env)))
}

// Execute runs one argument vector, serializing mutations and returning any command or output failure.
func Execute(app *App, args []string) error {
	output := &commandOutput{writer: app.Out}
	app.output = output
	defer func() { app.output = nil }()
	var unlock func() error
	root := NewRoot(app)
	root.PersistentPreRunE = func(command *cobra.Command, _ []string) error {
		// Cobra normally validates flag relationships after this mutation boundary.
		if err := command.ValidateRequiredFlags(); err != nil {
			return err
		}
		if err := command.ValidateFlagGroups(); err != nil {
			return err
		}
		if !requiresConfigurationLock(app, command) {
			return nil
		}
		locked, err := app.Config.Lock(command.Context())
		if err != nil {
			return fmt.Errorf("%w; retry after the other command finishes", err)
		}
		unlock = locked
		return nil
	}
	root.SetArgs(args)
	command, err := root.ExecuteC()
	if output.err != nil && !errors.Is(err, output.err) {
		err = errors.Join(err, output.err)
	}
	err = finishExecution(err, unlock)
	if err == nil || credentialInvocation(args) || output.err != nil {
		return err
	}
	jsonMode, _ := command.Flags().GetBool("json")
	if jsonMode && output.started {
		return err
	}
	renderer := app.Renderer()
	presentation.RenderError(renderer, err, jsonMode)
	return presentation.Presented(errors.Join(err, renderer.Err()))
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
	return fmt.Errorf("%w; release config lock: %w", commandErr, unlockErr)
}

func credentialInvocation(args []string) bool {
	return len(args) > 0 && args[0] == "credential"
}

func requiresConfigurationLock(app *App, command *cobra.Command) bool {
	if command == command.Root() {
		cfg, err := app.Config.Load()
		return err == nil && len(cfg.Profiles) == 0 && app.Interactive
	}
	path := strings.TrimPrefix(command.CommandPath(), command.Root().Name()+" ")
	switch path {
	case "setup", "add", "use", "rotate", "rollback", "uninstall", "update",
		"account connect", "account disconnect", "account edit",
		"profile add", "profile edit", "profile remove",
		"adapter enable", "adapter disable", "config import":
		return true
	case "sync", "repair", "account rename", "profile rename":
		dryRun, err := command.Flags().GetBool("dry-run")
		return err != nil || !dryRun
	default:
		return false
	}
}

// NewDefault constructs an App from the current platform, environment, configuration, and credential backend.
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
	diagnosticCredentialStore, err := secrets.NewDiagnosticCredentialStore(secretStore)
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
		Accounts:           diagnosticCredentialStore,
		Env:                os.Environ(),
		In:                 os.Stdin,
		Out:                os.Stdout,
		Err:                os.Stderr,
		Interactive:        console.Interactive(os.Stdin),
		Color:              console.ColorEnabled(runtime.GOOS, env, console.Interactive(os.Stdout), console.EnableVirtualTerminal),
		Runner:             process.Runner{},
		HTTP:               &http.Client{},
		Prompt:             prompt.New(os.Stdin, os.Stdout, env["NO_COLOR"] != ""),
		Discovery:          client.NewDiscoverer(client.DefaultRegistry(), discovery.Current()),
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
		Config: a.Config, Secrets: a.Secrets, Clients: invocation.Synchronizer(a.invocationContext()), Env: a.Env, Out: a.outputWriter(),
		Inspect: func(cfg configuration.Config) map[string]domainreadiness.Client {
			return readiness.InspectClients(a.invocationContext(), cfg)
		},
		RenderOut: a.outputWriter(),
		Color:     a.Color, Width: console.PresentationWidth(a.Out, environmentMap(a.Env)),
	})
}

func (a *App) catalogDependencies() catalog.Dependencies {
	return catalog.Dependencies{
		Config: a.Config, Secrets: a.Secrets, HTTP: a.HTTP, Out: a.outputWriter(),
		RenderOut: a.outputWriter(),
		Color:     a.Color, Width: console.PresentationWidth(a.Out, environmentMap(a.Env)),
	}
}

func (a *App) invocationContext() invocation.Context {
	return invocation.Context{
		Version: appVersion(a), Executable: a.Executable, InstallTarget: a.InstallTarget,
		ClaudeSettingsPath: a.ClaudeSettingsPath,
		Config:             a.Config, Secrets: a.Secrets, Accounts: a.Accounts, Out: a.outputWriter(),
		In:        a.In,
		RenderOut: a.outputWriter(),
		Color:     a.Color, Width: console.PresentationWidth(a.Out, environmentMap(a.Env)), Interactive: a.Interactive,
		Runner: a.Runner, HTTP: a.HTTP, Prompt: a.Prompt,
		Discovery: a.Discovery, Updater: a.Updater, Now: a.Now, Problem: presentation.ProblemError,
	}
}

// Version is the product version injected by the release build.
var Version = "0.1.0-dev"

// NewRoot constructs the complete Cobra command tree from its authoritative command metadata.
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
	root.SetOut(app.outputWriter())
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
	recoveryCommands := []*cobra.Command{
		app.doctorCommand(), recovery.NewRepairCommand(runtime), recovery.NewSyncCommand(runtime),
		recovery.NewRollbackCommand(runtime), updatecli.NewCommand(runtime),
		installcli.NewInstallCommand(runtime), installcli.NewUninstallCommand(runtime),
	}
	advanced := []*cobra.Command{
		accountcli.NewAddCommand(runtime), accountcli.NewCommand(runtime, renaming.NewAccountCommand(runtime)),
		profile.NewCommand(runtime, renaming.NewProfileCommand(runtime)),
		route.NewCommand(runtime), adapter.NewCommand(runtime),
		manifest.NewCommand(runtime), readiness.NewTestCommand(runtime),
		verification.NewCommand(runtime), catalog.NewModelsCommand(app.catalogDependencies()),
		catalog.NewCatalogCommand(app.catalogDependencies()), accountcli.NewBalanceCommand(runtime),
	}
	for group, commands := range map[string][]*cobra.Command{
		"connect": connect, "daily": daily, "recover": recoveryCommands, "advanced": advanced,
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
	r := app.Renderer()
	title := "Command help"
	if command.Parent() != nil {
		title = command.CommandPath()
	}
	r.ProductTitle(title)
	if command.Long != "" {
		for line := range strings.SplitSeq(strings.TrimSpace(command.Long), "\n") {
			r.Text(line)
		}
	} else if command.Short != "" {
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
	if command.Example != "" {
		r.Section("Examples")
		for line := range strings.SplitSeq(strings.TrimSpace(command.Example), "\n") {
			r.Command(line)
		}
	}
	groups := map[string][]*cobra.Command{}
	for _, child := range command.Commands() {
		if !child.IsAvailableCommand() {
			continue
		}
		groups[child.GroupID] = append(groups[child.GroupID], child)
	}
	for _, group := range append(slices.Clone(command.Groups()), &cobra.Group{Title: "Commands"}) {
		if len(groups[group.ID]) == 0 {
			continue
		}
		r.Section(group.Title)
		for _, child := range groups[group.ID] {
			r.Row(child.Name(), child.Short)
		}
	}
	optionWidth := max(console.PresentationWidth(app.Out, environmentMap(app.Env))-2, 0)
	options := presentation.New(app.outputWriter(), app.Color)
	for _, group := range []struct {
		title string
		flags *pflag.FlagSet
	}{
		{title: "Options", flags: command.NonInheritedFlags()},
		{title: "Inherited options", flags: command.InheritedFlags()},
	} {
		if !group.flags.HasAvailableFlags() {
			continue
		}
		r.Section(group.title)
		for line := range strings.SplitSeq(strings.TrimRight(group.flags.FlagUsagesWrapped(optionWidth), "\n"), "\n") {
			options.Text(strings.TrimRight(line, " \t"))
		}
	}
}

func newCompletionCommand(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use: "completion <bash|zsh|fish|powershell>", Short: "Generate shell completion", Args: cobra.ExactArgs(1),
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
