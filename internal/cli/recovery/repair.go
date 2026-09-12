// Package recovery owns explicit configuration repair and synchronization
// commands; it never mutates client conversation history or model metadata.
package recovery

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/client"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/presentation"

	"github.com/spf13/cobra"
)

// NewRepairCommand constructs the command that reconciles recoverable AIGW-owned projection drift.
func NewRepairCommand(runtime invocation.Context) *cobra.Command {
	var dryRun, jsonMode bool
	cmd := &cobra.Command{
		Use: "repair", Short: "Discover and repair client configuration", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return runRepair(cmd.Context(), runtime, dryRun, jsonMode) },
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview repair without writing configuration or client projections")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the secret-free repair preview or result as JSON")
	return cmd
}

type repairResult struct {
	DryRun              bool                      `json:"dry_run"`
	ConfigurationAction string                    `json:"configuration_action"`
	Projections         []repairProjectionPreview `json:"projections,omitempty"`
	NextAction          string                    `json:"next_action"`
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
	synchronizer := invocation.Synchronizer(runtime)
	after, discovered, err := synchronizer.DesiredClientConfiguration(before)
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
	var plans []client.ProjectionPlan
	if dryRun {
		plans, err = synchronizer.Plan(before, after)
		if err != nil {
			return err
		}
	} else if err := synchronizer.CommitProjection(ctx, before, after, "repair"); err != nil {
		return err
	}
	return renderRepairResult(runtime, dryRun, jsonMode, !reflect.DeepEqual(before, after), discovered, plans)
}

func renderRepairResult(runtime invocation.Context, dryRun, jsonMode, configurationChanged bool, discovered discovery.Result, plans []client.ProjectionPlan) error {
	result := repairResult{DryRun: dryRun, ConfigurationAction: "already-converged", NextAction: "aigw check"}
	if dryRun {
		result.NextAction = "aigw repair"
	}
	if configurationChanged {
		result.ConfigurationAction = "update"
	}
	for _, plan := range plans {
		surfaceID := "claude-settings"
		if plan.Client == configuration.ClientCodex {
			surfaceID = "codex-home-explicit"
			if surface, ok := discovered.SurfaceForConfigPath(plan.Target); ok {
				surfaceID = surface.ID
			}
		}
		result.Projections = append(result.Projections, repairProjectionPreview{Client: plan.Client, SurfaceID: surfaceID, Action: plan.Action})
	}
	if jsonMode {
		encoder := json.NewEncoder(runtime.Out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}
	r := invocation.Renderer(runtime)
	if dryRun {
		r.ProductTitle("Repair preview")
		r.Row("Configuration", result.ConfigurationAction)
		for _, plan := range result.Projections {
			r.Row(plan.Client+" · "+plan.SurfaceID, plan.Action)
		}
		r.Success("Preview did not write configuration, state files, authentication, client executables, or conversations")
	} else {
		r.ProductTitle("Repair completed")
		r.Section("Results")
		r.Status(presentation.OK, "Client", "Rediscovered")
		r.Status(presentation.OK, "Configuration", "Synchronized")
	}
	r.Next(result.NextAction)
	return nil
}
