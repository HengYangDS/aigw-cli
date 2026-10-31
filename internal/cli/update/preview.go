package update

import (
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/program"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// previewSuccess ends an update preview's human output.
const previewSuccess = "Preview did not download, verify or run a release, or change any file or Keychain item"

// runPreview reports the version that an update would install and the
// program file and rollback copy that it would write, from public release
// metadata, or from a local candidate archive's name alone.
func runPreview(cmd *cobra.Command, runtime invocation.Context, jsonMode bool, candidate program.CandidateArchive) error {
	var (
		result program.Transition
		err    error
	)
	if candidate.ArchivePath != "" {
		result, err = runtime.Updater.PreviewCandidate(runtime.Version, candidate)
	} else {
		result, err = runtime.Updater.Preview(cmd.Context(), runtime.Version)
	}
	if err != nil {
		return invocation.Problem(runtime,
			"Update preview did not complete",
			"AIGW could not determine the version that an update would install.",
			"No file, rollback copy or Keychain item changed.",
			"Run `aigw doctor` to inspect current state; verify the selected release source or use a verified local archive.", err)
	}
	document := updateDocument{DryRun: true, State: result.State, Version: result.Version, Summary: result.Summary,
		CurrentVersion: runtime.Version, Path: runtime.Executable}
	// An update continues with the sync command, as update itself does, and
	// applies with this program, which the path names.
	next := "aigw sync"
	if result.State == program.TransitionAvailable {
		document.Rollback = program.RollbackPath(runtime.Executable)
		next = invocation.ApplyWithThisProgram
	}
	document.NextAction, document.NextDetail = terminal.NextSteps(next)
	if jsonMode {
		return terminal.WriteDocument(runtime.Out, schemaVersion, true, document)
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
	r.Success(previewSuccess)
	r.Text(result.Summary)
	r.Next(next)
	return nil
}
