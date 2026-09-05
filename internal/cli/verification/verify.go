// Package verification owns explicit, quota-consuming live model verification
// and the secret-free verified checkpoint written after complete success.
package verification

import (
	"fmt"
	"slices"

	"aigw-cli/internal/cli/invocation"
	clientdomain "aigw-cli/internal/client"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/presentation"
	"github.com/spf13/cobra"
)

func NewCommand(runtime invocation.Context) *cobra.Command {
	var client, profileName string
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Run one minimal live request to verify the model protocol path",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			synchronizer := invocation.Synchronizer(runtime)
			admittedClients := synchronizer.ClientIDs()
			if profileName != "" && client != "" {
				return fmt.Errorf("choose either --profile or --for, not both")
			}
			if client != "" && !slices.Contains(admittedClients, client) && client != "all" {
				return fmt.Errorf("--for must be %s; run `aigw verify --help`", configuration.AdmittedClientUsage("all"))
			}
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			if profileName != "" && client == "" {
				client, err = cfg.ClientForProfile(profileName)
				if err != nil {
					return err
				}
			}
			var clients []string
			switch {
			case slices.Contains(admittedClients, client):
				clients = []string{client}
			case client == "all":
				clients = admittedClients
			default:
				return fmt.Errorf("--for must be %s; run `aigw verify --help`", configuration.AdmittedClientUsage("all"))
			}
			clientRuntimes := make(map[string]configuration.Runtime, len(clients))
			for _, target := range clients {
				clientRuntime, err := cfg.ResolveRuntime(target, profileName)
				if err != nil {
					return err
				}
				clientRuntimes[target] = clientRuntime
				if client != "all" {
					continue
				}
				status := synchronizer.Inspect(cmd.Context(), cfg, target, clientRuntime, clientdomain.InspectionOptions{})
				if !status.Ready {
					return fmt.Errorf("Full verification requires a ready %s adapter: %s; run `%s`", invocation.Title(target), status.Issue, status.RepairAction)
				}
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Live protocol verification")
			r.Section("Minimal request")
			r.Detail("This makes one minimal model request; it does not modify client configuration or restart clients.")
			for _, target := range clients {
				clientRuntime := clientRuntimes[target]
				result, err := synchronizer.Verify(cmd.Context(), cfg, target, clientRuntime)
				if err != nil {
					return err
				}
				if result.Version != "" || result.SHA256 != "" {
					r.Detail(fmt.Sprintf("%s client: %s · SHA-256 %s", invocation.Title(target), result.Version, result.SHA256))
				}
				r.Status(presentation.OK, invocation.Title(target), clientRuntime.ProfileID+" · Completed")
			}
			if client == "all" {
				if err := runtime.Config.SaveVerifiedCheckpoint(cfg, clients); err != nil {
					return err
				}
				r.Detail("Updated the latest full verification checkpoint.")
			}
			r.Next("aigw doctor")
			return nil
		},
	}
	cmd.Flags().StringVar(&client, "for", "", "Verify the selected Route for "+configuration.AdmittedClientLabelUsage("all"))
	cmd.Flags().StringVar(&profileName, "profile", "", "Verify one Profile using its declared client without changing Routes")
	return cmd
}
