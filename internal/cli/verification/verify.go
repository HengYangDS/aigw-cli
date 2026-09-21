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
	var client, profileName string
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Run one minimal live request to verify the model protocol path",
		Args: cobra.MatchAll(cobra.NoArgs, func(_ *cobra.Command, _ []string) error {
			if client == "" {
				return fmt.Errorf("choose a verification client with --for; run `aigw verify --help`")
			}
			if client != "" && client != "all" && !configuration.IsAdmittedClient(client) {
				return fmt.Errorf("--for must be %s; run `aigw verify --help`", configuration.AdmittedClientUsage("all"))
			}
			if client == "all" && profileName != "" {
				return fmt.Errorf("--profile requires one explicit client, not --for all; run `aigw verify --help`")
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
				clientRuntime, err := cfg.ResolveRuntime(target, profileName)
				if err != nil {
					return err
				}
				clientRuntimes[target] = clientRuntime
				if client != "all" {
					continue
				}
				status := synchronizer.Inspect(cmd.Context(), cfg, target, clientRuntime)
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
				result, err := synchronizer.Verify(cmd.Context(), cfg, target, clientRuntime, profileName)
				if err != nil {
					return err
				}
				if result.Version != "" || result.SHA256 != "" {
					r.Detail(fmt.Sprintf("%s client: %s · SHA-256 %s", invocation.Title(target), result.Version, result.SHA256))
				}
				r.Status(presentation.OK, invocation.Title(target), clientRuntime.ProfileID+" · Completed")
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
	cmd.Flags().StringVar(&client, "for", "", "Client whose selected Profile to verify: "+configuration.AdmittedClientLabelUsage("all")+"; all means enabled clients")
	cmd.Flags().StringVar(&profileName, "profile", "", "Verify this Profile for the explicit client without changing its binding")
	return cmd
}
