// Package add implements `aigw add`, which connects one Account with its first
// Route.
package add

import (
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// schemaVersion is the version of `aigw add`'s JSON document.
const schemaVersion = 1

// addDocument is `aigw add`'s JSON document: the Account and Route that it
// connected and the client that it bound. A preview names those that it would
// connect and bind, and stores no Token.
type addDocument struct {
	DryRun        bool   `json:"dry_run,omitempty"`
	Account       string `json:"account"`
	Route         string `json:"route"`
	Client        string `json:"client"`
	Label         string `json:"label"`
	Model         string `json:"model"`
	UpstreamModel string `json:"upstream_model"`
	TokenStored   bool   `json:"token_stored"`
	invocation.Preview
	NextAction string `json:"next_action,omitempty"`
	NextDetail string `json:"next_detail,omitempty"`
}

// NewCommand constructs the guided command that connects one Account and its first Route.
func NewCommand(runtime invocation.Context) *cobra.Command {
	var label, openAIURL, anthropicURL, client, model string
	var tokenStdin, jsonMode, dryRun bool
	cmd := &cobra.Command{
		Use:   "add <account>",
		Short: "Add and connect one Account with its first Route",
		Args: cobra.MatchAll(cobra.ExactArgs(1), func(cmd *cobra.Command, args []string) error {
			if !domain.ValidIdentifier(args[0]) {
				return fmt.Errorf("Invalid account ID %q; use letters, numbers, dots, hyphens, or underscores; run `%s --help`", args[0], cmd.CommandPath())
			}
			if !runtime.Clients.Admission().Admits(client) || strings.TrimSpace(model) == "" {
				return fmt.Errorf("--for and --upstream-model are required; --for must be %s; run `%s --help`", runtime.Clients.Admission().Usage(), cmd.CommandPath())
			}
			if jsonMode && !tokenStdin && !dryRun {
				return fmt.Errorf("--json reads the Token only from --token-stdin; run `%s --help`", cmd.CommandPath())
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
			next := "aigw check"
			var preview invocation.Preview
			if dryRun {
				if preview, err = previewConnection(runtime, cfg, name, client, account, route); err != nil {
					return fmt.Errorf("%w; run `%s --help`", err, cmd.CommandPath())
				}
				next = invocation.ApplyCommand(cmd, args)
			} else {
				acquireToken := func() (string, error) { return invocation.ReadToken(runtime, tokenStdin, true) }
				if err := runtime.Synchronizer().ConnectAccount(cmd.Context(), cfg, name, client, account, route, acquireToken); err != nil {
					return fmt.Errorf("%w; run `%s --help`", err, cmd.CommandPath())
				}
			}
			if jsonMode {
				document := addDocument{
					DryRun: dryRun, Account: name, Route: name, Client: client, Label: label,
					Model: route.Model, UpstreamModel: route.UpstreamModelID(), TokenStored: !dryRun, Preview: preview,
				}
				document.NextAction, document.NextDetail = terminal.NextSteps(next)
				return terminal.WriteDocument(runtime.Out, schemaVersion, true, document)
			}
			r := invocation.Renderer(runtime)
			if dryRun {
				r.ProductTitle("Account connection preview")
			} else {
				r.ProductTitle("Account connected")
			}
			r.Section("Selection")
			r.Rows(
				terminal.Field{Label: "Account", Value: label},
				terminal.Field{Label: "Account ID", Value: name},
				terminal.Field{Label: "Route ID", Value: name},
				terminal.Field{Label: "Client", Value: runtime.Clients.Admission().Label(client)},
				terminal.Field{Label: "Model", Value: route.Model},
			)
			if dryRun {
				r.Rows(terminal.Field{State: terminal.Info, Label: "Token", Value: "Not read in a preview"})
				invocation.RenderPreview(r, preview, next)
				return nil
			}
			r.Rows(terminal.Field{State: terminal.OK, Label: "Token", Value: "Securely stored"})
			r.Next(next)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the Account, Route and Client Binding that would be added without reading a Token or writing anything")
	cmd.Flags().StringVar(&label, "label", "", "Provider display name")
	cmd.Flags().StringVar(&openAIURL, "openai-url", "", "OpenAI Responses base URL")
	cmd.Flags().StringVar(&anthropicURL, "anthropic-url", "", "Anthropic base URL")
	cmd.Flags().StringVar(&client, "for", "", "Client: "+runtime.Clients.Admission().Usage())
	cmd.Flags().StringVar(&model, "upstream-model", "", "Exact provider model ID")
	cmd.Flags().BoolVar(&tokenStdin, "token-stdin", false, "Read one token line from standard input")
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write the connected Account and Route as JSON")
	return cmd
}

// previewConnection reports what connecting the Account would change beyond
// the configuration, as if its Token were stored, without reading, checking or
// storing it. add always stores the Token, so a backend that stores none
// refuses the preview as it would refuse add.
func previewConnection(runtime invocation.Context, cfg domain.Config, name, client string, account domain.Account, route domain.Route) (invocation.Preview, error) {
	if secret.IsReadOnly(runtime.Secrets) {
		return invocation.Preview{}, errors.New("add cannot store a Token in the read-only environment secret backend")
	}
	after, projection, err := runtime.Synchronizer().PreviewConnection(cfg, name, client, account, route)
	if err != nil {
		return invocation.Preview{}, err
	}
	return invocation.WithEntrypoint(runtime, after, projection)
}
