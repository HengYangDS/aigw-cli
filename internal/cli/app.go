// Package cli composes the AIGW command tree. It builds the App, runs one
// command under the configuration lock, presents help and errors, and groups
// the top-level commands, each implemented by its own subpackage.
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
	"sync"

	"aigw-cli/internal/app"
	accountcli "aigw-cli/internal/cli/account"
	"aigw-cli/internal/cli/add"
	"aigw-cli/internal/cli/balance"
	"aigw-cli/internal/cli/catalog"
	"aigw-cli/internal/cli/check"
	clientcli "aigw-cli/internal/cli/client"
	completioncli "aigw-cli/internal/cli/completion"
	"aigw-cli/internal/cli/config"
	credentialcli "aigw-cli/internal/cli/credential"
	"aigw-cli/internal/cli/doctor"
	installcli "aigw-cli/internal/cli/install"
	"aigw-cli/internal/cli/installation"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/cli/models"
	"aigw-cli/internal/cli/repair"
	"aigw-cli/internal/cli/rollback"
	"aigw-cli/internal/cli/rotate"
	"aigw-cli/internal/cli/route"
	"aigw-cli/internal/cli/setup"
	"aigw-cli/internal/cli/status"
	synccli "aigw-cli/internal/cli/sync"
	testcli "aigw-cli/internal/cli/test"
	"aigw-cli/internal/cli/uninstall"
	updatecli "aigw-cli/internal/cli/update"
	"aigw-cli/internal/cli/use"
	"aigw-cli/internal/cli/verify"
	"aigw-cli/internal/client"
	"aigw-cli/internal/client/discovery"
	"aigw-cli/internal/platform"
	"aigw-cli/internal/platform/process"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
	"aigw-cli/internal/terminal"
	"aigw-cli/internal/upgrade"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// App owns the dependencies and presentation state for one AIGW invocation.
type App struct {
	GOOS                  string
	DataDir               string
	Version               string
	Executable            string
	CredentialPath        string
	ResolveCredentialPath func() (string, error)
	ResolveProgram        func() (secret.Program, error)
	Files                 secret.Files
	InstallTarget         string
	Host                  discovery.Host
	Config                store.Store
	Secrets               secret.Store
	Accounts              secret.DiagnosticCredentialStore
	Env                   []string
	In                    io.Reader
	Out                   io.Writer
	Err                   io.Writer
	Interactive           bool
	Color                 bool
	Runner                process.VerificationRunner
	HTTP                  app.HTTPDoer
	Prompt                invocation.Prompter
	Discovery             discovery.Discoverer
	// Clients is the composition root's adapter registry.
	Clients client.Registry
	Updater app.Updater
	output  *commandOutput
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
func (a *App) Renderer() *terminal.Renderer {
	return terminal.NewRendererWithWidth(a.outputWriter(), a.Color, terminal.PresentationWidth(a.Out, environmentMap(a.Env)))
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
			return fmt.Errorf("%w; run `%s --help`", err, command.CommandPath())
		}
		if err := command.ValidateFlagGroups(); err != nil {
			return fmt.Errorf("%w; run `%s --help`", err, command.CommandPath())
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
	if err != nil && credentialInvocation(args) {
		renderer := terminal.NewRenderer(app.Err, false)
		terminal.RenderCredentialError(renderer, err)
		return terminal.Presented(errors.Join(err, renderer.Err()))
	}
	if err == nil || output.err != nil {
		return err
	}
	jsonMode, _ := command.Flags().GetBool("json")
	if jsonMode && output.started {
		return err
	}
	renderer := app.Renderer()
	terminal.RenderError(renderer, terminal.LocalFailure(Version, err), jsonMode)
	return terminal.Presented(errors.Join(err, renderer.Err()))
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
		return err == nil && len(cfg.Routes) == 0 && app.Interactive
	}
	path := strings.TrimPrefix(command.CommandPath(), command.Root().Name()+" ")
	switch path {
	case "setup", "add", "rotate", "rollback", "uninstall", "update",
		"account diagnostics enable", "account diagnostics disable", "account edit",
		"route add", "route edit", "route remove",
		"client enable", "client disable":
		return true
	case "use", "sync", "repair", "account rename", "route rename", "config import", "config migrate":
		dryRun, err := command.Flags().GetBool("dry-run")
		return err != nil || !dryRun
	default:
		return false
	}
}

// NewDefault constructs an App from the current platform, environment, configuration, and credential backend,
// operating clients only through the composition root's registry.
func NewDefault(clients client.Registry) (*App, error) {
	env := environmentMap(os.Environ())
	paths, err := platform.PathsFor(runtime.GOOS, env)
	if err != nil {
		return nil, err
	}
	host := discovery.Current()
	if err := clients.ValidateHost(host.Platform()); err != nil {
		return nil, err
	}
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve AIGW executable: %w", err)
	}
	secretStore, err := secret.Select(secret.Selection{
		Backend:    env["AIGW_SECRET_BACKEND"],
		GOOS:       runtime.GOOS,
		Root:       paths.Secrets,
		Executable: executable,
		Getenv:     os.Getenv,
	})
	if err != nil {
		return nil, err
	}
	diagnosticCredentialStore, err := secret.NewDiagnosticCredentialStore(secretStore)
	if err != nil {
		return nil, err
	}
	app := &App{
		GOOS:          runtime.GOOS,
		DataDir:       paths.Data,
		Version:       Version,
		Executable:    executable,
		InstallTarget: filepath.Join(paths.InstallDir, paths.InstallName),
		Host:          host.Platform(),
		Config:        store.NewStore(paths.Config, clients.Admission()),
		Secrets:       secretStore,
		Accounts:      diagnosticCredentialStore,
		Env:           os.Environ(),
		In:            os.Stdin,
		Out:           os.Stdout,
		Err:           os.Stderr,
		Interactive:   terminal.Interactive(os.Stdin),
		Color:         terminal.ColorEnabled(runtime.GOOS, env, terminal.Interactive(os.Stdout), terminal.EnableVirtualTerminal),
		Runner:        process.Runner{},
		HTTP:          &http.Client{},
		Prompt:        terminal.NewPrompter(os.Stdin, os.Stdout, env["NO_COLOR"] != ""),
		Discovery:     client.NewDiscoverer(clients, host),
		Clients:       clients,
		Updater:       upgrade.Current(executable),
	}
	app.UseExecutableIdentity(nil, paths.InstallName)
	return app, nil
}

