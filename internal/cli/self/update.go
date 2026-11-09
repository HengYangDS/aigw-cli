package self

import (
	"errors"
	"fmt"
	"strings"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/program"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// updateSchemaVersion is the version of the JSON document of `aigw self
// update` and `aigw self rollback`, which continues from `aigw update`'s 1.
const updateSchemaVersion = 2

// updateDocument is the JSON document of `aigw self update` and `aigw self
// rollback`. Its state is the program transition's, or prepared when
// credentials are ready for an earlier Homebrew release; its version is the
// active program's afterward, and its summary says what happened. A
// transition that completed while a later step failed names that step's
// problem, and the document's ok is false.
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

// updatePreviewSuccess ends an update preview's human output.
const updatePreviewSuccess = "Preview did not download, verify or run a release, or change any file or Keychain item"

// newUpdateCommand constructs `aigw self update`, which replaces the portable
// program with a verified newer release, from a release source or from a
// local archive and its checksums.
func newUpdateCommand(runtime invocation.Context) *cobra.Command {
	var jsonMode, dryRun bool
	var archive program.CandidateArchive
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update this portable program to a verified newer release",
		Long: "Replace only a portable AIGW program; Homebrew upgrades a program that it\n" +
			"installed. Client settings and Tokens do not change. Run aigw sync, then\n" +
			"aigw check, before resuming clients. --archive installs a local release\n" +
			"archive, verified against --checksums, without network access.",
		Example: "aigw self update --dry-run\naigw self update\naigw self update --archive aigw.tar.gz --checksums checksums.txt --json",
		Args: cobra.MatchAll(cobra.NoArgs, func(cmd *cobra.Command, _ []string) error {
			for _, name := range []string{"archive", "checksums"} {
				if flag := cmd.Flags().Lookup(name); flag.Changed && strings.TrimSpace(flag.Value.String()) == "" {
					return fmt.Errorf("--%s requires a non-empty path; run `aigw self update --help`", name)
				}
			}
			return nil
		}),
		RunE: func(ctx *cobra.Command, _ []string) error {
			if err := program.RequirePortableOwnership(runtime.Executable); err != nil {
				if errors.Is(err, program.ErrHomebrewManaged) {
					return homebrewManaged("brew upgrade --cask aigw", "No program file changed.", err)
				}
				return err
			}
			if runtime.Updater == nil {
				return fmt.Errorf("automatic update is unavailable; install a verified release from GitHub")
			}
			if dryRun {
				return previewUpdate(ctx, runtime, jsonMode, archive)
			}
			var (
				result program.Transition
				err    error
			)
			if archive.ArchivePath != "" {
				result, err = runtime.Updater.UpdateCandidate(ctx.Context(), runtime.Version, archive)
			} else {
				result, err = runtime.Updater.Update(ctx.Context(), runtime.Version)
			}
			if err != nil && result.State != "" {
				return reportIncompleteCleanup(runtime, jsonMode, false, result, err)
			}
			switch {
			case errors.Is(err, program.ErrCandidateIdentity):
				return terminal.ProblemError(
					"The archive holds other program bytes at the current version",
					"The verified local archive does not match the active program.",
					"The active and retained programs are unchanged.",
					"Use the exact current release archive or a verified newer version.", err)
			case errors.Is(err, program.ErrProgramStartupVerification):
				return terminal.ProblemError(
					"The archive's program failed its startup check",
					"The program in the archive did not run its version check.",
					"The installed program is unchanged.",
					"Verify the archive for this platform, and run the update again only with a valid one.", err)
			case err != nil:
				return programProblem("Program update did not complete", "AIGW could not complete release verification and program replacement.",
					"No updated program version was confirmed active; client settings and credentials were not changed.",
					releaseFix(err, archive.ArchivePath != ""), err)
			}
			title := "Update"
			if archive.ArchivePath != "" {
				title = "Verified local archive"
			}
			return reportTransition(runtime, jsonMode, title, result)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the version that an update would install, from public release metadata or the --archive file's name, without downloading or writing anything")
	cmd.Flags().StringVar(&archive.ArchivePath, "archive", "", "Install one local release archive without network access")
	cmd.Flags().StringVar(&archive.ChecksumsPath, "checksums", "", "Checksum manifest for --archive")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the program transition as JSON")
	cmd.MarkFlagsRequiredTogether("archive", "checksums")
	return cmd
}

// previewUpdate reports the version that an update would install and the
// program file and rollback copy that it would write, from public release
// metadata, or from a local archive's name alone.
func previewUpdate(cmd *cobra.Command, runtime invocation.Context, jsonMode bool, archive program.CandidateArchive) error {
	var (
		result program.Transition
		err    error
	)
	if archive.ArchivePath != "" {
		result, err = runtime.Updater.PreviewCandidate(runtime.Version, archive)
	} else {
		result, err = runtime.Updater.Preview(cmd.Context(), runtime.Version)
	}
	if err != nil {
		title, evidence, impact := "Update preview did not complete", "AIGW could not determine the version that an update would install.",
			"No file, rollback copy or Keychain item changed."
		// A preview continues with a preview, even when the error names the
		// update that installs.
		if archive.ArchivePath == "" && errors.Is(err, program.ErrReleaseSource) {
			return terminal.ProblemError(title, evidence, impact, "aigw self update --dry-run --archive <archive> --checksums <checksums>", err)
		}
		return programProblem(title, evidence, impact, "aigw self status", err)
	}
	document := updateDocument{DryRun: true, State: result.State, Version: result.Version, Summary: result.Summary,
		CurrentVersion: runtime.Version, Path: runtime.Executable}
	// An update continues with the sync command, as the update itself does,
	// and applies with this program, which the path names.
	next := "aigw sync"
	if result.State == program.TransitionAvailable {
		document.Rollback = program.RollbackPath(runtime.Executable)
		next = invocation.ApplyWithThisProgram
	}
	document.NextAction, document.NextDetail = terminal.NextSteps(next)
	if jsonMode {
		return terminal.WriteDocument(runtime.Out, updateSchemaVersion, true, document)
	}
	r := invocation.Renderer(runtime)
	r.ProductTitle("Update preview")
	rows := []terminal.Field{
		{Label: "Installed version", Value: runtime.Version},
		{Label: "Release version", Value: result.Version},
		{Label: "Program file", Value: runtime.Executable},
	}
	if document.Rollback != "" {
		rows = append(rows, terminal.Field{Label: "Rollback copy", Value: document.Rollback})
	}
	r.Rows(rows...)
	r.Success(updatePreviewSuccess)
	r.Text(result.Summary)
	r.Next(next)
	return nil
}

// releaseFix is the fix of a failed update: a verified local archive when the
// release sources failed and the command did not already name one; otherwise
// the report of the active program.
func releaseFix(err error, archive bool) string {
	if !archive && errors.Is(err, program.ErrReleaseSource) {
		return "aigw self update --archive <archive> --checksums <checksums>"
	}
	return "aigw self status"
}

// reportTransition writes a completed program transition and its next step:
// the active program reconciles the client settings that it owns.
func reportTransition(runtime invocation.Context, jsonMode bool, title string, result program.Transition) error {
	if jsonMode {
		return terminal.WriteDocument(runtime.Out, updateSchemaVersion, true,
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
	fix := "Run `aigw self status` and `aigw check --depth local` before another program update or rollback."
	problem := terminal.ProblemError(title,
		"Program replacement completed, but an owned temporary resource could not be removed.",
		impact, fix, cause)
	if !jsonMode {
		return problem
	}
	document := updateDocument{State: result.State, Version: result.Version, Summary: result.Summary, Problem: title}
	document.NextAction, document.NextDetail = terminal.NextSteps(fix)
	if err := terminal.WriteDocument(runtime.Out, updateSchemaVersion, false, document); err != nil {
		return err
	}
	return terminal.Presented(problem)
}
