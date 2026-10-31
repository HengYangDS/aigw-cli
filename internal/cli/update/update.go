// Package update owns verified program update, candidate installation, and
// rollback command behavior.
package update

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/program"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// schemaVersion is the version of `aigw update`'s JSON document.
const schemaVersion = 1

// updateDocument is `aigw update`'s JSON document. Its state is the program
// transition's, or prepared when credentials are ready for an earlier
// Homebrew release; its version is the active program's afterward, and its
// summary says what happened. A transition that completed while a later step
// failed names that step's problem, and the document's ok is false.
type updateDocument struct {
	DryRun     bool   `json:"dry_run,omitempty"`
	State      string `json:"state"`
	Version    string `json:"version,omitempty"`
	Summary    string `json:"summary,omitempty"`
	Problem    string `json:"problem,omitempty"`
	NextAction string `json:"next_action,omitempty"`
	NextDetail string `json:"next_detail,omitempty"`
	// CurrentVersion, Path and Rollback are a preview's installed version,
	// the program file and the rollback copy that an update would write.
	CurrentVersion string `json:"current_version,omitempty"`
	Path           string `json:"path,omitempty"`
	Rollback       string `json:"rollback,omitempty"`
}

// prepareHomebrewRollback returns macOS Keychain items to the reader of an
// earlier release before Homebrew installs it; Homebrew keeps its program files.
// If the operator keeps this release, its next Token use takes the items back.
// Without a readable configuration it cannot name those items, so it stops
// before any native request.
func prepareHomebrewRollback(runtime invocation.Context, jsonMode bool) error {
	cfg, err := runtime.Config.Load()
	if err != nil {
		return invocation.Problem(runtime,
			"Configuration is not ready for a Homebrew rollback",
			"AIGW could not read the configuration, including each connection's retry policy.",
			"No credential, policy or program changed.",
			"Run `aigw doctor` to inspect or restore the configuration, then retry `aigw update --rollback`.", err)
	}
	for _, binding := range cfg.Clients {
		if binding.RetryPolicy != nil {
			return invocation.Problem(runtime,
				"Homebrew rollback retry-policy capability is unproved",
				"No retained Homebrew program was supplied to the native configuration probe.",
				"No credential, policy or program changed.",
				"Keep this compatible AIGW release, or use `aigw rollback` to restore the accepted configuration before choosing a predecessor.",
				fmt.Errorf("Homebrew rollback cannot prove the selected retry policy"))
		}
	}
	custody, err := credentialCustody(runtime.Dependencies)
	if err != nil {
		return invocation.Problem(runtime,
			"Credentials are not ready for a Homebrew rollback",
			"AIGW could not read the configuration that names its credentials.",
			"No credential or program changed.",
			"Run `aigw doctor` to inspect or restore the configuration, then retry `aigw update --rollback` before Homebrew installs the earlier release.",
			err)
	}
	if err := custody.Release(""); err != nil {
		return invocation.Problem(runtime,
			"Credentials are not ready for a Homebrew rollback",
			"AIGW could not return every Keychain item to the reader of earlier releases.",
			"The installed program is unchanged and can still read every credential.",
			"Run `aigw check`, then retry `aigw update --rollback` before Homebrew installs the earlier release.",
			errors.Join(err, custody.Reclaim()))
	}
	if jsonMode {
		return terminal.WriteDocument(runtime.Out, schemaVersion, true, updateDocument{State: "prepared",
			Summary:    "Credentials are ready for an earlier AIGW release.",
			NextDetail: "Install the earlier release with Homebrew, then run `aigw sync` and `aigw check`."})
	}
	r := invocation.Renderer(runtime)
	r.ProductTitle("Homebrew rollback")
	r.Success("Credentials are ready for an earlier AIGW release.")
	r.Text("Install the earlier release with Homebrew, then run its sync and check commands. If you keep this release, its next Token use takes the credentials back.")
	r.Next("aigw sync")
	return nil
}

