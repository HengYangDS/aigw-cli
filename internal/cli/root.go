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
		Short:         "团队 AI API 配置、切换与诊断工具",
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
	root.Version = Version
	root.InitDefaultHelpFlag()
	if flag := root.Flags().Lookup("help"); flag != nil {
		flag.Usage = "查看帮助"
	}
	root.InitDefaultVersionFlag()
	if flag := root.Flags().Lookup("version"); flag != nil {
		flag.Usage = "显示版本"
	}
	root.SetHelpFunc(func(command *cobra.Command, _ []string) { renderCommandHelp(app, command) })
	root.SetHelpCommand(&cobra.Command{Use: "help [command]", Short: "查看命令帮助", Hidden: true})
	root.CompletionOptions.DisableDefaultCmd = true
	root.AddGroup(
		&cobra.Group{ID: "daily", Title: "日常使用"},
		&cobra.Group{ID: "advanced", Title: "高级管理"},
	)
	daily := []*cobra.Command{newUseCommand(app), newRotateCommand(app), newCheckCommand(app), newVerifyCommand(app), newRollbackCommand(app), newModelsCommand(app), newCatalogCommand(app), newBalanceCommand(app), newRepairCommand(app), newUpdateCommand(app)}
	advanced := []*cobra.Command{newSetupCommand(app), newAddCommand(app), newStatusCommand(app), newTestCommand(app), newDoctorCommand(app), newSyncCommand(app), newAccountCommand(app), newProfileCommand(app), newRouteCommand(app), newAdapterCommand(app), newConfigCommand(app)}
	for _, command := range daily {
		command.GroupID = "daily"
		root.AddCommand(command)
	}
	for _, command := range advanced {
		command.GroupID = "advanced"
		root.AddCommand(command)
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

func renderCommandHelp(app *App, command *cobra.Command) {
	r := renderer(app)
	title := "命令帮助"
	if command.Parent() != nil {
		title = command.CommandPath()
	}
	r.Title("AIGW", title)
	if command.Short != "" {
		r.Text(command.Short)
	}
	r.Section("用法")
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
	for _, item := range []struct{ id, title string }{{"daily", "日常使用"}, {"advanced", "高级管理"}} {
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
		r.Section("命令")
		for _, child := range ungrouped {
			r.Row(child.Name(), child.Short)
		}
	}
	flags := command.NonInheritedFlags()
	if flags.HasAvailableFlags() {
		r.Section("选项")
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
				usage = "查看帮助"
			}
			r.Row(name, usage)
		})
	}
}

func newCompletionCommand(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use: "completion <bash|zsh|fish|powershell>", Short: "生成 Shell 自动补全脚本", Args: cobra.ExactArgs(1),
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
