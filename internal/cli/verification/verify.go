// Package verification owns explicit, quota-consuming live model verification
// and the secret-free verified checkpoint written after complete success.
package verification

import (
	"fmt"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/presentation"

	"github.com/spf13/cobra"
)

// NewCommand constructs the real-client verification command.
func NewCommand(runtime invocation.Context) *cobra.Command {
	var client, routeName string
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify a Route through a real native client (may use quota)",
		Long: "Run a bounded session through one real native client, or all enabled\n" +
			"clients with --for all. A session may make multiple provider requests\n" +
			"and may use quota. --for all writes a configuration checkpoint only\n" +
			"after every client succeeds. No client binding or native settings\n" +
			"are changed.",
		Args: cobra.MatchAll(cobra.NoArgs, func(_ *cobra.Command, _ []string) error {
			if client == "" {
				return fmt.Errorf("choose a verification client with --for; run `aigw verify --help`")
			}
			if client != "" && client != "all" && !runtime.Clients.Admission().Admits(client) {
				return fmt.Errorf("--for must be %s; run `aigw verify --help`", runtime.Clients.Admission().Usage("all"))
			}
			if client == "all" && routeName != "" {
				return fmt.Errorf("--route requires one explicit client, not --for all; run `aigw verify --help`")
			}
			return nil
		}),
		RunE: func(cmd *cobra.Command, _ []string) error {
			synchronizer := invocation.Synchronizer(runtime)
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			clients := []string{client}
			if client == "all" {
				clients = cfg.EnabledClientIDs()
				if len(clients) == 0 {
					return fmt.Errorf("no enabled clients to verify; run `aigw status`")
				}
			}
			clientRuntimes := make(map[string]configuration.Runtime, len(clients))
			for _, target := range clients {
				clientRuntime, err := cfg.ResolveRuntime(target, routeName)
				if err != nil {
					return err
				}
				clientRuntimes[target] = clientRuntime
				if client != "all" {
					continue
				}
				status := synchronizer.Inspect(cmd.Context(), cfg, target, clientRuntime)
				if !status.Ready {
					return presentation.ProblemError(
						fmt.Sprintf("Full verification requires a ready %s adapter: %s", runtime.Clients.Admission().Label(target), status.Issue), "",
						"No client was verified and the latest full verification checkpoint is unchanged.",
						status.RepairAction, nil,
					)
				}
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Live protocol verification")
			r.Section("Native client verification")
			r.Text("Each selected client runs a bounded verification session and may make multiple provider requests. AIGW does not modify client configuration or restart clients.")
			for _, target := range clients {
				clientRuntime := clientRuntimes[target]
				result, err := synchronizer.Verify(cmd.Context(), cfg, target, clientRuntime, routeName)
				if err != nil {
					return err
				}
				// Each client is a group of one, rendered as its result arrives.
				row := presentation.Field{State: presentation.OK, Label: runtime.Clients.Admission().Label(target), Value: clientRuntime.RouteID + " · Completed"}
				if result.Version != "" || result.SHA256 != "" {
					row.Details = append(row.Details, fmt.Sprintf("%s client: %s · SHA-256 %s", runtime.Clients.Admission().Label(target), result.Version, result.SHA256))
				}
				if result.Scope != "" {
					row.Details = append(row.Details, result.Scope)
				}
				r.Rows(row)
			}
			if client == "all" {
				if err := runtime.Config.SaveVerifiedCheckpoint(cmd.Context(), cfg, clients); err != nil {
					return err
				}
				r.Detail("Updated the latest full verification checkpoint.")
			}
			r.Next("aigw doctor")
			return nil
		},
	}
	cmd.Flags().StringVar(&client, "for", "", "Client whose selected Route to verify: "+runtime.Clients.Admission().Usage("all")+"; all means enabled clients")
	cmd.Flags().StringVar(&routeName, "route", "", "Verify this Route for the explicit client without changing its binding")
	return cmd
}
