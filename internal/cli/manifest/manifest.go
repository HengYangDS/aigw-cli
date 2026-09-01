// Package manifest owns secret-free configuration import and export commands.
package manifest

import (
	"fmt"
	"os"
	"strings"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/secrets"
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
	root.AddCommand(newPathCommand(runtime), newExportCommand(runtime), newImportCommand(runtime))
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
	var replaceAccounts, replaceProfiles []string
	cmd := &cobra.Command{Use: "import <configuration.toml>", Short: "Merge a secret-free configuration manifest", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
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
		cfg, err = configuration.MergeWithOptions(cfg, incoming, configuration.MergeOptions{ReplaceAccounts: ReplacementSet(replaceAccounts), ReplaceProfiles: ReplacementSet(replaceProfiles)})
		if err != nil {
			return err
		}
		if err := invocation.Synchronizer(runtime).Commit(cmd.Context(), before, cfg, "configuration manifest"); err != nil {
			return err
		}
		accountNames := configuration.ManifestAccountNames(incoming)
		missing := []string{}
		r := renderer(runtime)
		r.ProductTitle("Configuration manifest imported")
		r.Row("Profiles", fmt.Sprintf("%d", len(incoming.Profiles)))
		r.Row("Accounts", fmt.Sprintf("%d", len(accountNames)))
		for _, name := range accountNames {
			available, observationErr := runtime.Secrets.Exists(name)
			if observationErr != nil {
				r.Status(presentation.Warn, name, "Credential status unavailable · "+observationErr.Error())
				continue
			}
			if available {
				r.Status(presentation.OK, "System secret", name+" Token available")
				continue
			}
			missing = append(missing, name)
			instruction, _ := credential.TokenRecovery(runtime.Secrets, name)
			r.Status(presentation.Warn, name, "Token required · "+instruction)
		}
		if len(missing) > 0 && secrets.IsReadOnly(runtime.Secrets) {
			r.Next("Set the listed environment variables, then run `aigw check`")
		} else if len(missing) == 1 {
			r.Next("aigw rotate " + missing[0])
		} else if len(missing) > 1 {
			r.Next("aigw rotate <account>")
		} else {
			r.Next("aigw models")
		}
		return nil
	}}
	cmd.Flags().StringSliceVar(&replaceAccounts, "replace-account", nil, "Explicitly replace conflicting account metadata; system tokens remain unchanged")
	cmd.Flags().StringSliceVar(&replaceProfiles, "replace-profile", nil, "Explicitly replace conflicting model profiles")
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

func renderer(runtime invocation.Context) *presentation.Renderer {
	out := runtime.RenderOut
	if out == nil {
		out = runtime.Out
	}
	return presentation.NewWithWidth(out, runtime.Color, runtime.Width)
}
