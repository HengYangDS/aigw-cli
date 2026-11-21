package config

import (
	"errors"
	"os"

	clientactivation "aigw-cli/internal/app/activation"
	"aigw-cli/internal/app/synchronization"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// rollbackSchemaVersion is the version of `aigw config rollback`'s JSON
// document, which continues from `aigw rollback`'s 1.
const rollbackSchemaVersion = 3

// rollbackDocument is `aigw config rollback`'s JSON document: the source that
// the configuration was restored from. A preview names the source that it
// would restore and what restoring would change beyond the configuration.
type rollbackDocument struct {
	DryRun bool   `json:"dry_run,omitempty"`
	Source string `json:"source"`
	// Selections names each client whose selection the rollback restores to
	// another model than the one it uses now.
	Selections []restoredSelection `json:"selections,omitempty"`
	invocation.Preview
	NextAction string `json:"next_action,omitempty"`
	NextDetail string `json:"next_detail,omitempty"`
}

// restoredSelection is one client whose selection a rollback changes: the
// model that the restored configuration selects, absent when it selects
// none, and the model that the client uses now, a model chosen in the client
// included.
type restoredSelection struct {
	Client   string `json:"client"`
	Model    string `json:"model,omitempty"`
	Replaces string `json:"replaces,omitempty"`
}

// restoredSelections lists, in admission order, the clients for which the
// restored configuration selects another model than current, the working
// configuration, which follows a model chosen in a client. A restored
// selection of the model that the client's file already shows is that
// choice, which the invocation's report names as followed, and no entry
// here.
func restoredSelections(runtime invocation.Context, current, restored domain.Config) []restoredSelection {
	var selections []restoredSelection
	for _, client := range runtime.Clients.Admission().IDs() {
		if synchronization.SameSelection(current, restored, client) {
			continue
		}
		selection := restoredSelection{Client: client}
		if then := restored.Clients[client].Model; !then.IsZero() {
			selection.Model = then.String()
		}
		if now := current.Clients[client].Model; !now.IsZero() {
			selection.Replaces = now.String()
		}
		selections = append(selections, selection)
	}
	return selections
}

// renderRestoredSelections names the selections that a rollback restores.
func renderRestoredSelections(r *terminal.Renderer, runtime invocation.Context, selections []restoredSelection) {
	if len(selections) == 0 {
		return
	}
	rows := make([]terminal.Field, 0, len(selections))
	for _, selection := range selections {
		value := selection.Model
		switch {
		case value == "":
			value = "No model, in place of " + selection.Replaces
		case selection.Replaces != "":
			value += ", in place of " + selection.Replaces
		}
		rows = append(rows, terminal.Field{Label: runtime.Clients.Admission().Label(selection.Client), Value: value})
	}
	r.Section("Selections")
	r.Rows(rows...)
}

// rollbackFailureImpact reports the configuration outcome that a failed
// rollback verified, or the conservative state when it verified none.
func rollbackFailureImpact(err error) string {
	if impact, ok := terminal.ConfigurationRecoveryImpact(err); ok {
		return impact
	}
	return "The configuration or client files may have changed; inspect them before you run it again."
}

// rollbackUnavailable refuses a rollback without a recovery source. Without a
// configuration there is nothing to roll back, and a local check would fail
// the same way, so the fix is setup; otherwise the local check inspects the
// configuration that stays.
func rollbackUnavailable(runtime invocation.Context, cause error) error {
	fix := "aigw check --depth local"
	if _, err := os.Stat(runtime.Config.Path()); err != nil {
		// A configuration that loaded means its folder is there, so a blocked
		// path holds only when a file replaced the folder since the load.
		failure := platform.ClassifyPath(runtime.Config.Path(), platform.FileExpected, err)
		switch {
		case failure.Blocked():
			fix = platform.PathFix(failure, platform.PathContext{Home: invocation.Home(runtime.Dependencies), Configuration: runtime.Config.Path(), Rollback: true})
		case failure.Absent():
			fix = "aigw setup"
		}
	}
	return terminal.ProblemError(
		"Configuration rollback is unavailable",
		"No valid recovery source is available for the current configuration.",
		"The current configuration remains active and unchanged.",
		fix,
		cause,
	)
}

// nextAfterRollback is the step after a rollback. An enabled client of the
// restored configuration whose provider has no Token that AIGW can use waits:
// the rollback left its files as they were, so the step is the one that
// `aigw sync` names for it, where the local check would stop at the same
// Token. Without such a client the step is the local check.
func nextAfterRollback(restored domain.Config, secrets secret.Store) string {
	activation := clientactivation.AssessActivation(restored, secrets)
	for _, client := range restored.EnabledClientIDs() {
		if activation.ClientCredentialPrerequisites[client] != "" {
			return activation.NextActionFor(nil)
		}
	}
	return "aigw check --depth local"
}

// restoreSource is the configuration that a rollback restores, with its
// label and its document identifier: the latest fully verified snapshot,
// unless lastChange asks for the backup, or the backup when there is none.
func restoreSource(runtime invocation.Context, lastChange bool) (domain.Config, string, string, error) {
	var snapshotErr error
	if !lastChange {
		snapshot, err := runtime.Config.LoadVerifiedCheckpoint()
		if err == nil {
			return snapshot.Config, "Latest fully verified configuration", "verified_checkpoint", nil
		}
		snapshotErr = err
	}
	backup, err := runtime.Config.LoadBackup()
	if err != nil {
		return domain.Config{}, "", "", rollbackUnavailable(runtime, errors.Join(snapshotErr, err))
	}
	return backup, "Previous configuration", "previous_backup", nil
}

// newRollbackCommand constructs `aigw config rollback`, which restores the
// configuration and the client files from the verified snapshot, or else
// the previous backup.
func newRollbackCommand(runtime invocation.Context) *cobra.Command {
	var lastChange, jsonMode, dryRun bool
	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "Restore the configuration and client files, not the program",
		Long: "Restore the configuration and the client files from the latest fully\n" +
			"verified snapshot, or else from the previous backup. --last-change\n" +
			"selects only that backup. This does not replace the AIGW program.",
		Example: "aigw config rollback --dry-run\naigw config rollback\naigw config rollback --last-change --json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			current, err := runtime.LoadConfiguration()
			if err != nil {
				return err
			}
			restored, source, sourceID, err := restoreSource(runtime, lastChange)
			if err != nil {
				return err
			}
			// A rollback restores each client's selection as it was, a model
			// chosen in the client since then included, and names each one
			// that it restores. A refusal of its plan changed nothing and
			// passes on as it is.
			projection, err := runtime.Synchronizer().PreviewRestore(current, restored)
			if err != nil {
				return err
			}
			selections := restoredSelections(runtime, current, restored)
			if dryRun {
				preview, err := invocation.WithEntrypoint(runtime, restored, projection)
				if err != nil {
					return err
				}
				apply := invocation.ApplyCommand(cmd, nil)
				if jsonMode {
					document := rollbackDocument{DryRun: true, Source: sourceID, Selections: selections, Preview: preview}
					document.NextAction, document.NextDetail = terminal.NextSteps(apply)
					return terminal.WriteDocument(runtime.Out, rollbackSchemaVersion, true, document)
				}
				r := invocation.Renderer(runtime)
				r.ProductTitle("Rollback preview")
				r.Section("Restore source")
				r.Rows(terminal.Field{Label: "Configuration", Value: source})
				renderRestoredSelections(r, runtime, selections)
				invocation.RenderPreview(r, preview, apply)
				return nil
			}
			if err := runtime.Synchronizer().CommitRestore(cmd.Context(), current, restored, "rollback"); err != nil {
				// A refusal before anything was written names its own step.
				if _, refused := errors.AsType[*synchronization.PreflightError](err); refused {
					return err
				}
				return terminal.ProblemError(
					"Configuration rollback did not complete",
					"One rollback step failed; the cause identifies the affected boundary.",
					rollbackFailureImpact(err),
					"aigw check --depth local",
					err,
				)
			}
			next := nextAfterRollback(restored, runtime.Secrets)
			if jsonMode {
				document := rollbackDocument{Source: sourceID, Selections: selections}
				document.NextAction, document.NextDetail = terminal.NextSteps(next)
				return terminal.WriteDocument(runtime.Out, rollbackSchemaVersion, true, document)
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Rolled back safely")
			r.Section("Restore source")
			r.Rows(terminal.Field{Label: "Configuration", Value: source})
			renderRestoredSelections(r, runtime, selections)
			r.Success("Client selections and files were restored; clients were not restarted.")
			r.Next(next)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the source and the client files that a rollback would restore without writing them")
	cmd.Flags().BoolVar(&lastChange, "last-change", false, "Restore only the previous configuration backup")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the restore source as JSON")
	return cmd
}
