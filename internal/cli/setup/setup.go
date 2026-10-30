// Package setup implements `aigw setup`, the first-time setup of an Account,
// its Token and its first Route, from flags, a manifest or a guided wizard.
package setup

import (
	clientactivation "aigw-cli/internal/activation"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"

	"aigw-cli/internal/secret"
	"aigw-cli/internal/secret/probe"
	"aigw-cli/internal/terminal"

	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

// Request contains the explicit setup inputs before discovery and validation resolve their effects.
type Request struct {
	From, Account, Route, Label      string
	OpenAIURL, AnthropicURL, ChatURL string
	Client, Model, Protocol          string
	TokenStdin                       bool
	PromptToken                      bool
	JSON                             bool
}

// NewCommand constructs the setup command and binds it to one invocation context.
func NewCommand(runtime invocation.Context) *cobra.Command {
	request := Request{}
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Complete first-time setup",
		Args: cobra.MatchAll(cobra.NoArgs, func(cmd *cobra.Command, _ []string) error {
			request.From = strings.TrimSpace(request.From)
			request.Account = strings.TrimSpace(request.Account)
			if !cmd.Flags().Changed("from") {
				if request.JSON {
					return fmt.Errorf("--json requires --from")
				}
				return nil
			}
			if request.From == "" {
				return fmt.Errorf("--from requires a configuration manifest path")
			}
			if cmd.Flags().Changed("account") && request.Account == "" {
				return fmt.Errorf("--account requires an Account ID from the configuration manifest")
			}
			if request.TokenStdin && request.Account == "" {
				return fmt.Errorf("--token-stdin requires --account so one Token has one unambiguous owner; run `%s --help`", cmd.CommandPath())
			}
			return nil
		}),
		RunE: func(cmd *cobra.Command, _ []string) error {
			operation := runtime
			operation.Secrets = secret.ObserveAvailability(runtime.Secrets)
			if cmd.Flags().Changed("from") {
				return runManifestSetup(cmd.Context(), operation, request)
			}
			if operation.Interactive && request == (Request{}) {
				cfg, err := operation.Config.Load()
				if err != nil {
					return err
				}
				if err := operation.Synchronizer().AdmitSetup(cfg); err != nil {
					return err
				}
				return RunWizard(cmd.Context(), operation)
			}
			return runSetup(cmd.Context(), operation, request)
		},
	}
	cmd.Flags().StringVar(&request.From, "from", "", "Set up all routes from a token-free configuration manifest")
	cmd.Flags().StringVar(&request.Account, "account", "", "Account ID; guided setup uses the first Route ID when omitted, --from connects only this Account")
	cmd.Flags().StringVar(&request.Route, "route", "", "First route ID")
	cmd.Flags().StringVar(&request.Label, "label", "", "Provider display name")
	cmd.Flags().StringVar(&request.OpenAIURL, "openai-url", "", "OpenAI Responses base URL")
	cmd.Flags().StringVar(&request.AnthropicURL, "anthropic-url", "", "Anthropic base URL")
	cmd.Flags().StringVar(&request.ChatURL, "chat-url", "", "OpenAI Chat Completions base URL")
	cmd.Flags().StringVar(&request.Client, "for", "", "Client for the first route: "+runtime.Clients.Admission().Usage())
	cmd.Flags().StringVar(&request.Model, "model", "", "Upstream model ID for --for")
	cmd.Flags().StringVar(&request.Protocol, "protocol", "", "Endpoint protocol when more than one URL is configured")
	cmd.Flags().BoolVar(&request.TokenStdin, "token-stdin", false, "Read one token line from standard input")
	cmd.Flags().BoolVar(&request.JSON, "json", false, "Write the manifest setup result as JSON")
	for _, name := range []string{"route", "label", "openai-url", "anthropic-url", "chat-url", "for", "model", "protocol"} {
		cmd.MarkFlagsMutuallyExclusive("from", name)
	}
	return cmd
}

