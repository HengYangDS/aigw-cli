package model

import (
	"fmt"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

func newRemoveCommand(runtime invocation.Context) *cobra.Command {
	var dryRun, jsonMode bool
	cmd := &cobra.Command{
		Use:   "remove <provider/model>",
		Short: "Remove a model that no client uses; its provider and Token stay",
		Example: "aigw model remove ucloud/old-sonnet\n" +
			"aigw model remove ucloud/old-sonnet --dry-run --json",
		Args:              modelArgument,
		ValidArgsFunction: invocation.CompleteModels(runtime, noClient),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, key, model, err := loadModel(runtime, args[0])
			if err != nil {
				return err
			}
			ref := model.Ref()
			// A client that selects the model keeps it, enabled or not.
			if clients := cfg.SelectedClientsForModel(key); len(clients) > 0 {
				label := runtime.Clients.Admission().Label(clients[0])
				return terminal.ProblemError(fmt.Sprintf("%s uses %s, so it cannot be removed", label, ref), "", "",
					fmt.Sprintf("aigw use %s <provider/model>", clients[0]), nil)
			}
			before := cfg.Clone()
			item := describe(cfg, key)
			delete(cfg.Models, key)
			recommendations := cfg.RemoveRecommendedModel(key)
			rows := []terminal.Field{{Label: "Model", Value: item.Model}, {Label: "Label", Value: item.Label}}
			if len(recommendations) > 0 {
				rows = append(rows, terminal.Field{Label: "Recommendations", Value: "Dropped for " + clientLabels(runtime.Clients.Admission(), recommendations)})
			}
			return change{
				before: before, after: cfg, subject: "model remove", version: removeSchemaVersion, title: "Model removed", rows: rows,
				next: "aigw model list", document: changeDocument{modelItem: item, Recommendations: recommendations},
			}.apply(cmd, runtime, args, dryRun, jsonMode)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the removal without writing anything")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the removed model as JSON")
	return cmd
}
