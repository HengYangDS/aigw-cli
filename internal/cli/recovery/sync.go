package recovery

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/synchronization"
	"github.com/spf13/cobra"
)

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
				return err
			}
			if dryRun {
				plans, err := synchronizer.Plan(before, after)
				if err != nil {
					return err
				}
				preview := struct {
					DryRun  bool                             `json:"dry_run"`
					Routes  map[string]string                `json:"routes"`
					Targets []synchronization.ProjectionPlan `json:"targets"`
				}{DryRun: true, Routes: after.Routes, Targets: plans}
				if jsonMode {
					enc := json.NewEncoder(runtime.Out)
					enc.SetIndent("", "  ")
					return enc.Encode(preview)
				}
				r := invocation.Renderer(runtime)
				r.ProductTitle("Synchronization preview")
				for _, client := range configuration.AdmittedClientIDs() {
					r.Row("Route · "+client, after.Routes[client])
				}
				if len(plans) == 0 {
					r.Status(presentation.OK, "Projection", "No client configuration needs changing")
				} else {
					for _, plan := range plans {
						r.Row(plan.Target, plan.Action)
					}
				}
				r.Success("Preview did not write configuration, state files, authentication, or conversations")
				r.Next("aigw sync")
				return nil
			}
			if err := synchronizer.CommitProjection(cmd.Context(), before, after, "sync"); err != nil {
				return err
			}
			if !synchronization.ProjectionChanged(before, after) && !synchronization.ClaudeProjectionChanged(before, after) {
				if err := synchronizer.Reconcile(cmd.Context(), after, after); err != nil {
					return err
				}
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Synchronization completed")
			r.Success("Client configuration is aligned; authentication was unchanged")
			r.Next("aigw check")
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the synchronization plan without writing configuration")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write synchronization preview as JSON")
	return cmd
}

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
			if !lastChange {
				checkpoint, checkpointErr := runtime.Config.LoadVerifiedCheckpoint()
				if checkpointErr == nil {
					restored = checkpoint.Config
					source = "Latest fully verified configuration"
				} else if !errors.Is(checkpointErr, os.ErrNotExist) {
					return checkpointErr
				}
			}
			if source == "" {
				restored, err = runtime.Config.LoadBackup()
				if err != nil {
					if errors.Is(err, os.ErrNotExist) {
						return fmt.Errorf("No fully verified checkpoint or previous configuration backup is available")
					}
					return err
				}
				source = "Previous configuration"
			}
			if err := invocation.Synchronizer(runtime).Commit(cmd.Context(), current, restored, "rollback"); err != nil {
				return err
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Rolled back safely")
			r.Section("Restore source")
			r.Row("Configuration", source)
			r.Success("Routes and client projections were restored; clients were not restarted.")
			r.Next("aigw doctor")
			return nil
		},
	}
	cmd.Flags().BoolVar(&lastChange, "last-change", false, "Restore only the immediately previous configuration backup")
	return cmd
}
