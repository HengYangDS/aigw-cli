package model

import (
	"fmt"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

func newRemoveCommand(runtime invocation.Context) *cobra.Command {
	var dryRun, jsonMode bool
	var protocol domain.EndpointProtocol
	cmd := &cobra.Command{
		Use:   "remove <provider/model>",
		Short: "Remove a model that no client uses; its provider and Token stay",
		Example: "aigw model remove ucloud/old-sonnet\n" +
			"aigw model remove ucloud/old-sonnet --dry-run --json",
		Args:              modelArgument,
		ValidArgsFunction: invocation.CompleteModels(runtime),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, key, model, err := loadModel(runtime, args[0], protocolSelector(cmd, protocol))
			if err != nil {
				return err
			}
			ref := model.Ref()
			// A client that selects the model keeps it, enabled or not, and the
			// configuration keeps at least one model.
			if clients := cfg.SelectedClientsForModel(key); len(clients) > 0 {
				return terminal.ProblemError(
					fmt.Sprintf("%s %s %s, so it cannot be removed", clientLabels(runtime.Clients.Admission(), clients), uses(len(clients)), ref), "", "",
					fmt.Sprintf("aigw use %s <provider/model>", clients[0]), nil)
			}
			if len(cfg.Models) == 1 {
				return terminal.ProblemError(fmt.Sprintf("%s is the only model, and aigw needs at least one", ref), "", "",
					"aigw model add <provider/model>", nil)
			}
			before := cfg.Clone()
			item := describe(cfg, key)
			delete(cfg.Models, key)
			recommendations := cfg.RemoveRecommendedModel(key)
			rows := []terminal.Field{{Label: "Model", Value: item.ID}, {Label: "Label", Value: item.Label}}
			if len(recommendations) > 0 {
				rows = append(rows, terminal.Field{Label: "Recommendations", Value: "Dropped for " + clientLabels(runtime.Clients.Admission(), recommendations)})
			}
			return change{
				before: before, after: cfg, subject: "model remove", version: removeSchemaVersion, title: "Model removed", rows: rows,
				next: "aigw model list", document: changeDocument{modelItem: item, Recommendations: recommendations},
			}.apply(cmd, runtime, args, dryRun, jsonMode)
		},
	}
	invocation.BindProtocol(cmd, &protocol, "The model that declares this protocol, when one name names several")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the removal without writing anything")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the removed model as JSON")
	return cmd
}

// uses is the verb that agrees with a count of clients.
func uses(clients int) string {
	if clients == 1 {
		return "uses"
	}
	return "use"
}
