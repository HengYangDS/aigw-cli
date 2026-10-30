// Package rotate implements `aigw rotate`, which validates and stores a new
// Account Token.
package rotate

import (
	clientactivation "aigw-cli/internal/app/activation"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/secret/probe"
	"aigw-cli/internal/terminal"
	"fmt"

	"github.com/spf13/cobra"
)

// NewCommand constructs the command that rotates credentials for one existing account.
func NewCommand(runtime invocation.Context) *cobra.Command {
	var tokenStdin bool
	cmd := &cobra.Command{
		Use:   "rotate [account]",
		Short: "Update one Account Token",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			name, err := invocation.AccountReference(cfg, args)
			if err != nil {
				return err
			}
			accountName, account, err := cfg.ResolveAccount(name)
			if err != nil {
				return err
			}
			if instruction, writable := secret.TokenRecovery(runtime.Secrets, accountName); !writable {
				cause := fmt.Errorf("credential backend is read-only")
				return invocation.Problem(
					runtime,
					"Token cannot be rotated by this credential backend",
					"The selected environment backend reads Tokens from the parent process and does not persist changes.",
					"No Token was read, validated, or changed.",
					instruction,
					cause,
				)
			}
			var token string
			if tokenStdin {
				token, err = invocation.ReadToken(runtime, true, false)
			} else if runtime.Interactive {
				token, err = runtime.Prompt.Secret("Paste " + account.Label + " token: ")
			} else {
				return fmt.Errorf("Token input requires an interactive terminal; pipe one token line to `aigw rotate %s --token-stdin`", accountName)
			}
			if err != nil {
				return err
			}
			account.ID = accountName
			if err := probe.Validate(cmd.Context(), runtime.HTTP, runtime.Clients.Admission(), account, token); err != nil {
				return fmt.Errorf("Token validation failed: %w", err)
			}
			if err := cmd.Context().Err(); err != nil {
				return err
			}
			_, err = secret.Replace(runtime.Secrets, map[string]string{accountName: token})
			if err != nil {
				return err
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Token updated")
			r.Section("Account")
			r.Rows(
				terminal.Field{Label: "Account", Value: account.Label},
				terminal.Field{Label: "Account ID", Value: accountName},
				terminal.Field{State: terminal.OK, Label: "Account Token", Value: "Validated and securely stored"},
			)
			r.Success("Credential helpers read the new Token when next invoked; clients control their refresh timing")
			r.Next(clientactivation.NextActionAfterAccountConnection(cfg, accountName))
			return nil
		},
	}
	cmd.Flags().BoolVar(&tokenStdin, "token-stdin", false, "Read one token line from standard input")
	return cmd
}
