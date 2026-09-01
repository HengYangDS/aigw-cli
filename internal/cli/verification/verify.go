// Package verification owns explicit, quota-consuming live model verification
// and the secret-free verified checkpoint written after complete success.
package verification

import (
	"context"
	"fmt"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/presentation"
	domainverification "aigw-cli/internal/verification"
	"github.com/spf13/cobra"
)

func NewCommand(runtime invocation.Context) *cobra.Command {
	var client, profileName string
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Run one minimal live request to verify the model protocol path",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileName != "" && client != "" {
				return fmt.Errorf("choose either --profile or --for, not both")
			}
			if client != "" && !configuration.IsAdmittedClient(client) && client != "all" {
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
			case configuration.IsAdmittedClient(client):
				clients = []string{client}
			case client == "all":
				clients = configuration.AdmittedClientIDs()
			default:
				return fmt.Errorf("--for must be %s; run `aigw verify --help`", configuration.AdmittedClientUsage("all"))
			}
			if client == "all" {
				if err := domainverification.ValidateFullReadiness(cfg); err != nil {
					return err
				}
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Live protocol verification")
			r.Section("Minimal request")
			r.Detail("This makes one minimal model request; it does not modify client configuration or restart clients.")
			for _, target := range clients {
				clientRuntime, err := cfg.ResolveRuntime(target, profileName)
				if err != nil {
					return err
				}
				ctx, cancel := context.WithTimeout(cmd.Context(), domainverification.ProtocolTimeout)
				if target == configuration.ClientCodex {
					identity, verifyErr := domainverification.VerifyCodexInvocation(ctx, runtime.Runner, cfg, clientRuntime)
					err = verifyErr
					if verifyErr == nil {
						r.Detail(fmt.Sprintf("Codex client: %s · SHA-256 %s", identity.Version, identity.SHA256))
					}
				} else {
					accountName := clientRuntime.AccountID
					token, tokenErr := runtime.Secrets.Get(accountName)
					if tokenErr != nil {
						cancel()
						instruction, _ := credential.TokenRecovery(runtime.Secrets, accountName)
						return fmt.Errorf("Token for account %q is unavailable: %w; %s", accountName, tokenErr, instruction)
					}
					err = domainverification.VerifyClaudeInvocation(ctx, runtime.Runner, cfg, clientRuntime, token)
				}
				cancel()
				if err != nil {
					return err
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
