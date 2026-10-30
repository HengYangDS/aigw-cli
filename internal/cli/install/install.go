// Package install implements `aigw install`, which copies the running portable
// program to its installation target.
package install

import (
	"errors"

	"strings"

	"aigw-cli/internal/cli/invocation"

	"aigw-cli/internal/upgrade"

	"github.com/spf13/cobra"
)

// NewCommand constructs the portable executable installation command.
func NewCommand(runtime invocation.Context) *cobra.Command {
	target := runtime.InstallTarget
	command := &cobra.Command{
		Use:   "install",
		Short: "Install this portable AIGW executable",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if strings.TrimSpace(target) == "" {
				return errors.New("portable installation target is unavailable")
			}
			if err := upgrade.Install(runtime.Executable, target); err != nil {
				return err
			}
			render := invocation.Renderer(runtime)
			render.ProductTitle("Portable installation")
			render.Success("Installed " + target)
			render.Text("PATH is unchanged. Use the installed path directly, or add its directory to PATH before running the command below.")
			render.Next("aigw setup")
			return nil
		},
	}
	command.Flags().StringVar(&target, "target", target, "destination executable path")
	return command
}