type setupPlan struct {
	request Request
	before  domain.Config
	config  domain.Config
	route   domain.Route
}

type setupCredential struct {
	account string
	token   string
	write   bool
}

func runSetup(ctx context.Context, runtime invocation.Context, request Request) error {
	cfg, err := runtime.Config.Load()
	if err != nil {
		return err
	}
	if err := runtime.Synchronizer().AdmitSetup(cfg); err != nil {
		return err
	}
	plan, err := planSetup(cfg, request)
	if err != nil {
		return err
	}
	selected, err := plan.config.ResolveRuntime(plan.request.Client, "")
	if err != nil {
		return err
	}
	credentialChange, err := setupToken(runtime, plan.request)
	if err != nil {
		return err
	}
	if err := probe.ValidateRuntime(ctx, runtime.HTTP, runtime.Clients.Admission(), selected, credentialChange.token); err != nil {
		return fmt.Errorf("Token validation failed: %w", err)
	}

	tokens := make(map[string]string)
	if credentialChange.write {
		tokens[credentialChange.account] = credentialChange.token
	}
	plan.config, err = runtime.Synchronizer().Setup(ctx, plan.before, plan.config, tokens, plan.request.Client)
	if err != nil {
		return err
	}
	renderSetupService(runtime, plan)
	renderSetupClients(runtime, plan.config, plan.request.Client)
	return nil
}

func planSetup(cfg domain.Config, request Request) (setupPlan, error) {
	plan := setupPlan{request: request, before: cfg.Clone(), config: cfg}
	plan.request.Route = strings.TrimSpace(plan.request.Route)
	plan.request.Account = strings.TrimSpace(plan.request.Account)
	if plan.request.Route == "" {
		return setupPlan{}, fmt.Errorf("--route is required; import the reviewed team manifest with `aigw setup --from <path>` or run `aigw setup --help`")
	}
	if plan.request.Account == "" {
		plan.request.Account = plan.request.Route
	}
	if !domain.ValidIdentifier(plan.request.Account) {
		return setupPlan{}, fmt.Errorf("Invalid account ID %q; use letters, numbers, dots, hyphens, or underscores", plan.request.Account)
	}
	if !domain.ValidIdentifier(plan.request.Route) {
		return setupPlan{}, fmt.Errorf("Invalid route ID %q; use letters, numbers, dots, hyphens, or underscores", plan.request.Route)
	}
	if plan.request.Label == "" {
		plan.request.Label = plan.request.Account
	}
	endpoints := domain.Endpoints{
		OpenAIResponses:       strings.TrimRight(strings.TrimSpace(plan.request.OpenAIURL), "/"),
		Anthropic:             strings.TrimRight(strings.TrimSpace(plan.request.AnthropicURL), "/"),
		OpenAIChatCompletions: strings.TrimRight(strings.TrimSpace(plan.request.ChatURL), "/"),
	}
	if plan.request.Client == "" {
		return setupPlan{}, fmt.Errorf("--for is required and must be %s; a model route belongs to exactly one client", cfg.Admission().Usage())
	}
	spec, ok := cfg.Admission().Spec(plan.request.Client)
	if !ok {
		return setupPlan{}, fmt.Errorf("--for must be %s; run `aigw setup --help`", cfg.Admission().Usage())
	}
	account := domain.Account{ID: plan.request.Account, Endpoints: endpoints}
	if compatible := spec.CompatibleProtocols(account); plan.request.Protocol == "" && len(compatible) > 1 {
		names := make([]string, 0, len(compatible))
		for _, candidate := range compatible {
			names = append(names, string(candidate))
		}
		refusal := fmt.Errorf("--for %s has multiple compatible endpoints; specify --protocol", plan.request.Client)
		return setupPlan{}, terminal.ProblemError(refusal.Error(), "Compatible protocols: "+strings.Join(names, ", ")+".",
			"No Account, Route or Client Binding was changed.",
			"Rerun the same aigw setup command with --protocol <"+strings.Join(names, "|")+">", refusal)
	}
	_, protocol, err := spec.ResolveEndpoint(account, domain.EndpointProtocol(plan.request.Protocol))
	if err != nil {
		if _, ok := errors.AsType[*domain.RuntimeMissingEndpointError](err); !ok {
			return setupPlan{}, err
		}
		required := spec.EndpointProtocols[0]
		if plan.request.Protocol != "" {
			required = domain.EndpointProtocol(plan.request.Protocol)
		}
		return setupPlan{}, fmt.Errorf("--for %s requires %s", plan.request.Client, setupEndpointFlag(required))
	}
	if strings.TrimSpace(plan.request.Model) == "" {
		return setupPlan{}, fmt.Errorf("--for %s requires --model", plan.request.Client)
	}
	storedAccount := domain.Account{Label: plan.request.Label, Endpoints: endpoints}
	plan.route = domain.Route{
		Label: plan.request.Label, Account: plan.request.Account, Model: strings.TrimSpace(plan.request.Model),
		Interfaces: map[domain.EndpointProtocol][]domain.Capability{protocol: {}},
	}
	plan.config.Accounts[plan.request.Account] = storedAccount
	plan.config.Routes[plan.request.Route] = plan.route
	plan.config.Clients[plan.request.Client] = domain.ClientBinding{Route: plan.request.Route, Enabled: true, Protocol: protocol}
	if err := plan.config.Validate(); err != nil {
		return setupPlan{}, err
	}
	return plan, nil
}

