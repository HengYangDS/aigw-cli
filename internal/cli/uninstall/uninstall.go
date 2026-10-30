// Package uninstall implements `aigw uninstall`, which withdraws AIGW's client
// projections and removes the portable program.
package uninstall

import (
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/program"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// NewCommand constructs the command that removes only the installed portable executable and its owned backup.
func NewCommand(runtime invocation.Context) *cobra.Command {
	var target string
	command := &cobra.Command{
		Use:   "uninstall",
		Short: "Withdraw managed projections and remove a portable program",
		Long: "Withdraw AIGW-owned client projections, then remove the portable\n" +
			"executable and its rollback copy. This retains Account and Route data\n" +
			"and stored Tokens. Homebrew-managed copies must be removed with\n" +
			"Homebrew; this command does not manage them.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(target) == "" {
				target = runtime.Executable
			}
			if err := program.RequirePortableOwnership(target); err != nil {
				return err
			}
			_, targetErr := os.Stat(target)
			if targetErr != nil && !errors.Is(targetErr, os.ErrNotExist) {
				return fmt.Errorf("inspect installed AIGW executable: %w", targetErr)
			}
			_, statErr := os.Stat(runtime.Config.Path())
			if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
				return fmt.Errorf("inspect AIGW configuration: %w", statErr)
			}
			if statErr == nil {
				before, err := runtime.Config.Load()
				if err != nil {
					return err
				}
				after := before.Clone()
				synchronizer := runtime.Synchronizer()
				if err := synchronizer.Withdraw(&after); err != nil {
					return err
				}
				if err := synchronizer.CommitProjection(cmd.Context(), before, after, "uninstall"); err != nil {
					return err
				}
			}
			if err := program.Uninstall(target); err != nil {
				title := "Portable program removal is incomplete"
				impact := "No AIGW configuration existed; no managed client withdrawal was needed. Tokens and versioned credential readers are retained."
				if statErr == nil {
					title = "Client withdrawal completed; program removal is incomplete"
					impact = "Managed client withdrawal is committed; configuration, Tokens and versioned credential readers are retained."
				}
				return invocation.Problem(runtime,
					title,
					"AIGW could not remove every selected portable program file.",
					impact,
					"Inspect the selected target and retained files; after resolving file access, retry `uninstall --target <path>` from another verified AIGW executable.", err)
			}
			render := invocation.Renderer(runtime)
			render.ProductTitle("Portable uninstall")
			render.Success("Removed AIGW client projections, executable, and its single rollback copy")
			render.Text("Configuration and credential-store secrets were preserved.")
			render.Text("Versioned credential readers were retained for cached or other-installation callers.")
			return nil
		},
	}
	command.Flags().StringVar(&target, "target", "", "installed executable path; defaults to the running executable")
	return command
}