// NewCommand constructs the signed portable release update command.
func NewCommand(runtime invocation.Context) *cobra.Command {
	var rollback, jsonMode, dryRun bool
	var candidateArchive string
	var candidateChecksums string
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update or roll back the portable AIGW program",
		Long: "Replace only a portable AIGW program; Homebrew-managed copies must be\n" +
			"upgraded with Homebrew. Client settings and Tokens do not change.\n" +
			"Run aigw sync, then aigw check before resuming clients. --rollback\n" +
			"restores the retained program, not configuration. An incompatible\n" +
			"current configuration blocks program rollback without changing files.\n" +
			"Before Homebrew installs an earlier release, --rollback returns macOS\n" +
			"Keychain items to that release's reader.",
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
			if err := program.RequirePortableOwnership(runtime.Executable); err != nil {
				if rollback && errors.Is(err, program.ErrHomebrewManaged) {
					return prepareHomebrewRollback(runtime, jsonMode)
				}
				return err
			}
			if runtime.Updater == nil {
				return fmt.Errorf("Automatic update is unavailable; install a verified release from GitLab or GitHub")
			}
			if dryRun {
				return runPreview(ctx, runtime, jsonMode, program.CandidateArchive{
					ArchivePath:   candidateArchive,
					ChecksumsPath: candidateChecksums,
				})
			}
			var (
				result program.Transition
				err    error
			)
			if rollback {
				config, readErr := runtime.Config.RollbackConfiguration()
				if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
					return fmt.Errorf("read configuration before program rollback: %w", readErr)
				}
				// A configuration this program cannot load names no credential;
				// the retained program's own compatibility check then decides.
				custody, _ := credentialCustody(runtime.Dependencies)
				result, err = runtime.Updater.Rollback(ctx.Context(), config, custody)
			} else if candidateArchive != "" {
				result, err = runtime.Updater.UpdateCandidate(ctx.Context(), runtime.Version, program.CandidateArchive{
					ArchivePath:   candidateArchive,
					ChecksumsPath: candidateChecksums,
				})
			} else {
				result, err = runtime.Updater.Update(ctx.Context(), runtime.Version)
			}
			if err != nil && result.State != "" {
				return reportIncompleteCleanup(runtime, jsonMode, rollback, result, err)
			}
			if !rollback && errors.Is(err, program.ErrCandidateIdentity) {
				return invocation.Problem(runtime,
					"Candidate has different program bytes at the current version",
					"The verified local archive does not match the active program identity.",
					"The active and retained programs are unchanged.",
					"Use the exact current release artifact or a verified newer version.", err)
			}
			if !rollback && errors.Is(err, program.ErrProgramStartupVerification) {
				return invocation.Problem(runtime,
					"Candidate program failed startup verification",
					"The candidate did not run its version check.",
					"The installed program is unchanged.",
					"Verify the candidate archive for this platform and retry only with a valid artifact.", err)
			}
			if errors.Is(err, program.ErrRollbackConfiguration) {
				return invocation.Problem(runtime,
					"Program rollback did not complete",
					"The retained program did not complete the isolated configuration-read check.",
					"The active program, retained program and configuration remain unchanged.",
					"Keep a compatible program, or use `aigw rollback` to restore the accepted configuration before retrying; do not delete policy fields manually.", err)
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
			title := "Update"
			if rollback {
				title = "Program rollback"
			} else if candidateArchive != "" {
				title = "Verified local candidate"
			}
			return reportTransition(runtime, jsonMode, title, result)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the version that an update would install, from public release metadata or the --candidate archive's name, without downloading or writing anything")
	cmd.Flags().BoolVar(&rollback, "rollback", false, "Roll back the portable AIGW program to the previous version offline")
	cmd.Flags().StringVar(&candidateArchive, "candidate", "", "Install one local portable archive without network access")
	cmd.Flags().StringVar(&candidateChecksums, "checksums", "", "Checksum manifest for --candidate")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the program transition as JSON")
	cmd.MarkFlagsRequiredTogether("candidate", "checksums")
	cmd.MarkFlagsMutuallyExclusive("rollback", "candidate")
	// A rollback preview would have to run the retained program to learn its
	// version, which a preview never does.
	cmd.MarkFlagsMutuallyExclusive("dry-run", "rollback")
	return cmd
}

// reportTransition writes a completed program transition and its next step:
// the active program reconciles the client settings that it owns.
func reportTransition(runtime invocation.Context, jsonMode bool, title string, result program.Transition) error {
	if jsonMode {
		return terminal.WriteDocument(runtime.Out, schemaVersion, true,
			updateDocument{State: result.State, Version: result.Version, Summary: result.Summary, NextAction: "aigw sync"})
	}
	r := invocation.Renderer(runtime)
	r.ProductTitle(title)
	r.Success(result.Summary)
	r.Text("Run the active version's sync command to reconcile owned client settings, then check readiness.")
	r.Next("aigw sync")
	return nil
}

// reportIncompleteCleanup reports a transition whose program replacement
// completed while an owned temporary resource could not be removed. As JSON,
// its own document carries the transition and the problem, with ok false.
func reportIncompleteCleanup(runtime invocation.Context, jsonMode, rollback bool, result program.Transition, cause error) error {
	title, impact := "Program updated; cleanup is incomplete", "The replacement is active; client settings and credentials were not changed."
	if rollback {
		title, impact = "Program restored; cleanup is incomplete", "The previous program is active and reads its credentials; client settings were not changed."
	}
	fix := "Run `aigw installation` and `aigw doctor` before retrying cleanup or another program transition."
	problem := invocation.Problem(runtime, title,
		"Program replacement completed, but an owned temporary resource could not be removed.",
		impact, fix, cause)
	if !jsonMode {
		return problem
	}
	document := updateDocument{State: result.State, Version: result.Version, Summary: result.Summary, Problem: title}
	document.NextAction, document.NextDetail = terminal.NextSteps(fix)
	if err := terminal.WriteDocument(runtime.Out, schemaVersion, false, document); err != nil {
		return err
	}
	return terminal.Presented(problem)
}
