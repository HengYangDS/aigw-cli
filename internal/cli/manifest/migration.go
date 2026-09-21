package manifest

import (
	"fmt"
	"strings"

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
	renderer.Row("Schema", fmt.Sprintf("%d → %d", result.FromVersion, result.ToVersion))
	renderer.Row("Accounts", fmt.Sprintf("%d", len(result.Accounts)))
	renderer.Row("Profiles", fmt.Sprintf("%d", len(result.Profiles)))
	renderMigrationClients(renderer, result.Clients)
	if len(result.Recommendations) > 0 {
		recommendations := make([]string, 0, len(result.Recommendations))
		for _, client := range configuration.AdmittedClientIDs() {
			if selection, exists := result.Recommendations[client]; exists {
				recommendations = append(recommendations, client+"="+selection.Profile)
			}
		}
		renderer.Row("Recommendations", strings.Join(recommendations, ", "))
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

func renderMigrationClients(renderer *presentation.Renderer, bindings map[string]configuration.ClientBinding) {
	for _, client := range configuration.AdmittedClientIDs() {
		binding, exists := bindings[client]
		if !exists {
			continue
		}
		detail := binding.Profile
		if detail == "" {
			detail = "disabled"
		}
		if binding.Protocol != "" {
			detail += " · " + string(binding.Protocol)
		}
		if len(binding.Targets) > 0 {
			detail += fmt.Sprintf(" · %d native target(s)", len(binding.Targets))
		}
		renderer.Row(invocation.Title(client), detail)
	}
}
