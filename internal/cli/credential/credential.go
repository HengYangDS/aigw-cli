// Package credential exposes the narrow token-helper surface consumed by
// admitted native clients. It never prints diagnostics to standard output.
package credential

import (
	"fmt"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"github.com/spf13/cobra"
)

// NewCommand builds the private credential-helper command used by admitted clients.
func NewCommand(runtime invocation.Context) *cobra.Command {
	return &cobra.Command{
		Use:    "credential <client>",
		Short:  "Read the active client gateway credential",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			client := args[0]
			if !configuration.IsAdmittedClient(client) {
				return fmt.Errorf("credential helper requires an admitted client: %s", configuration.AdmittedClientUsage())
			}
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			adapter := cfg.Adapters[client]
			if !adapter.Enabled {
				return fmt.Errorf("%s adapter is not enabled", client)
			}
			clientRuntime, err := cfg.ResolveRuntime(client, "")
			if err != nil {
				return err
			}
			if !clientRuntime.RequiresAccountToken() {
				return fmt.Errorf(
					"%s uses client-owned authentication; run `aigw verify --for %s` to verify it through the client",
					clientRuntime.ProfileID,
					client,
				)
			}
			token, err := runtime.Secrets.Get(clientRuntime.AccountID)
			if err != nil {
				return fmt.Errorf("%s gateway credential is unavailable: %w", client, err)
			}
			_, err = fmt.Fprintln(runtime.Out, token)
			return err
		},
	}
}
