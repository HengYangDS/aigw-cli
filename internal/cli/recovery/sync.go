package recovery

import (
	"errors"

	clientactivation "aigw-cli/internal/activation"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/client"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/synchronization"

	"github.com/spf13/cobra"
)

type credentialEntrypointPlan struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}

func (plan credentialEntrypointPlan) humanAction() string {
	verb := "Install"
	if plan.Action == string(synchronization.CredentialEntrypointRemove) {
		verb = "Remove"
	}
	return verb + " " + plan.Path
}

type syncResult struct {
	DryRun               bool                      `json:"dry_run"`
	Selections           map[string]string         `json:"selections"`
	Targets              []client.ProjectionPlan   `json:"targets,omitempty"`
	CredentialEntrypoint *credentialEntrypointPlan `json:"credential_entrypoint,omitempty"`
	EnabledClients       int                       `json:"enabled_clients"`
	State                string                    `json:"state,omitempty"`
	NextAction           string                    `json:"next_action"`
}

// NewSyncCommand constructs the command that projects current bindings to discovered clients.
func NewSyncCommand(runtime invocation.Context) *cobra.Command {
	var dryRun bool
	var jsonMode bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Resynchronize client configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			before, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			synchronizer := invocation.Synchronizer(runtime)
			after, _, err := synchronizer.DesiredClientConfiguration(before)
			if err != nil {
				return invocation.Problem(
					runtime,
					"Synchronization prerequisites are unavailable",
					"AIGW could not determine which selected Client Bindings can be projected with the currently available clients and credentials.",
					"Configuration and client projections remain unchanged.",
					"aigw doctor",
					err,
				)
			}
			result := syncResult{DryRun: dryRun, Selections: map[string]string{}, NextAction: "aigw check"}
			activation := clientactivation.AssessActivation(after, runtime.Secrets)
			result.EnabledClients = activation.EnabledClients
			result.State = string(activation.State)
			if activation.EnabledClients == 0 {
				result.NextAction = activation.NextAction
			}
			for _, client := range configuration.AdmittedClientIDs() {
				if route := after.SelectedRoute(client); route != "" {
					result.Selections[client] = route
				}
			}
			if dryRun {
				if err := prepareSyncPreview(synchronizer, before, after, runtime.CredentialPath, &result); err != nil {
					return err
				}
			} else if err := synchronizer.CommitProjection(cmd.Context(), before, after, "sync"); err != nil {
				return err
			}
			if jsonMode {
				return presentation.WriteJSON(runtime.Out, result)
			}
			r := invocation.Renderer(runtime)
			if dryRun {
				renderSyncPreview(r, after, result)
			} else {
				r.ProductTitle("Synchronization completed")
			}
			if result.EnabledClients == 0 {
				r.Status(presentation.Info, "Client activation", "No client is enabled")
				r.Detail("Authentication was unchanged; connect one compatible Account, then synchronize")
			} else if !dryRun {
				r.Success("Client configuration is aligned; authentication was unchanged")
			}
			r.Next(result.NextAction)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the synchronization plan without writing configuration")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the synchronization preview or result as JSON")
	return cmd
}

func prepareSyncPreview(synchronizer synchronization.Synchronizer, before, after configuration.Config, credentialPath string, result *syncResult) error {
	plans, err := synchronizer.Plan(before, after)
	if err != nil {
		return err
	}
	entrypoint, err := planCredentialEntrypoint(synchronizer, after, credentialPath)
	if err != nil {
		return err
	}
	result.Targets = plans
	result.CredentialEntrypoint = entrypoint
	if entrypoint != nil {
		result.NextAction = "aigw sync"
	}
	for _, plan := range plans {
		if plan.ChangesState {
			result.NextAction = "aigw sync"
			break
		}
	}
	return nil
}

func planCredentialEntrypoint(synchronizer synchronization.Synchronizer, after configuration.Config, path string) (*credentialEntrypointPlan, error) {
	action, err := synchronizer.CredentialEntrypointPlan(after)
	if err != nil || action == synchronization.CredentialEntrypointUnchanged {
		return nil, err
	}
	return &credentialEntrypointPlan{Path: path, Action: string(action)}, nil
}

func renderSyncPreview(r *presentation.Renderer, after configuration.Config, result syncResult) {
	r.ProductTitle("Synchronization preview")
	if result.CredentialEntrypoint != nil {
		r.Row("Credential entrypoint", result.CredentialEntrypoint.humanAction())
	}
	bindings := make([]presentation.Field, 0, len(configuration.AdmittedClientIDs()))
	for _, client := range configuration.AdmittedClientIDs() {
		bindings = append(bindings, presentation.Field{Label: "Client · " + client, Value: after.SelectedRoute(client)})
	}
	r.Rows(bindings...)
	if len(result.Targets) == 0 {
		r.Status(presentation.OK, "Projection", "No client configuration needs changing")
	} else {
		targets := make([]presentation.Field, 0, len(result.Targets))
		for _, plan := range result.Targets {
			targets = append(targets, presentation.Field{Label: plan.Target, Value: plan.Action})
		}
		r.Rows(targets...)
	}
	r.Success("Preview did not write configuration, state files, authentication, or conversations")
}

// NewRollbackCommand constructs the command that restores the last verified AIGW configuration checkpoint.
func NewRollbackCommand(runtime invocation.Context) *cobra.Command {
	var lastChange bool
	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "Roll back to the latest fully verified configuration or the previous configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			current, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			restored := configuration.Config{}
			source := ""
			var checkpointErr error
			if !lastChange {
				checkpoint, loadErr := runtime.Config.LoadVerifiedCheckpoint()
				checkpointErr = loadErr
				if checkpointErr == nil {
					restored = checkpoint.Config
					source = "Latest fully verified configuration"
				}
			}
			if source == "" {
				var backupErr error
				restored, backupErr = runtime.Config.LoadBackup()
				if backupErr != nil {
					cause := backupErr
					if checkpointErr != nil {
						cause = errors.Join(checkpointErr, backupErr)
					}
					return invocation.Problem(
						runtime,
						"Configuration rollback is unavailable",
						"No valid recovery source is available for the current configuration.",
						"The current configuration remains active and unchanged.",
						"aigw doctor",
						cause,
					)
				}
				source = "Previous configuration"
			}
			if err := invocation.Synchronizer(runtime).Commit(cmd.Context(), current, restored, "rollback"); err != nil {
				return invocation.Problem(
					runtime,
					"Configuration rollback did not complete",
					"One rollback step failed; the cause identifies the affected boundary.",
					"Configuration or client projections may have changed; inspect current state before retrying.",
					"aigw doctor",
					err,
				)
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Rolled back safely")
			r.Section("Restore source")
			r.Row("Configuration", source)
			r.Success("Client selections and projections were restored; clients were not restarted.")
			r.Next("aigw doctor")
			return nil
		},
	}
	cmd.Flags().BoolVar(&lastChange, "last-change", false, "Restore only the immediately previous configuration backup")
	return cmd
}
