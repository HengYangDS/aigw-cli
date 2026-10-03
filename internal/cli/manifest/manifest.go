// Package manifest owns secret-free configuration import and export commands.
package manifest

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/client"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/presentation"

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

type importResult struct {
	DryRun               bool     `json:"dry_run"`
	ImportedAccounts     int      `json:"imported_accounts"`
	ImportedRoutes       int      `json:"imported_routes"`
	KeptAccounts         []string `json:"kept_accounts,omitempty"`
	RetiredRoutes        []string `json:"retired_routes,omitempty"`
	ProjectionCandidates []string `json:"projection_candidates,omitempty"`
	NextAction           string   `json:"next_action"`
}

func newImportCommand(runtime invocation.Context) *cobra.Command {
	var keepAccounts, replaceAccounts, replaceModels, replaceRoutes, retireRoutes []string
	var dryRun, jsonMode bool
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
		keeping := selectorSet(keepAccounts)
		retiring := selectorSet(retireRoutes)
		cfg, err = configuration.MergeWithOptions(cfg, incoming, configuration.MergeOptions{
			KeepAccounts:    keeping,
			ReplaceAccounts: selectorSet(replaceAccounts),
			ReplaceModels:   selectorSet(replaceModels),
			ReplaceRoutes:   selectorSet(replaceRoutes),
			RetireRoutes:    retiring,
		})
		if err != nil {
			return err
		}
		result := importResult{
			DryRun:               dryRun,
			ImportedAccounts:     len(configuration.ManifestAccountNames(incoming)),
			ImportedRoutes:       len(incoming.Routes),
			KeptAccounts:         slices.Sorted(maps.Keys(keeping)),
			RetiredRoutes:        slices.Sorted(maps.Keys(retiring)),
			ProjectionCandidates: client.DefaultRegistry().ChangedClients(before, cfg),
			NextAction:           "aigw status",
		}
		if dryRun {
			result.NextAction = "Review the preview, then rerun without --dry-run"
		} else if err := invocation.Synchronizer(runtime).Commit(cmd.Context(), before, cfg, "configuration manifest"); err != nil {
			return err
		}
		if jsonMode {
			return presentation.WriteJSON(runtime.Out, result)
		}
		r := invocation.Renderer(runtime)
		if dryRun {
			r.ProductTitle("Configuration import preview")
		} else {
			r.ProductTitle("Configuration manifest imported")
		}
		r.Row("Routes", fmt.Sprintf("%d", result.ImportedRoutes))
		r.Row("Accounts", fmt.Sprintf("%d", result.ImportedAccounts))
		if len(keeping) > 0 {
			r.Row("Kept Accounts", strings.Join(result.KeptAccounts, ", "))
		}
		if len(retiring) > 0 {
			r.Row("Retired Routes", strings.Join(result.RetiredRoutes, ", "))
		}
		if dryRun {
			candidates := "none"
			if len(result.ProjectionCandidates) > 0 {
				candidates = strings.Join(result.ProjectionCandidates, ", ")
			}
			r.Row("Potential client projections", candidates)
			r.Detail("No configuration, Token, or client projection was changed")
		}
		r.Next(result.NextAction)
		return nil
	}}
	cmd.Flags().StringSliceVar(&keepAccounts, "keep-account", nil, "Retain local public Account metadata instead of the imported version")
	cmd.Flags().StringSliceVar(&replaceAccounts, "replace-account", nil, "Explicitly replace conflicting account metadata; system tokens remain unchanged")
	cmd.Flags().StringSliceVar(&replaceModels, "replace-model", nil, "Explicitly replace conflicting canonical model metadata")
	cmd.Flags().StringSliceVar(&replaceRoutes, "replace-route", nil, "Explicitly replace conflicting model routes")
	cmd.Flags().StringSliceVar(&retireRoutes, "retire-route", nil, "Retire an existing unselected Route absent from the imported manifest")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview the merge and possible client projection changes without writing")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the import preview or result as JSON")
	return cmd
}

// selectorSet normalizes explicit replacement or retirement IDs.
func selectorSet(names []string) map[string]bool {
	result := make(map[string]bool, len(names))
	for _, name := range names {
		if name = strings.TrimSpace(name); name != "" {
			result[name] = true
		}
	}
	return result
}
