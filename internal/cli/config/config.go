// Package config implements `aigw config`, which prints the configuration's
// path, exports it as a secret-free team file and rolls it back.
package config

import (
	"fmt"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/store"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// NewCommand constructs the configuration manifest command tree.
func NewCommand(runtime invocation.Context) *cobra.Command {
	// Like the other command groups, bare `aigw config` shows its help.
	root := &cobra.Command{Use: "config", Short: "Show, export and roll back the configuration"}
	root.AddCommand(newPathCommand(runtime), newExportCommand(runtime), newRollbackCommand(runtime))
	return root
}

// pathSchemaVersion is the version of `aigw config path`'s JSON document.
const pathSchemaVersion = 1

// pathDocument is `aigw config path`'s JSON document.
type pathDocument struct {
	Path string `json:"path"`
}

func newPathCommand(runtime invocation.Context) *cobra.Command {
	var jsonMode bool
	cmd := &cobra.Command{Use: "path", Short: "Print the local configuration path", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		if jsonMode {
			return terminal.WriteDocument(runtime.Out, pathSchemaVersion, true, pathDocument{Path: runtime.Config.Path()})
		}
		_, err := fmt.Fprintln(runtime.Out, runtime.Config.Path())
		return err
	}}
	cmd.Example = "aigw config path\naigw config path --json"
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the configuration path as JSON")
	return cmd
}

func newExportCommand(runtime invocation.Context) *cobra.Command {
	return &cobra.Command{Use: "export", Short: "Write the configuration as a secret-free team file, without clients", Args: cobra.NoArgs,
		Example: "aigw config export\naigw config export > team.toml", RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			data, err := store.Export(cfg)
			if err != nil {
				return err
			}
			_, err = runtime.Out.Write(data)
			return err
		}}
}
