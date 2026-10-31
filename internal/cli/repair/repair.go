// Package repair implements `aigw repair`, which rediscovers the clients and
// repairs their configuration. It never changes client conversation history or model metadata.
package repair

import (
	"context"
	"fmt"
	"reflect"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/app"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// schemaVersion is the version of `aigw repair`'s JSON document.
const schemaVersion = 2

// NewCommand constructs the command that reconciles recoverable AIGW-owned projection drift.
func NewCommand(runtime invocation.Context) *cobra.Command {
	var dryRun, jsonMode bool
	cmd := &cobra.Command{
		Use: "repair", Short: "Discover and repair client configuration", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return runRepair(cmd.Context(), runtime, dryRun, jsonMode) },
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview repair without writing configuration or client projections")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the secret-free repair preview or result as JSON")
	return cmd
}

// repairResult is `aigw repair`'s JSON document: what repair did or would do
// to the configuration, and in a preview the client files that it would write
// and the credential reader that it would install.
type repairResult struct {
	DryRun               bool                          `json:"dry_run,omitempty"`
	ConfigurationAction  string                        `json:"configuration_action"`
	Projections          []invocation.Projection       `json:"projections,omitempty"`
	CredentialEntrypoint *app.CredentialEntrypointPlan `json:"credential_entrypoint,omitempty"`
	NextAction           string                        `json:"next_action"`
}

func runRepair(ctx context.Context, runtime invocation.Context, dryRun, jsonMode bool) error {
	before, err := runtime.Config.Load()
	if err != nil {
		return err
	}
	if len(before.Routes) == 0 {
		return terminal.ProblemError("Not configured", "No Routes have been created.", "Cannot check, synchronize, or repair configuration that does not exist.", "aigw setup", fmt.Errorf("not configured"))
	}
	synchronizer := runtime.Synchronizer()
	after, _, err := synchronizer.DesiredClientConfiguration(before)
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
	var plans []adapter.ProjectionPlan
	var entrypoint *app.CredentialEntrypointPlan
	if dryRun {
		plans, err = synchronizer.Plan(before, after)
		if err != nil {
			return err
		}
		entrypoint, err = app.PlanCredentialEntrypoint(synchronizer, after)
		if err != nil {
			return err
		}
	} else if err := synchronizer.CommitProjection(ctx, before, after, "repair"); err != nil {
		return err
	}
	return renderRepairResult(runtime, dryRun, jsonMode, !reflect.DeepEqual(before, after), plans, entrypoint)
}

func renderRepairResult(runtime invocation.Context, dryRun, jsonMode, configurationChanged bool, plans []adapter.ProjectionPlan, entrypoint *app.CredentialEntrypointPlan) error {
	// Human output keeps the planner's spelling beside the projection rows;
	// the document spells it in snake_case.
	configuration := "already-converged"
	if configurationChanged {
		configuration = "update"
	}
	result := repairResult{DryRun: dryRun, ConfigurationAction: terminal.SnakeValue(configuration), CredentialEntrypoint: entrypoint, NextAction: "aigw check"}
	if dryRun && configurationChanged {
		result.NextAction = "aigw repair"
	}
	if entrypoint != nil {
		result.NextAction = "aigw repair"
	}
	for _, plan := range plans {
		if dryRun && plan.ChangesState {
			result.NextAction = "aigw repair"
		}
	}
	result.Projections = invocation.Projections(runtime, plans)
	if jsonMode {
		return terminal.WriteDocument(runtime.Out, schemaVersion, true, result)
	}
	r := invocation.Renderer(runtime)
	if dryRun {
		r.ProductTitle("Repair preview")
		rows := []terminal.Field{{Label: "Configuration", Value: configuration}}
		for _, plan := range plans {
			rows = append(rows, terminal.Field{Label: runtime.Clients.Admission().Label(plan.Client) + " · " + plan.Surface, Value: plan.Action})
		}
		r.Rows(rows...)
		if entrypoint != nil {
			// The entrypoint path is too long to share the projection column.
			r.Section("Credential entrypoint")
			r.Text("Install " + entrypoint.Path)
		}
		r.Success("Preview did not write configuration, state files, authentication, client executables, or conversations")
	} else {
		r.ProductTitle("Repair completed")
		r.Section("Results")
		r.Rows(
			terminal.Field{State: terminal.OK, Label: "Client", Value: "Rediscovered"},
			terminal.Field{State: terminal.OK, Label: "Configuration", Value: "Synchronized"},
		)
	}
	r.Next(result.NextAction)
	return nil
}
