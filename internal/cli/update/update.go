// Package update owns verified program update, candidate installation, and
// rollback command behavior.
package update

import (
	"fmt"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/upgrade"
	"github.com/spf13/cobra"
)

func NewCommand(runtime invocation.Context) *cobra.Command {
	var rollback bool
	var candidateArchive string
	var candidateChecksums string
	cmd := &cobra.Command{
		Use: "update", Short: "Install a verified release, a local candidate, or restore the previous portable program", Args: cobra.NoArgs,
		RunE: func(ctx *cobra.Command, _ []string) error {
			if runtime.Updater == nil {
				return fmt.Errorf("Automatic update is unavailable; install a verified release from GitLab or GitHub")
			}
			var (
				result string
				err    error
			)
			if rollback {
				result, err = runtime.Updater.Rollback(ctx.Context())
			} else if candidateArchive != "" {
				result, err = runtime.Updater.UpdateCandidate(ctx.Context(), runtime.Version, upgrade.CandidateArchive{
					ArchivePath:   candidateArchive,
					ChecksumsPath: candidateChecksums,
				})
			} else {
				result, err = runtime.Updater.Update(ctx.Context(), runtime.Version)
			}
			if err != nil {
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
			r.Next("aigw check")
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
