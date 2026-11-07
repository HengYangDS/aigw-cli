// Package rotate implements `aigw rotate`, which validates and stores a new
// Account Token.
package rotate

import (
	"fmt"

	clientactivation "aigw-cli/internal/app/activation"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/secret/probe"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// schemaVersion is the version of `aigw rotate`'s JSON document.
const schemaVersion = 1

// rotateDocument is `aigw rotate`'s JSON document.
type rotateDocument struct {
	Provider       string `json:"account"`
	Label          string `json:"label"`
	TokenValidated bool   `json:"token_validated"`
	TokenStored    bool   `json:"token_stored"`
	NextAction     string `json:"next_action,omitempty"`
	NextDetail     string `json:"next_detail,omitempty"`
}

// NewCommand constructs the command that rotates credentials for one existing account.
func NewCommand(runtime invocation.Context) *cobra.Command {
	var tokenStdin, jsonMode bool
	cmd := &cobra.Command{
		Use:   "rotate [account]",
		Short: "Update one Account Token",
		Args: cobra.MatchAll(cobra.MaximumNArgs(1), func(cmd *cobra.Command, _ []string) error {
			if jsonMode && !tokenStdin {
				return fmt.Errorf("--json reads the Token only from --token-stdin; run `%s --help`", cmd.CommandPath())
			}
			return nil
		}),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			name, err := invocation.ProviderReference(cfg, args)
			if err != nil {
				return err
			}
			accountName, account, err := cfg.ResolveProvider(name)
			if err != nil {
				return err
			}
			if instruction, writable := secret.TokenRecovery(runtime.Secrets, accountName); !writable {
				cause := fmt.Errorf("credential backend is read-only")
				return terminal.ProblemError(
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
				return invocation.TokenValidationProblem(cmd, args, runtime.Secrets, accountName, func(protocol domain.EndpointProtocol) string {
					return editEndpoint(accountName, protocol)
				}, err)
			}
			if err := cmd.Context().Err(); err != nil {
				return err
			}
			_, err = secret.Replace(runtime.Secrets, map[string]string{accountName: token})
			if err != nil {
				return err
			}
			next := clientactivation.NextActionAfterProviderConnection(cfg, accountName)
			if jsonMode {
				document := rotateDocument{Provider: accountName, Label: account.Label, TokenValidated: true, TokenStored: true}
				document.NextAction, document.NextDetail = terminal.NextSteps(next)
				return terminal.WriteDocument(runtime.Out, schemaVersion, true, document)
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
			r.Next(next)
			return nil
		},
	}
	cmd.Flags().BoolVar(&tokenStdin, "token-stdin", false, "Read one token line from standard input")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the rotated Account Token's outcome as JSON")
	return cmd
}

// editEndpoint is the account edit that corrects the endpoint that the Token
// probe reached: the OpenAI Responses endpoint when the Account has one, and
// otherwise the Anthropic endpoint.
func editEndpoint(accountName string, protocol domain.EndpointProtocol) string {
	flag := "--anthropic-url"
	if protocol == domain.ProtocolOpenAIResponses {
		flag = "--openai-url"
	}
	return "aigw account edit " + accountName + " " + flag + " <url>"
}