func renderSetupService(runtime invocation.Context, plan setupPlan) {
	r := invocation.Renderer(runtime)
	r.ProductTitle("First-time setup")
	r.Section("Account")
	r.Rows(
		terminal.Field{Label: "Account", Value: plan.request.Account},
		terminal.Field{Label: "Route", Value: plan.request.Route},
		terminal.Field{Label: "Model", Value: plan.route.Model},
		terminal.Field{State: terminal.OK, Label: "API Token", Value: "Validated"},
	)
}

func renderSetupClients(runtime invocation.Context, cfg domain.Config, selectedClient string) {
	r := invocation.Renderer(runtime)
	activation := clientactivation.AssessActivation(cfg, runtime.Secrets)
	r.Section("Clients")
	rows := make([]terminal.Field, 0, len(runtime.Clients.Admission().Specs()))
	for _, spec := range runtime.Clients.Admission().Specs() {
		binding := cfg.Clients[spec.ID]
		switch {
		case !binding.Enabled:
			rows = append(rows, terminal.Field{State: terminal.Info, Label: spec.Label, Value: "Not configured"})
		case activation.ProjectionPrerequisites[spec.ID] != "":
			rows = append(rows, terminal.Field{State: terminal.Info, Label: spec.Label, Value: "Selected; projection deferred"})
		default:
			rows = append(rows, terminal.Field{State: terminal.OK, Label: spec.Label, Value: "Projection configured"})
		}
	}
	r.Rows(rows...)
	if action := activation.ProjectionPrerequisites[selectedClient]; action != "" {
		r.Success("Account and Route are ready; selected client activation is deferred.")
		r.Next(action)
		return
	}
	r.Success("Client projection configured; real-client use is not yet verified.")
	r.Next(activation.NextActionFor(nil))
}

func setupEndpointFlag(protocol domain.EndpointProtocol) string {
	switch protocol {
	case domain.ProtocolAnthropic:
		return "--anthropic-url"
	case domain.ProtocolOpenAIChatCompletions:
		return "--chat-url"
	case domain.ProtocolOpenAIResponses:
		return "--openai-url"
	default:
		return "--protocol"
	}
}

