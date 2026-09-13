// Package update owns verified program update, candidate installation, and
// rollback command behavior.
package update

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/upgrade"

	"github.com/spf13/cobra"
)

// NewCommand constructs the signed portable release update command.
func NewCommand(runtime invocation.Context) *cobra.Command {
	var rollback bool
	var candidateArchive string
	var candidateChecksums string
	cmd := &cobra.Command{
		Use: "update", Short: "Install a verified release, a local candidate, or restore the previous portable program",
		Long: "Replace the program without changing client settings. Run sync after upgrading. Before rollback, disable enabled client integrations; the retained program must read an isolated copy of the current configuration before activation. If incompatible, explicitly restore a supported configuration first. Then restore the program and run its sync before resuming clients.",
		Args: cobra.MatchAll(cobra.NoArgs, func(cmd *cobra.Command, _ []string) error {
			for _, name := range []string{"candidate", "checksums"} {
				flag := cmd.Flags().Lookup(name)
				if flag.Changed && strings.TrimSpace(flag.Value.String()) == "" {
					return fmt.Errorf("--%s requires a non-empty path; run `aigw update --help`", name)
				}
			}
			return nil
		}),
		RunE: func(ctx *cobra.Command, _ []string) error {
			if runtime.Updater == nil {
				return fmt.Errorf("Automatic update is unavailable; install a verified release from GitLab or GitHub")
			}
			var (
				result string
				err    error
			)
			if rollback {
				config, readErr := os.ReadFile(runtime.Config.Path())
				if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
					return fmt.Errorf("read configuration before program rollback: %w", readErr)
				}
				result, err = runtime.Updater.Rollback(ctx.Context(), config)
			} else if candidateArchive != "" {
				result, err = runtime.Updater.UpdateCandidate(ctx.Context(), runtime.Version, upgrade.CandidateArchive{
					ArchivePath:   candidateArchive,
					ChecksumsPath: candidateChecksums,
				})
			} else {
				result, err = runtime.Updater.Update(ctx.Context(), runtime.Version)
			}
			if err != nil {
				if errors.Is(err, upgrade.ErrRollbackConfiguration) {
					return invocation.Problem(runtime,
						"Program rollback is incompatible with the current configuration",
						"The retained program could not read an isolated copy of the current configuration.",
						"The active program, retained program and configuration remain unchanged.",
						"Restore a configuration supported by the retained program, then retry `aigw update --rollback`.", err)
				}
				if rollback {
					return invocation.Problem(
						runtime,
						"Program rollback did not complete",
						"AIGW could not activate the retained previous program.",
						"No previous program version was confirmed active.",
						"aigw check",
						err,
					)
				}
				return err
			}
			r := invocation.Renderer(runtime)
			title := "Update"
			if rollback {
				title = "Program rollback"
			} else if candidateArchive != "" {
				title = "Verified local candidate"
			}
			r.ProductTitle(title)
			r.Success(result)
			r.Text("Run the active version's sync command to reconcile owned client settings, then check readiness.")
			r.Next("aigw sync")
			return nil
		},
	}
	cmd.Flags().BoolVar(&rollback, "rollback", false, "Roll back the portable AIGW program to the previous version offline")
	cmd.Flags().StringVar(&candidateArchive, "candidate", "", "Install one local portable archive without network access")
	cmd.Flags().StringVar(&candidateChecksums, "checksums", "", "Checksum manifest for --candidate")
	cmd.MarkFlagsRequiredTogether("candidate", "checksums")
	cmd.MarkFlagsMutuallyExclusive("rollback", "candidate")
	return cmd
}