// UseExecutableIdentity binds the App's credential reader to the bytes of its
// Executable. The program is read through files at most once for this App,
// and only when a command first needs the reader.
func (a *App) UseExecutableIdentity(files secret.Files, installName string) {
	executable, dataDir := a.Executable, a.DataDir
	program := sync.OnceValues(func() (secret.Program, error) {
		return secret.ReadProgram(files, executable)
	})
	a.Files, a.ResolveProgram = files, program
	a.ResolveCredentialPath = sync.OnceValues(func() (string, error) {
		current, err := program()
		if err != nil {
			return "", err
		}
		return current.EntrypointPath(dataDir, installName)
	})
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

func (a *App) invocationContext() invocation.Context {
	return invocation.Context{
		Version: appVersion(a), Executable: a.Executable, CredentialPath: a.CredentialPath, InstallTarget: a.InstallTarget,
		ResolveCredentialPath: a.ResolveCredentialPath,
		ResolveProgram:        a.ResolveProgram,
		Files:                 a.Files,
		Host:                  a.Host,
		Env:                   a.Env,
		Config:                a.Config, Secrets: a.Secrets, Accounts: a.Accounts, Out: a.outputWriter(),
		In:        a.In,
		RenderOut: a.outputWriter(),
		Color:     a.Color, Width: terminal.PresentationWidth(a.Out, environmentMap(a.Env)), Interactive: a.Interactive,
		Runner: a.Runner, HTTP: a.HTTP, Prompt: a.Prompt,
		Discovery: a.Discovery, Clients: a.Clients, Updater: a.Updater, Problem: terminal.ProblemError,
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
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			if len(cfg.Routes) == 0 && app.Interactive {
				return setup.RunWizard(cmd.Context(), app.invocationContext())
			}
			return status.RunStatus(app.invocationContext(), false)
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
	connect := []*cobra.Command{setup.NewCommand(runtime)}
	daily := []*cobra.Command{status.NewCommand(runtime), use.NewCommand(runtime), check.NewCommand(runtime), rotate.NewCommand(runtime)}
	recoveryCommands := []*cobra.Command{
		doctor.NewCommand(runtime), repair.NewCommand(runtime), synccli.NewCommand(runtime),
		rollback.NewCommand(runtime), updatecli.NewCommand(runtime),
		installcli.NewCommand(runtime), uninstall.NewCommand(runtime),
	}
	advanced := []*cobra.Command{
		installation.NewCommand(runtime),
		add.NewCommand(runtime), accountcli.NewCommand(runtime),
		route.NewCommand(runtime),
		clientcli.NewCommand(runtime),
		config.NewCommand(runtime), testcli.NewCommand(runtime),
		verify.NewCommand(runtime), models.NewCommand(runtime),
		catalog.NewCommand(runtime), balance.NewCommand(runtime),
	}
	for group, commands := range map[string][]*cobra.Command{
		"connect": connect, "daily": daily, "recover": recoveryCommands, "advanced": advanced,
	} {
		for _, command := range commands {
			command.GroupID = group
			root.AddCommand(command)
		}
	}
	completion := completioncli.NewCommand()
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

// startPath is the journey root help begins with. Each label is a command to
// copy, so it stays whole at every width.
func startPath(root string) []terminal.Field {
	return []terminal.Field{
		{Label: root + " setup", Value: "Connect the first account", Command: true},
		{Label: root + " use --for <client> <route>", Value: "Select one Route for one client", Command: true},
		{Label: root + " check", Value: "Check enabled clients (may use quota)", Command: true},
	}
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
		r.Rows(startPath(command.CommandPath())...)
	}
	r.Section("Usage")
	if command.Runnable() {
		r.Text(command.UseLine())
	}
	if command.HasAvailableSubCommands() {
		r.Text(command.CommandPath() + " [command]")
	}
	if command.Example != "" {
		r.Section("Examples")
		r.Command(command.Example)
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
		rows := make([]terminal.Field, 0, len(groups[group.ID]))
		for _, child := range groups[group.ID] {
			rows = append(rows, terminal.Field{Label: child.Name(), Value: child.Short, Command: true})
		}
		r.Rows(rows...)
	}
	// pflag owns the option layout and indents every line by the two-cell section indentation.
	optionWidth := terminal.PresentationWidth(app.Out, environmentMap(app.Env))
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
			r.Text(strings.TrimPrefix(strings.TrimRight(line, " \t"), "  "))
		}
	}
}
