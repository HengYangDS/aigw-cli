// Package recovery owns explicit configuration repair and synchronization
// commands; it never mutates client conversation history or model metadata.
package recovery

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/synchronization"
	"github.com/spf13/cobra"
)

func NewRepairCommand(runtime invocation.Context) *cobra.Command {
	var dryRun, jsonMode bool
	cmd := &cobra.Command{
		Use: "repair", Short: "Discover and repair client configuration", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return runRepair(cmd.Context(), runtime, dryRun, jsonMode) },
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview repair without writing configuration or authentication")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write a secret-free repair preview as JSON")
	return cmd
}

type repairPreview struct {
	DryRun              bool                      `json:"dry_run"`
	ConfigurationAction string                    `json:"configuration_action"`
	Projections         []repairProjectionPreview `json:"projections"`
}

type repairProjectionPreview struct {
	Client    string `json:"client"`
	SurfaceID string `json:"surface_id"`
	Action    string `json:"action"`
}

func runRepair(ctx context.Context, runtime invocation.Context, dryRun, jsonMode bool) error {
	before, err := runtime.Config.Load()
	if err != nil {
		return err
	}
	if len(before.Profiles) == 0 {
		return presentation.ProblemError("Not configured", "No service profiles have been created.", "Cannot check, synchronize, or repair configuration that does not exist.", "aigw setup", fmt.Errorf("not configured"))
	}
	after, discovered, err := invocation.Synchronizer(runtime).DesiredClientConfiguration(before)
	if err != nil {
		return invocation.Problem(
			runtime,
			"Repair prerequisites are unavailable",
			"AIGW could not inspect the current clients and configuration needed to plan a repair.",
			"Configuration and client projections remain unchanged.",
			"aigw doctor",
			err,
		)
	}
	if dryRun {
		plans, err := invocation.Synchronizer(runtime).Plan(before, after)
		if err != nil {
			return err
		}
		return renderRepairPreview(runtime, jsonMode, before, after, discovered, plans)
	}
	if err := invocation.Synchronizer(runtime).Commit(ctx, before, after, "repair"); err != nil {
		return err
	}
	if after.Adapters[configuration.ClientCodex].Enabled && !synchronization.ProjectionChanged(before, after) {
		if err := invocation.Synchronizer(runtime).Reconcile(ctx, after, after); err != nil {
			return fmt.Errorf("Failed to repair Codex configuration projection: %w", err)
		}
	}
	r := invocation.Renderer(runtime)
	r.ProductTitle("Repair completed")
	r.Section("Results")
	r.Status(presentation.OK, "Client", "Rediscovered")
	r.Status(presentation.OK, "Configuration", "Synchronized")
	authentication := "Unchanged"
	if synchronization.AuthenticationChanged(before, after) {
		authentication = "Bound"
	}
	r.Status(presentation.OK, "Authentication", authentication)
	r.Next("aigw check")
	return nil
}

func renderRepairPreview(runtime invocation.Context, jsonMode bool, before, after configuration.Config, discovered discovery.Result, plans []synchronization.ProjectionPlan) error {
	preview := repairPreview{DryRun: true, ConfigurationAction: "already-converged", Projections: make([]repairProjectionPreview, 0, len(plans))}
	if !reflect.DeepEqual(before, after) {
		preview.ConfigurationAction = "update"
	}
	for _, plan := range plans {
		surfaceID := "claude-settings"
		if plan.Client == configuration.ClientCodex {
			surfaceID = "codex-home-explicit"
			if surface, ok := discovered.SurfaceForConfigPath(plan.Target); ok {
				surfaceID = surface.ID
			}
		}
		preview.Projections = append(preview.Projections, repairProjectionPreview{Client: plan.Client, SurfaceID: surfaceID, Action: plan.Action})
	}
	if jsonMode {
		encoder := json.NewEncoder(runtime.Out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(preview)
	}
	r := invocation.Renderer(runtime)
	r.ProductTitle("Repair preview")
	r.Row("Configuration", preview.ConfigurationAction)
	for _, plan := range preview.Projections {
		r.Row(plan.Client+" · "+plan.SurfaceID, plan.Action)
	}
	r.Success("Preview did not write configuration, state files, authentication, client executables, or conversations")
	r.Next("aigw repair")
	return nil
}
