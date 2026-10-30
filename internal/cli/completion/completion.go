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
	return &cobra.Command{
		Use: "completion <bash|zsh|fish|powershell>", Short: "Generate shell completion", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
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