// setupToken prefers a credential that was already supplied by the active
// secret backend. This is essential for non-interactive CI/container use with
// AIGW_SECRET_BACKEND=env: the environment store is intentionally read-only,
// so setup must validate and reference its token rather than asking for a
// second copy and attempting to persist it.
func setupToken(runtime invocation.Context, request Request) (setupCredential, error) {
	credential := setupCredential{account: request.Account}
	previous, err := runtime.Secrets.Get(request.Account)
	if err != nil && !errors.Is(err, secret.ErrNotFound) {
		return setupCredential{}, err
	}
	if !request.PromptToken && !request.TokenStdin && err == nil {
		credential.token = previous
		return credential, nil
	}
	if request.PromptToken {
		credential.token, err = runtime.Prompt.Secret("Paste " + request.Label + " token: ")
	} else {
		credential.token, err = invocation.ReadToken(runtime, request.TokenStdin, true)
	}
	if err != nil {
		return setupCredential{}, err
	}
	credential.write = true
	return credential, nil
}

// RunWizard is deliberately provider-neutral. AIGW never assumes a gateway,
// token slot, URL, or model catalogue for a new user; the user may instead
// import a secret-free configuration manifest before running this flow.
func RunWizard(ctx context.Context, runtime invocation.Context) error {
	account, err := runtime.Prompt.Text("Account ID (for example, team-primary): ")
	if err != nil {
		return err
	}
	if !domain.ValidIdentifier(account) {
		return fmt.Errorf("invalid account ID %q; use letters, numbers, dots, hyphens, or underscores", account)
	}
	label, err := runtime.Prompt.Text("Provider display name: ")
	if err != nil {
		return err
	}
	clientSpecs := runtime.Clients.Admission().Specs()
	clientChoices := make([]terminal.Choice, 0, len(clientSpecs))
	for _, spec := range clientSpecs {
		clientChoices = append(clientChoices, terminal.Choice{Value: spec.ID, Label: spec.Label})
	}
	client, err := runtime.Prompt.Select("Client for the first Route: ", clientChoices)
	if err != nil {
		return err
	}
	spec, ok := runtime.Clients.Admission().Spec(client)
	if !ok || len(spec.EndpointProtocols) == 0 {
		return fmt.Errorf("unsupported setup client %q", client)
	}
	protocol := spec.EndpointProtocols[0]
	if len(spec.EndpointProtocols) > 1 {
		protocolChoices := make([]terminal.Choice, 0, len(spec.EndpointProtocols))
		for _, candidate := range spec.EndpointProtocols {
			protocolChoices = append(protocolChoices, terminal.Choice{Value: string(candidate), Label: string(candidate)})
		}
		selected, selectErr := runtime.Prompt.Select("Endpoint protocol: ", protocolChoices)
		if selectErr != nil {
			return selectErr
		}
		protocol = domain.EndpointProtocol(selected)
		if !slices.Contains(spec.EndpointProtocols, protocol) {
			return fmt.Errorf("unsupported %s endpoint protocol %q", spec.Label, selected)
		}
	}
	request := Request{Account: account, Label: label, Client: client, Protocol: string(protocol), PromptToken: true}
	var endpointLabel string
	var endpointURL *string
	switch protocol {
	case domain.ProtocolAnthropic:
		endpointLabel, endpointURL = "Anthropic Messages URL: ", &request.AnthropicURL
	case domain.ProtocolOpenAIResponses:
		endpointLabel, endpointURL = "OpenAI Responses URL: ", &request.OpenAIURL
	case domain.ProtocolOpenAIChatCompletions:
		endpointLabel, endpointURL = "OpenAI Chat Completions URL: ", &request.ChatURL
	default:
		return fmt.Errorf("unsupported endpoint protocol %q", protocol)
	}
	*endpointURL, err = runtime.Prompt.Text(endpointLabel)
	if err != nil {
		return err
	}
	request.Route, err = runtime.Prompt.Text("Route ID (for example, team-model): ")
	if err != nil {
		return err
	}
	request.Model, err = runtime.Prompt.Text("Upstream model ID: ")
	if err != nil {
		return err
	}
	return runSetup(ctx, runtime, request)
}
