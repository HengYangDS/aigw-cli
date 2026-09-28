// Package manifest owns secret-free configuration import and export commands.
package manifest

import (
	"fmt"
	"os"
	"strings"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"

	"github.com/spf13/cobra"
)

// NewCommand constructs the configuration manifest command tree.
func NewCommand(runtime invocation.Context) *cobra.Command {
	root := &cobra.Command{
		Use: "config", Short: "Import and export configuration",
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("Choose a config subcommand; run `aigw config --help`")
			}
			return fmt.Errorf("Unknown config subcommand %q; run `aigw config --help`", args[0])
		},
	}
	root.AddCommand(newPathCommand(runtime), newExportCommand(runtime), newImportCommand(runtime), newMigrateCommand(runtime))
	return root
}

func newPathCommand(runtime invocation.Context) *cobra.Command {
	return &cobra.Command{Use: "path", Short: "Print the local configuration path", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		_, err := fmt.Fprintln(runtime.Out, runtime.Config.Path())
		return err
	}}
}

func newExportCommand(runtime invocation.Context) *cobra.Command {
	return &cobra.Command{Use: "export", Short: "Export a secret-free configuration manifest", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		cfg, err := runtime.Config.Load()
		if err != nil {
			return err
		}
		data, err := configuration.Export(cfg)
		if err != nil {
			return err
		}
		_, err = runtime.Out.Write(data)
		return err
	}}
}

func newImportCommand(runtime invocation.Context) *cobra.Command {
	var replaceAccounts, replaceModels, replaceRoutes []string
	cmd := &cobra.Command{Use: "import <configuration.toml>", Short: "Merge a secret-free configuration manifest", Args: cobra.MatchAll(cobra.ExactArgs(1), func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(args[0]) == "" {
			return fmt.Errorf("Configuration manifest path must not be blank; run `%s --help`", cmd.CommandPath())
		}
		return nil
	}), RunE: func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("Failed to read configuration manifest: %w", err)
		}
		incoming, err := configuration.Parse(data)
		if err != nil {
			return err
		}
		cfg, err := runtime.Config.Load()
		if err != nil {
			return err
		}
		before := cfg.Clone()
		cfg, err = configuration.MergeWithOptions(cfg, incoming, configuration.MergeOptions{
			ReplaceAccounts: ReplacementSet(replaceAccounts),
			ReplaceModels:   ReplacementSet(replaceModels),
			ReplaceRoutes:   ReplacementSet(replaceRoutes),
		})
		if err != nil {
			return err
		}
		if err := invocation.Synchronizer(runtime).Commit(cmd.Context(), before, cfg, "configuration manifest"); err != nil {
			return err
		}
		accountNames := configuration.ManifestAccountNames(incoming)
		r := invocation.Renderer(runtime)
		r.ProductTitle("Configuration manifest imported")
		r.Row("Routes", fmt.Sprintf("%d", len(incoming.Routes)))
		r.Row("Accounts", fmt.Sprintf("%d", len(accountNames)))
		r.Next("aigw status")
		return nil
	}}
	cmd.Flags().StringSliceVar(&replaceAccounts, "replace-account", nil, "Explicitly replace conflicting account metadata; system tokens remain unchanged")
	cmd.Flags().StringSliceVar(&replaceModels, "replace-model", nil, "Explicitly replace conflicting canonical model metadata")
	cmd.Flags().StringSliceVar(&replaceRoutes, "replace-route", nil, "Explicitly replace conflicting model routes")
	return cmd
}

// ReplacementSet normalizes an explicit manifest replacement list.
func ReplacementSet(names []string) map[string]bool {
	result := make(map[string]bool, len(names))
	for _, name := range names {
		if name = strings.TrimSpace(name); name != "" {
			result[name] = true
		}
	}
	return result
}
