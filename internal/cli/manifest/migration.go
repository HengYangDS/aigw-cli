package manifest

import (
	"fmt"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/presentation"

	"github.com/spf13/cobra"
)

type migrationOutput struct {
	DryRun bool `json:"dry_run"`
	configuration.MigrationPlan
	NextAction string `json:"next_action"`
}

func newMigrateCommand(runtime invocation.Context) *cobra.Command {
	var dryRun, rollback, jsonMode bool
	command := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate retained local configuration to or from the current schema",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			plan, err := runtime.Config.PrepareMigration(rollback)
			if err != nil {
				return err
			}
			nextAction := "aigw check"
			if plan.Required {
				nextAction = "aigw sync"
			}
			if plan.Required && rollback {
				nextAction = "aigw update --rollback"
			}
			if dryRun && plan.Required {
				nextAction = "aigw config migrate"
				if rollback {
					nextAction += " --rollback"
				}
			} else if err := runtime.Config.ApplyMigration(plan); err != nil {
				return err
			}
			result := migrationOutput{DryRun: dryRun, MigrationPlan: plan, NextAction: nextAction}
			if jsonMode {
				return presentation.WriteJSON(runtime.Out, result)
			}
			return renderMigration(runtime, result)
		},
	}
	command.Flags().BoolVar(&dryRun, "dry-run", false, "Preview the exact schema transition without writing configuration")
	command.Flags().BoolVar(&rollback, "rollback", false, "Restore the retained predecessor configuration before program rollback")
	command.Flags().BoolVar(&jsonMode, "json", false, "Write the migration preview or result as JSON")
	return command
}

func renderMigration(runtime invocation.Context, result migrationOutput) error {
	renderer := invocation.Renderer(runtime)
	title := "Configuration migrated"
	if result.DryRun {
		title = "Configuration migration preview"
	} else if !result.Required {
		title = "Configuration is current"
	} else if result.Direction == configuration.MigrationRollback {
		title = "Configuration migration rolled back"
	}
	renderer.ProductTitle(title)
	renderer.Rows(
		presentation.Field{Label: "Schema", Value: fmt.Sprintf("%d → %d", result.FromVersion, result.ToVersion)},
		presentation.Field{Label: "Accounts", Value: fmt.Sprintf("%d", len(result.Accounts))},
		presentation.Field{Label: "Routes", Value: fmt.Sprintf("%d", len(result.Routes))},
	)
	if bindings := migrationClientRows(runtime.Clients.Admission(), result.Clients); len(bindings) > 0 {
		renderer.Section("Client Bindings")
		renderer.Rows(bindings...)
	}
	if len(result.Recommendations) > 0 {
		recommendations := make([]presentation.Field, 0, len(result.Recommendations))
		for _, client := range runtime.Clients.Admission().IDs() {
			if recommendation, exists := result.Recommendations[client]; exists {
				recommendations = append(recommendations, presentation.Field{Label: runtime.Clients.Admission().Label(client), Value: recommendation.Primary.Route})
			}
		}
		renderer.Section("Recommendations")
		renderer.Rows(recommendations...)
	}
	switch {
	case result.DryRun:
		renderer.Success("Preview did not write configuration, credentials, client files, or sessions")
	case !result.Required:
		renderer.Success("No migration was required")
	default:
		renderer.Success("Only AIGW configuration changed; credentials, client files, and sessions were preserved")
	}
	renderer.Next(result.NextAction)
	return renderer.Err()
}

func migrationClientRows(admission configuration.Admission, bindings map[string]configuration.ClientBinding) []presentation.Field {
	rows := make([]presentation.Field, 0, len(bindings))
	for _, client := range admission.IDs() {
		binding, exists := bindings[client]
		if !exists {
			continue
		}
		detail := binding.Route
		if detail == "" {
			detail = "disabled"
		}
		if binding.Protocol != "" {
			detail += " · " + configuration.ProtocolLabel(binding.Protocol)
		}
		switch targets := len(binding.Targets); {
		case targets == 1:
			detail += " · 1 native target"
		case targets > 1:
			detail += fmt.Sprintf(" · %d native targets", targets)
		}
		rows = append(rows, presentation.Field{Label: admission.Label(client), Value: detail})
	}
	return rows
}
