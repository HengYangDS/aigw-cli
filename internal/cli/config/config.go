// Package config implements `aigw config`, which imports, exports and
// migrates the secret-free configuration manifest.
package config

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/store"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// importSchemaVersion is the version of `aigw config import`'s JSON document.
const importSchemaVersion = 2

// NewCommand constructs the configuration manifest command tree.
func NewCommand(runtime invocation.Context) *cobra.Command {
	// Like the other command groups, bare `aigw config` shows its help.
	root := &cobra.Command{Use: "config", Short: "Import and export configuration"}
	root.AddCommand(newPathCommand(runtime), newExportCommand(runtime), newImportCommand(runtime), newMigrateCommand(runtime))
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
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the configuration path as JSON")
	return cmd
}

func newExportCommand(runtime invocation.Context) *cobra.Command {
	return &cobra.Command{Use: "export", Short: "Export a secret-free configuration manifest", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
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

// importResult is `aigw config import`'s JSON document: what the merge
// imported, kept and retired, and in a preview the clients whose projections
// it may change. A preview continues with the command that applies it.
type importResult struct {
	DryRun               bool     `json:"dry_run,omitempty"`
	ImportedAccounts     int      `json:"imported_accounts"`
	ImportedRoutes       int      `json:"imported_routes"`
	KeptAccounts         []string `json:"kept_accounts,omitempty"`
	RetiredRoutes        []string `json:"retired_routes,omitempty"`
	ProjectionCandidates []string `json:"projection_candidates,omitempty"`
	NextAction           string   `json:"next_action,omitempty"`
	NextDetail           string   `json:"next_detail,omitempty"`
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
		incoming, err := store.Parse(data, runtime.Config.Admission())
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
		cfg, err = domain.MergeWithOptions(cfg, incoming, domain.MergeOptions{
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
			ImportedAccounts:     len(domain.ManifestAccountNames(incoming)),
			ImportedRoutes:       len(incoming.Routes),
			KeptAccounts:         slices.Sorted(maps.Keys(keeping)),
			RetiredRoutes:        slices.Sorted(maps.Keys(retiring)),
			ProjectionCandidates: runtime.Clients.ChangedClients(before, cfg),
		}
		next := "aigw status"
		if dryRun {
			next = invocation.ApplyCommand(cmd, args)
		} else if err := runtime.Synchronizer().Commit(cmd.Context(), before, cfg, "configuration manifest"); err != nil {
			return err
		}
		result.NextAction, result.NextDetail = terminal.NextSteps(next)
		if jsonMode {
			return terminal.WriteDocument(runtime.Out, importSchemaVersion, true, result)
		}
		r := invocation.Renderer(runtime)
		if dryRun {
			r.ProductTitle("Configuration import preview")
		} else {
			r.ProductTitle("Configuration manifest imported")
		}
		rows := []terminal.Field{
			{Label: "Routes", Value: fmt.Sprintf("%d", result.ImportedRoutes)},
			{Label: "Accounts", Value: fmt.Sprintf("%d", result.ImportedAccounts)},
		}
		if len(keeping) > 0 {
			rows = append(rows, terminal.Field{Label: "Kept Accounts", Value: strings.Join(result.KeptAccounts, ", ")})
		}
		if len(retiring) > 0 {
			rows = append(rows, terminal.Field{Label: "Retired Routes", Value: strings.Join(result.RetiredRoutes, ", ")})
		}
		if dryRun {
			candidates := "none"
			if len(result.ProjectionCandidates) > 0 {
				candidates = strings.Join(result.ProjectionCandidates, ", ")
			}
			rows = append(rows, terminal.Field{Label: "Potential client projections", Value: candidates})
		}
		r.Rows(rows...)
		if dryRun {
			r.Success("No configuration, Token, or client projection was changed")
		}
		r.Next(next)
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
