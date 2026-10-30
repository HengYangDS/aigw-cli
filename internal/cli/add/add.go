// Package add implements `aigw add`, which connects one Account with its first
// Route.
package add

import (
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/terminal"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// NewCommand constructs the guided command that connects one Account and its first Route.
func NewCommand(runtime invocation.Context) *cobra.Command {
	var label, openAIURL, anthropicURL, client, model string
	var tokenStdin bool
	cmd := &cobra.Command{
		Use:   "add <account>",
		Short: "Add and connect one Account with its first Route",
		Args: cobra.MatchAll(cobra.ExactArgs(1), func(cmd *cobra.Command, args []string) error {
			if !domain.ValidIdentifier(args[0]) {
				return fmt.Errorf("Invalid account ID %q; use letters, numbers, dots, hyphens, or underscores; run `%s --help`", args[0], cmd.CommandPath())
			}
			if !runtime.Clients.Admission().Admits(client) || strings.TrimSpace(model) == "" {
				return fmt.Errorf("--for and --model are required; --for must be %s; run `%s --help`", runtime.Clients.Admission().Usage(), cmd.CommandPath())
			}
			return nil
		}),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfg, err := runtime.Config.Load()
			if err != nil {
				return err
			}
			if label == "" {
				label = name
			}
			account := domain.Account{Label: label, Endpoints: domain.Endpoints{
				OpenAIResponses: strings.TrimRight(openAIURL, "/"),
				Anthropic:       strings.TrimRight(anthropicURL, "/"),
			}}
			account.ID = name
			spec, _ := runtime.Clients.Admission().Spec(client)
			_, protocol, err := spec.ResolveEndpoint(account, "")
			if err != nil {
				return err
			}
			route := domain.Route{
				Label: label, Account: name, Model: strings.TrimSpace(model),
				Interfaces: map[domain.EndpointProtocol][]domain.Capability{protocol: {}},
			}
			acquireToken := func() (string, error) { return invocation.ReadToken(runtime, tokenStdin, true) }
			if err := runtime.Synchronizer().ConnectAccount(cmd.Context(), cfg, name, client, account, route, acquireToken); err != nil {
				return fmt.Errorf("%w; run `%s --help`", err, cmd.CommandPath())
			}
			r := invocation.Renderer(runtime)
			r.ProductTitle("Account connected")
			r.Section("Selection")
			r.Rows(
				terminal.Field{Label: "Account", Value: label},
				terminal.Field{Label: "Account ID", Value: name},
				terminal.Field{Label: "Route ID", Value: name},
				terminal.Field{Label: "Client", Value: runtime.Clients.Admission().Label(client)},
				terminal.Field{Label: "Model", Value: route.Model},
				terminal.Field{State: terminal.OK, Label: "Token", Value: "Securely stored"},
			)
			r.Next("aigw check")
			return nil
		},
	}
	cmd.Flags().StringVar(&label, "label", "", "Provider display name")
	cmd.Flags().StringVar(&openAIURL, "openai-url", "", "OpenAI Responses base URL")
	cmd.Flags().StringVar(&anthropicURL, "anthropic-url", "", "Anthropic base URL")
	cmd.Flags().StringVar(&client, "for", "", "Client: "+runtime.Clients.Admission().Usage())
	cmd.Flags().StringVar(&model, "model", "", "Upstream model ID")
	cmd.Flags().BoolVar(&tokenStdin, "token-stdin", false, "Read one token line from standard input")
	return cmd
}
