package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

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
				return runWizard(cmd.Context(), app)
			}
			return runStatus(cmd, app, false)
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
		&cobra.Group{ID: "start", Title: "Start here"},
		&cobra.Group{ID: "operate", Title: "Operate"},
		&cobra.Group{ID: "maintain", Title: "Maintain"},
		&cobra.Group{ID: "advanced", Title: "Advanced"},
	)
	start := []*cobra.Command{newSetupCommand(app), newUseCommand(app), newStatusCommand(app)}
	operate := []*cobra.Command{newCheckCommand(app), newTestCommand(app), newVerifyCommand(app), newRotateCommand(app), newModelsCommand(app), newCatalogCommand(app), newBalanceCommand(app)}
	maintain := []*cobra.Command{newDoctorCommand(app), newRepairCommand(app), newSyncCommand(app), newRollbackCommand(app), newUpdateCommand(app)}
	advanced := []*cobra.Command{newAddCommand(app), newAccountCommand(app), newProfileCommand(app), newRouteCommand(app), newAdapterCommand(app), newConfigCommand(app)}
	for group, commands := range map[string][]*cobra.Command{
		"start": start, "operate": operate, "maintain": maintain, "advanced": advanced,
	} {
		for _, command := range commands {
			command.GroupID = group
			root.AddCommand(command)
		}
	}
	completion := newCompletionCommand(root)
	completion.GroupID = "advanced"
	root.AddCommand(completion)
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return fmt.Errorf("%w", err)
	})
	hiddenClaude := &cobra.Command{
		Use:    "__run-claude",
		Hidden: true,
		Args:   cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			return RunClaude(app, args)
		},
	}
	hiddenClaude.DisableFlagParsing = true
	root.AddCommand(hiddenClaude)
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
	r.Title("AIGW", title)
	if command.Short != "" {
		r.Text(command.Short)
	}
	if command.Parent() == nil {
		r.Section("Common path")
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
		{"start", "Start here"},
		{"operate", "Operate"},
		{"maintain", "Maintain"},
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
