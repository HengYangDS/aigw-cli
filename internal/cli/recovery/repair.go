// Package recovery owns explicit configuration repair and synchronization
// commands; it never mutates client conversation history or model metadata.
package recovery

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/presentation"
	surfaceidentity "aigw-cli/internal/surface"
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
	Codex               []repairProjectionPreview `json:"codex"`
}

type repairProjectionPreview struct {
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
	after, discovered, enableClaude, newClaude, err := repairDesiredConfig(runtime, before)
	if err != nil {
		return err
	}
	if dryRun {
		plans, err := invocation.Synchronizer(runtime).Plan(before, after)
		if err != nil {
			return err
		}
		return renderRepairPreview(runtime, jsonMode, before, after, discovered, plans)
	}
	if enableClaude {
		if _, err := runtime.ClaudeLauncher.EnableClaude(); err != nil {
			return err
		}
	}
	if err := invocation.Synchronizer(runtime).Commit(ctx, before, after, "repair"); err != nil {
		if newClaude {
			_ = runtime.ClaudeLauncher.DisableClaude()
		}
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

func repairDesiredConfig(runtime invocation.Context, before configuration.Config) (configuration.Config, discovery.Result, bool, bool, error) {
	after := before.Clone()
	discovered, err := invocation.Discover(runtime)
	if err != nil {
		return configuration.Config{}, discovery.Result{}, false, false, err
	}
	claudeRuntime, _, claudeRouteErr := after.ResolveRuntime(configuration.ClientClaude, "")
	codexRuntime, _, codexRouteErr := after.ResolveRuntime(configuration.ClientCodex, "")
	enableClaude := false
	newClaude := false
	claudeAdapter := after.Adapters[configuration.ClientClaude]
	claudeExecutable := claudeAdapter.Executable
	if claudeExecutable == "" {
		claudeExecutable = discovered.ClaudeExecutable
	}
	if claudeRouteErr == nil && claudeExecutable != "" && claudeRuntime.Endpoint != "" {
		enableClaude = true
		newClaude = !claudeAdapter.Enabled
		after.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: claudeExecutable}
	}
	if codexRouteErr == nil && codexRuntime.Endpoint != "" {
		currentCodex := after.Adapters[configuration.ClientCodex]
		targets := repairCodexTargets(discovered, currentCodex.Targets)
		executable := currentCodex.Executable
		if discovered.CodexExecutable != "" {
			executable = discovered.CodexExecutable
		}
		if executable != "" && len(targets) > 0 {
			after.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: executable, Targets: targets}
		} else if currentCodex.Enabled && len(targets) == 0 {
			delete(after.Adapters, configuration.ClientCodex)
		}
	}
	return after, discovered, enableClaude, newClaude, nil
}

func renderRepairPreview(runtime invocation.Context, jsonMode bool, before, after configuration.Config, discovered discovery.Result, plans []codex.ProjectionPlan) error {
	preview := repairPreview{DryRun: true, ConfigurationAction: "already-converged", Codex: make([]repairProjectionPreview, 0, len(plans))}
	if !reflect.DeepEqual(before, after) {
		preview.ConfigurationAction = "update"
	}
	for _, plan := range plans {
		surfaceID := "codex-home-explicit"
		if surface, ok := discovered.SurfaceForConfigPath(plan.Target); ok {
			surfaceID = surface.ID
		}
		preview.Codex = append(preview.Codex, repairProjectionPreview{SurfaceID: surfaceID, Action: plan.Action})
	}
	if jsonMode {
		encoder := json.NewEncoder(runtime.Out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(preview)
	}
	r := invocation.Renderer(runtime)
	r.ProductTitle("Repair preview")
	r.Row("Configuration", preview.ConfigurationAction)
	for _, plan := range preview.Codex {
		r.Row(plan.SurfaceID, plan.Action)
	}
	r.Success("Preview did not write configuration, state files, authentication, launchers, or conversations")
	r.Next("aigw repair")
	return nil
}

func repairCodexTargets(discovered discovery.Result, current []string) []string {
	seen := map[string]bool{}
	targets := make([]string, 0, len(current)+len(discovered.Surfaces))
	appendTarget := func(path string) {
		if path != "" && !seen[path] {
			seen[path] = true
			targets = append(targets, path)
		}
	}
	for _, path := range discovered.AutoManagedCodexTargets() {
		appendTarget(path)
	}
	for _, path := range current {
		if surface, ok := discovered.SurfaceForConfigPath(path); ok {
			if surface.ID == string(surfaceidentity.CodexHomeDefault) {
				appendTarget(path)
			}
			continue
		}
		// An unknown existing target was explicitly configured by the user. It
		// remains an explicit AIGW-owned Codex Home candidate.
		appendTarget(path)
	}
	return targets
}
