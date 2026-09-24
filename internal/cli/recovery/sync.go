package recovery

import (
	"errors"

	clientactivation "aigw-cli/internal/activation"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/client"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/presentation"

	"github.com/spf13/cobra"
)

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
			result := struct {
				DryRun         bool                    `json:"dry_run"`
				Selections     map[string]string       `json:"selections"`
				Targets        []client.ProjectionPlan `json:"targets,omitempty"`
				EnabledClients int                     `json:"enabled_clients"`
				State          string                  `json:"state,omitempty"`
				NextAction     string                  `json:"next_action"`
			}{DryRun: dryRun, Selections: map[string]string{}, NextAction: "aigw check"}
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
				plans, err := synchronizer.Plan(before, after)
				if err != nil {
					return err
				}
				result.Targets = plans
				for _, plan := range plans {
					if plan.ChangesState {
						result.NextAction = "aigw sync"
						break
					}
				}
			} else if err := synchronizer.CommitProjection(cmd.Context(), before, after, "sync"); err != nil {
				return err
			}
			if jsonMode {
				return presentation.WriteJSON(runtime.Out, result)
			}
			r := invocation.Renderer(runtime)
			if dryRun {
				r.ProductTitle("Synchronization preview")
				var bindings []presentation.Field
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
					"AIGW could not restore the selected configuration and its client projections.",
					"A rolled-back configuration was not confirmed.",
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
