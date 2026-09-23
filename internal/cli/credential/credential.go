// Package credential exposes the narrow token-helper surface consumed by
// admitted native clients. It never prints diagnostics to standard output.
package credential

import (
	"context"
	"errors"
	"fmt"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/presentation"

	"github.com/spf13/cobra"
)

// NewCommand builds the private credential-helper command used by admitted clients.
func NewCommand(runtime invocation.Context) *cobra.Command {
	return &cobra.Command{
		Use:    "credential <client> <projection-fingerprint>",
		Short:  "Read the Account Token matching a client projection",
		Hidden: true,
		Args:   cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			client := args[0]
			if !configuration.IsAdmittedClient(client) {
				return fmt.Errorf("credential helper requires an admitted client: %s", configuration.AdmittedClientUsage())
			}
			cfg, err := runtime.Config.Load()
			if err != nil {
				return presentation.ProblemError("Cannot read AIGW configuration", "", "Credential lookup did not start.", "Run `aigw doctor` to inspect the configuration.", err)
			}
			adapter := cfg.Clients[client]
			if !adapter.Enabled {
				return presentation.ProblemError(fmt.Sprintf("%s adapter is not enabled", client), "", "Credential lookup did not start.", fmt.Sprintf("Enable the %s adapter before using its credential helper.", client), nil)
			}
			clientRuntime, err := resolveProjectedRuntime(cfg, client, args[1])
			if err != nil {
				return presentation.ProblemError(fmt.Sprintf("%s credential projection no longer matches an available Account and endpoint", client), "", "No Account Token was read or returned.", "Run `aigw sync` and reload the client's configuration.", err)
			}
			if !clientRuntime.RequiresAccountToken() {
				return presentation.ProblemError(fmt.Sprintf("%s uses client-owned authentication", client), "", "AIGW does not supply an Account Token for this route.", fmt.Sprintf("Run `aigw verify --for %s` to verify authentication through the client.", client), nil)
			}
			token, err := runtime.Secrets.Get(clientRuntime.AccountID)
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					return presentation.ProblemError("Account Token read exceeded its deadline", "", "The credential subprocess was stopped; no Token was returned.", "Check the selected credential service before retrying; configuration synchronization cannot repair a stalled read.", err)
				}
				return presentation.ProblemError(fmt.Sprintf("%s Account Token is unavailable", client), "", "No usable credential was returned.", "Check the selected Account's credential in the configured backend.", err)
			}
			_, err = fmt.Fprintln(runtime.Out, token)
			if err != nil {
				return presentation.ProblemError("Cannot write the Account Token to the client", "", "Credential delivery did not complete.", "Check the client's credential-helper connection before retrying.", err)
			}
			return err
		},
	}
}

func resolveProjectedRuntime(cfg configuration.Config, client, fingerprint string) (configuration.Runtime, error) {
	spec, admitted := configuration.ClientSpecFor(client)
	if !admitted {
		return configuration.Runtime{}, fmt.Errorf("unknown client %q", client)
	}
	for _, routeID := range cfg.RouteIDs() {
		route := cfg.Routes[routeID]
		account, exists := cfg.Accounts[route.Account]
		if !exists {
			continue
		}
		account.ID = route.Account
		protocols := spec.CompatibleRouteProtocols(account, route)
		for _, protocol := range protocols {
			clientRuntime, err := cfg.ResolveRouteProtocol(client, routeID, protocol)
			if err != nil {
				continue
			}
			if clientRuntime.CredentialProjectionFingerprint(client) == fingerprint {
				return clientRuntime, nil
			}
		}
	}
	return configuration.Runtime{}, fmt.Errorf("unknown credential projection fingerprint")
}
