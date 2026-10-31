// Package install implements `aigw install`, which copies the running portable
// program to its installation target.
package install

import (
	"errors"
	"strings"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/program"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// schemaVersion is the version of `aigw install`'s JSON document.
const schemaVersion = 1

// installDocument is `aigw install`'s JSON document. PATH is unchanged, so
// `aigw` may name another program and the next step stays prose.
type installDocument struct {
	Path       string `json:"path"`
	NextDetail string `json:"next_detail"`
}

// NewCommand constructs the portable executable installation command.
func NewCommand(runtime invocation.Context) *cobra.Command {
	target := runtime.InstallTarget
	var jsonMode bool
	command := &cobra.Command{
		Use:   "install",
		Short: "Install this portable AIGW executable",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if strings.TrimSpace(target) == "" {
				return errors.New("portable installation target is unavailable")
			}
			if err := program.Install(runtime.Executable, target); err != nil {
				return err
			}
			if jsonMode {
				return terminal.WriteDocument(runtime.Out, schemaVersion, true, installDocument{Path: target,
					NextDetail: "PATH is unchanged. Use the installed path directly, or add its directory to PATH before running `aigw setup`."})
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
	command.Flags().BoolVar(&jsonMode, "json", false, "Write the installed path as JSON")
	return command
}
