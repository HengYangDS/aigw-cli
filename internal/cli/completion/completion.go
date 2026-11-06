// Package completion implements `aigw completion`, which writes a shell
// completion script for the command tree.
package completion

import (
	"fmt"

	"github.com/spf13/cobra"
)

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
		// The shell is an argument, so an unsupported one is refused with the
		// command's other argument errors, before anything is written.
		Args: cobra.MatchAll(cobra.ExactArgs(1), func(_ *cobra.Command, args []string) error {
			if _, supported := generators[args[0]]; !supported {
				return fmt.Errorf("supported shells: bash, zsh, fish, powershell")
			}
			return nil
		}),
		RunE: func(cmd *cobra.Command, args []string) error {
			return generators[args[0]](cmd.Root())
		},
	}
}
