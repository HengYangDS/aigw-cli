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
		Use:   "update",
		Short: "Update or roll back the portable AIGW program",
		Long: "Replace only a portable AIGW program; Homebrew-managed copies must be\n" +
			"upgraded with Homebrew. Client settings and credentials do not change.\n" +
			"Run aigw sync, then aigw check before resuming clients. --rollback\n" +
			"restores the retained program, not configuration. An incompatible\n" +
			"current configuration blocks program rollback without changing files.",
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
			if err := upgrade.RequirePortableOwnership(runtime.Executable); err != nil {
				return err
			}
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
			if err != nil && result != "" {
				title := "Program updated; cleanup is incomplete"
				if rollback {
					title = "Program restored; cleanup is incomplete"
				}
				return invocation.Problem(runtime, title,
					"Program replacement completed, but an owned temporary resource could not be removed.",
					"The replacement is active; client settings and credentials were not changed.",
					"Run `aigw installation` and `aigw doctor` before retrying cleanup or another program transition.", err)
			}
			if !rollback && errors.Is(err, upgrade.ErrCandidateIdentity) {
				return invocation.Problem(runtime,
					"Candidate has different program bytes at the current version",
					"The verified local archive does not match the active program identity.",
					"The active and retained programs are unchanged.",
					"Use the exact current release artifact or a verified newer version.", err)
			}
			if !rollback && errors.Is(err, upgrade.ErrProgramStartupVerification) {
				return invocation.Problem(runtime,
					"Candidate program failed startup verification",
					"The candidate did not run its version check.",
					"The installed program is unchanged.",
					"Verify the candidate archive for this platform and retry only with a valid artifact.", err)
			}
			if errors.Is(err, upgrade.ErrRollbackConfiguration) {
				return invocation.Problem(runtime,
					"Program rollback is incompatible with the current configuration",
					"The retained program could not read an isolated copy of the current configuration.",
					"The active program, retained program and configuration remain unchanged.",
					"Restore a configuration supported by the retained program, then retry `aigw update --rollback`.", err)
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
				return invocation.Problem(runtime,
					"Program update did not complete",
					"AIGW could not complete release verification and program replacement.",
					"No updated program version was confirmed active; client settings and credentials were not changed.",
					"Run `aigw doctor` to inspect current state; verify the selected release source or use a verified local archive.", err)
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
