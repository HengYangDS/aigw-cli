// Package completion implements `aigw completion`, which writes a shell
// completion script for the command tree.
package completion

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

// shells are the shells whose completion script the command writes, which
// its argument completes.
var shells = []cobra.Completion{"bash", "zsh", "fish", "powershell"}

// NewCommand constructs the completion command. It generates the script from
// the root of the command tree that it joins.
func NewCommand() *cobra.Command {
	generators := map[string]func(root *cobra.Command) error{
		"bash":       func(root *cobra.Command) error { return root.GenBashCompletion(root.OutOrStdout()) },
		"zsh":        func(root *cobra.Command) error { return root.GenZshCompletion(root.OutOrStdout()) },
		"fish":       func(root *cobra.Command) error { return root.GenFishCompletion(root.OutOrStdout(), true) },
		"powershell": func(root *cobra.Command) error { return root.GenPowerShellCompletion(root.OutOrStdout()) },
	}
	return &cobra.Command{
		Use: "completion <bash|zsh|fish|powershell>", Short: "Generate shell completion",
		Example: "aigw completion zsh > \"${fpath[1]}/_aigw\"\n" +
			"aigw completion bash > ~/.local/share/bash-completion/completions/aigw\n" +
			"aigw completion fish > ~/.config/fish/completions/aigw.fish",
		// The shell is an argument, so an unsupported one is refused with the
		// command's other argument errors, before anything is written.
		Args: cobra.MatchAll(cobra.ExactArgs(1), func(_ *cobra.Command, args []string) error {
			if _, supported := generators[args[0]]; !supported {
				return fmt.Errorf("supported shells: %s", strings.Join(shells, ", "))
			}
			return nil
		}),
		ValidArgsFunction: func(_ *cobra.Command, args []string, _ string) ([]cobra.Completion, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return slices.Clone(shells), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return generators[args[0]](cmd.Root())
		},
	}
}
