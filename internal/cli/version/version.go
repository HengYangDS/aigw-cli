// Package version implements `aigw version`, which prints the program's
// version as `aigw --version` does.
package version

import (
	"fmt"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// schemaVersion is the version document's schema; the document has no
// predecessor.
const schemaVersion = 1

// document is `aigw version --json`'s document.
type document struct {
	SchemaVersion int    `json:"schema_version"`
	OK            bool   `json:"ok"`
	Version       string `json:"version"`
}

// NewCommand constructs `aigw version`, which prints the line that `aigw
// --version` prints, or the version as JSON.
func NewCommand(runtime invocation.Context) *cobra.Command {
	var jsonMode bool
	command := &cobra.Command{
		Use: "version", Short: "Show the program's version",
		Example: "aigw version\naigw version --json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if jsonMode {
				return terminal.WriteJSON(runtime.Out, document{SchemaVersion: schemaVersion, OK: true, Version: runtime.Version})
			}
			_, err := fmt.Fprintf(runtime.Out, "%s version %s\n", cmd.Root().Name(), runtime.Version)
			return err
		},
	}
	command.Flags().BoolVar(&jsonMode, "json", false, "Write the version as JSON")
	return command
}
