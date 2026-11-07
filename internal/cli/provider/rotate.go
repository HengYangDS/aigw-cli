package provider

import (
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/secret/probe"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

func newRotateCommand(runtime invocation.Context) *cobra.Command {
	var tokenStdin, jsonMode bool
	cmd := &cobra.Command{
		Use:   "rotate <provider>",
		Short: "Replace a provider's Token with one that the provider accepts",
		Long: "Replace a provider's Token with one that the provider accepts.\n" +
			"AIGW sends one authenticated request that names no model and keeps the previous Token unless the provider accepts the new one.",
		Example: "aigw provider rotate dmxapi\naigw provider rotate ucloud --token-stdin --json",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := providerArgument(cmd, args); err != nil {
				return err
			}
			return requireTokenInput(cmd, runtime, tokenStdin, jsonMode, false)
		},
		ValidArgsFunction: completeProvider(runtime),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, provider, err := loadProvider(runtime, args[0])
			if err != nil {
				return err
			}
			if secret.IsReadOnly(runtime.Secrets) {
				return readOnlyToken(provider.ID, "replace", "")
			}
			token, err := readToken(runtime, "Paste the "+provider.Label+" Token: ", tokenStdin)
			if err != nil {
				return err
			}
			if err := probe.Validate(cmd.Context(), runtime.HTTP, runtime.Clients.Admission(), provider, token); err != nil {
				return invocation.TokenValidationProblem(cmd, args, runtime.Secrets, provider.ID, func(protocol domain.EndpointProtocol) string {
					return "aigw provider edit " + provider.ID + " --endpoint " + protocol.Name() + "=<url>"
				}, err)
			}
			// A cancelled command stores nothing, even after the provider
			// accepted the Token.
			if err := cmd.Context().Err(); err != nil {
				return err
			}
			if _, err := secret.Replace(runtime.Secrets, map[string]string{provider.ID: token}); err != nil {
				return err
			}
			document := rotateDocument{ID: provider.ID, Label: provider.Label, TokenValidated: true, TokenStored: true}
			if jsonMode {
				document.NextAction, document.NextDetail = terminal.NextSteps("aigw check")
				return terminal.WriteDocument(runtime.Out, rotateSchemaVersion, true, document)
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Token replaced")
			r.Rows(append(providerFields(provider.ID, provider), terminal.Field{State: terminal.OK, Label: "Token", Value: "Accepted by the provider and stored"})...)
			r.Success("Clients read the new Token the next time they ask for one; a running client decides when that is")
			r.Next("aigw check")
			return r.Err()
		},
	}
	cmd.Flags().BoolVar(&tokenStdin, "token-stdin", false, "Read the Token from standard input")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the outcome as JSON")
	return cmd
}
