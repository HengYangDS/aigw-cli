// Package setup implements `aigw setup`, the first-time setup of an Account,
// its Token and its first Route, from flags, a manifest or a guided wizard.
package setup

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	clientactivation "aigw-cli/internal/app/activation"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/secret/probe"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// setupRequest contains the explicit setup inputs before discovery and validation resolve their effects.
type setupRequest struct {
	From, Provider, Model, Label     string
	OpenAIURL, AnthropicURL, ChatURL string
	Client, UpstreamModel            string
	Protocol                         domain.EndpointProtocol
	TokenStdin                       bool
	PromptToken                      bool
	JSON                             bool
	// DryRun previews setup; apply is the command that applies the preview.
	DryRun bool
	apply  string
}

// NewCommand constructs the setup command and binds it to one invocation context.
func NewCommand(runtime invocation.Context) *cobra.Command {
	request := setupRequest{}
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Complete first-time setup",
		Args: cobra.MatchAll(cobra.NoArgs, func(cmd *cobra.Command, _ []string) error {
			request.From = strings.TrimSpace(request.From)
			request.Provider = strings.TrimSpace(request.Provider)
			if !cmd.Flags().Changed("from") {
				// Setup with --json never prompts, so the guided flow needs its
				// first Route as a flag, before anything is read or created.
				if request.JSON && strings.TrimSpace(request.Model) == "" {
					return fmt.Errorf("--json requires --route or --from; run `%s --help`", cmd.CommandPath())
				}
				return nil
			}
			if request.From == "" {
				return fmt.Errorf("--from requires a configuration manifest path")
			}
			if cmd.Flags().Changed("account") && request.Provider == "" {
				return fmt.Errorf("--account requires an Account ID from the configuration manifest")
			}
			if request.TokenStdin && request.Provider == "" {
				return fmt.Errorf("--token-stdin requires --account so one Token has one unambiguous owner; run `%s --help`", cmd.CommandPath())
			}
			return nil
		}),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if request.DryRun {
				request.apply = invocation.ApplyCommand(cmd, nil)
			}
			operation := runtime
			operation.Secrets = secret.ObserveAvailability(runtime.Secrets)
			// A JSON caller reads one document, so setup never prompts for it.
			operation.Interactive = operation.Interactive && !request.JSON
			if cmd.Flags().Changed("from") {
				return invocation.CancelledPrompt(cmd, nil, runManifestSetup(cmd.Context(), operation, request))
			}
			if operation.Interactive && request == (setupRequest{}) {
				cfg, err := operation.Config.Load()
				if err != nil {
					return err
				}
				if err := operation.Synchronizer().AdmitSetup(cfg); err != nil {
					return err
				}
				return invocation.CancelledPrompt(cmd, nil, runWizard(cmd.Context(), operation))
			}
			return invocation.CancelledPrompt(cmd, nil, runSetup(cmd.Context(), operation, request))
		},
	}
	cmd.Flags().StringVar(&request.From, "from", "", "Set up all routes from a token-free configuration manifest")
	cmd.Flags().StringVar(&request.Provider, "account", "", "Account ID; guided setup uses the first Route ID when omitted, --from connects only this Account")
	cmd.Flags().StringVar(&request.Model, "route", "", "First route ID")
	cmd.Flags().StringVar(&request.Label, "label", "", "Provider display name")
	cmd.Flags().StringVar(&request.OpenAIURL, "openai-url", "", "OpenAI Responses base URL")
	cmd.Flags().StringVar(&request.AnthropicURL, "anthropic-url", "", "Anthropic base URL")
	cmd.Flags().StringVar(&request.ChatURL, "chat-url", "", "OpenAI Chat Completions base URL")
	cmd.Flags().StringVar(&request.Client, "for", "", "Client for the first route: "+runtime.Clients.Admission().Usage())
	cmd.Flags().StringVar(&request.UpstreamModel, "upstream-model", "", "Exact provider model ID for --for")
	invocation.BindProtocol(cmd, &request.Protocol, "Endpoint protocol when more than one URL is configured")
	cmd.Flags().BoolVar(&request.TokenStdin, "token-stdin", false, "Read one token line from standard input")
	cmd.Flags().BoolVar(&request.JSON, "json", false, "Write the setup result as JSON")
	cmd.Flags().BoolVar(&request.DryRun, "dry-run", false, "Show the Accounts, Routes and Client Bindings that setup would create without reading a Token or writing anything")
	for _, name := range []string{"route", "label", "openai-url", "anthropic-url", "chat-url", "for", "upstream-model", "protocol"} {
		cmd.MarkFlagsMutuallyExclusive("from", name)
	}
	return cmd
}

type setupPlan struct {
	request setupRequest
	before  domain.Config
	config  domain.Config
	model   domain.Model
}

type setupCredential struct {
	provider string
	token    string
	write    bool
}

func runSetup(ctx context.Context, runtime invocation.Context, request setupRequest) error {
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
	if request.DryRun {
		if request.TokenStdin && secret.IsReadOnly(runtime.Secrets) {
			return fmt.Errorf("--token-stdin cannot replace a Token in the read-only environment secret backend")
		}
		return writeSetupPreview(runtime, plan)
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
		tokens[credentialChange.provider] = credentialChange.token
	}
	plan.config, err = runtime.Synchronizer().Setup(ctx, plan.before, plan.config, tokens, plan.request.Client)
	if err != nil {
		return err
	}
	if request.JSON {
		return writeSetupDocument(runtime, buildFlagSetupResult(runtime, plan))
	}
	renderSetupService(runtime, plan)
	renderSetupClients(runtime, plan.config, plan.request.Client)
	return nil
}

func planSetup(cfg domain.Config, request setupRequest) (setupPlan, error) {
	plan := setupPlan{request: request, before: cfg.Clone(), config: cfg}
	plan.request.Model = strings.TrimSpace(plan.request.Model)
	plan.request.Provider = strings.TrimSpace(plan.request.Provider)
	if plan.request.Model == "" {
		return setupPlan{}, fmt.Errorf("--route is required; import the reviewed team manifest with `aigw setup --from <path>` or run `aigw setup --help`")
	}
	if plan.request.Provider == "" {
		plan.request.Provider = plan.request.Model
	}
	if !domain.ValidIdentifier(plan.request.Provider) {
		return setupPlan{}, fmt.Errorf("Invalid account ID %q; use letters, numbers, dots, hyphens, or underscores", plan.request.Provider)
	}
	if !domain.ValidIdentifier(plan.request.Model) {
		return setupPlan{}, fmt.Errorf("Invalid route ID %q; use letters, numbers, dots, hyphens, or underscores", plan.request.Model)
	}
	if plan.request.Label == "" {
		plan.request.Label = plan.request.Provider
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
	account := domain.Provider{ID: plan.request.Provider, Endpoints: endpoints}
	if compatible := spec.CompatibleProtocols(account); plan.request.Protocol == "" && len(compatible) > 1 {
		names := make([]string, 0, len(compatible))
		for _, candidate := range compatible {
			names = append(names, candidate.Name())
		}
		refusal := fmt.Errorf("--for %s has multiple compatible endpoints; specify --protocol", plan.request.Client)
		return setupPlan{}, terminal.ProblemError(refusal.Error(), "Compatible protocols: "+strings.Join(names, ", ")+".",
			"No Account, Route or Client Binding was changed.",
			"Rerun the same aigw setup command with --protocol <"+strings.Join(names, "|")+">", refusal)
	}
	_, protocol, err := spec.ResolveEndpoint(account, plan.request.Protocol)
	if err != nil {
		if _, ok := errors.AsType[*domain.RuntimeMissingEndpointError](err); !ok {
			return setupPlan{}, err
		}
		required := spec.EndpointProtocols[0]
		if plan.request.Protocol != "" {
			required = plan.request.Protocol
		}
		return setupPlan{}, fmt.Errorf("--for %s requires %s", plan.request.Client, setupEndpointFlag(required))
	}
	if strings.TrimSpace(plan.request.UpstreamModel) == "" {
		return setupPlan{}, fmt.Errorf("--for %s requires --upstream-model", plan.request.Client)
	}
	storedAccount := domain.Provider{Label: plan.request.Label, Endpoints: endpoints}
	plan.model = domain.Model{
		Label: plan.request.Label, Provider: plan.request.Provider, Family: strings.TrimSpace(plan.request.UpstreamModel),
		Protocols: map[domain.EndpointProtocol][]domain.Capability{protocol: {}},
	}
	plan.config.Providers[plan.request.Provider] = storedAccount
	plan.config.Models[plan.request.Model] = plan.model
	plan.config.Clients[plan.request.Client] = domain.ClientBinding{Model: plan.request.Model, Enabled: true, Protocol: protocol}
	if err := plan.config.Validate(); err != nil {
		return setupPlan{}, err
	}
	return plan, nil
}

// writeSetupPreview reports the Account, Route and Client Binding that setup
// would create. A preview reads, checks and stores no Token, so it connects no
// Account and projects no client.
func writeSetupPreview(runtime invocation.Context, plan setupPlan) error {
	after, projection, err := runtime.Synchronizer().PreviewSetup(plan.before, plan.config, []string{plan.request.Provider}, plan.request.Client)
	if err != nil {
		return err
	}
	preview, err := invocation.WithEntrypoint(runtime, after, projection)
	if err != nil {
		return err
	}
	if plan.request.JSON {
		result := manifestSetupResult{
			DryRun:             true,
			Imported:           manifestSetupImported{Providers: []string{plan.request.Provider}, Models: []string{plan.request.Model}},
			ConnectedProviders: []string{},
			Selections:         map[string]string{plan.request.Client: plan.request.Model},
			ProjectedClients:   []string{},
			OnlineVerification: "not_checked",
			Preview:            preview,
		}
		result.NextAction, result.NextDetail = terminal.NextSteps(plan.request.apply)
		return terminal.WriteDocument(runtime.Out, schemaVersion, true, result)
	}
	r := invocation.Renderer(runtime)
	r.ProductTitle("First-time setup preview")
	r.Section("Account")
	r.Rows(
		terminal.Field{Label: "Account", Value: plan.request.Provider},
		terminal.Field{Label: "Route", Value: plan.request.Model},
		terminal.Field{Label: "Model", Value: plan.model.Family},
		terminal.Field{State: terminal.Info, Label: "API Token", Value: "Not read in a preview"},
	)
	r.Section("Client Bindings")
	r.Rows(terminal.Field{Label: runtime.Clients.Admission().Label(plan.request.Client), Value: plan.request.Model})
	invocation.RenderPreview(r, preview, plan.request.apply)
	return nil
}

func renderSetupService(runtime invocation.Context, plan setupPlan) {
	r := invocation.Renderer(runtime)
	r.ProductTitle("First-time setup")
	r.Section("Account")
	r.Rows(
		terminal.Field{Label: "Account", Value: plan.request.Provider},
		terminal.Field{Label: "Route", Value: plan.request.Model},
		terminal.Field{Label: "Model", Value: plan.model.Family},
		terminal.Field{State: terminal.OK, Label: "API Token", Value: "Validated"},
	)
}

// buildFlagSetupResult reports flag-driven setup in the document that setup
// --from writes: the one Account and Route that it created and connected, the
// client that it selected, and the same next step as the human output.
func buildFlagSetupResult(runtime invocation.Context, plan setupPlan) manifestSetupResult {
	activation := clientactivation.AssessActivation(plan.config, runtime.Secrets)
	result := manifestSetupResult{
		Imported:           manifestSetupImported{Providers: []string{plan.request.Provider}, Models: []string{plan.request.Model}},
		ConnectedProviders: []string{plan.request.Provider},
		Selections:         make(map[string]string, len(plan.config.Clients)),
		ProjectedClients:   []string{},
		OnlineVerification: "not_checked",
		NextAction:         activation.NextActionFor(nil),
	}
	for client, binding := range plan.config.Clients {
		if binding.Model != "" {
			result.Selections[client] = binding.Model
		}
	}
	if action := activation.ProjectionPrerequisites[plan.request.Client]; action != "" {
		result.DeferredActions, result.NextAction = []string{action}, action
	} else {
		result.ProjectedClients = append(result.ProjectedClients, plan.request.Client)
	}
	return result
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
func setupToken(runtime invocation.Context, request setupRequest) (setupCredential, error) {
	credential := setupCredential{provider: request.Provider}
	previous, err := runtime.Secrets.Get(request.Provider)
	if err != nil && !errors.Is(err, secret.ErrNotFound) {
		return setupCredential{}, err
	}
	if !request.PromptToken && !request.TokenStdin && err == nil {
		credential.token = previous
		return credential, nil
	}
	if request.JSON && !request.TokenStdin {
		return setupCredential{}, fmt.Errorf("--json reads the Token only from --token-stdin; run `aigw setup --help`")
	}
	if request.PromptToken {
		credential.token, err = runtime.Prompt.Secret("Paste " + request.Label + " token: ")
		err = invocation.Prompted("The Token prompt for provider "+request.Provider, err)
	} else {
		credential.token, err = invocation.ReadToken(runtime, request.TokenStdin, true)
	}
	if err != nil {
		return setupCredential{}, err
	}
	credential.write = true
	return credential, nil
}

// runWizard is deliberately provider-neutral. AIGW never assumes a gateway,
// token slot, URL, or model catalog for a new user; the user may instead
// import a secret-free configuration manifest before running this flow. Each
// prompt names itself when it fails, so that a caller can say which one ended
// without an answer: all of them come before anything is written.
func runWizard(ctx context.Context, runtime invocation.Context) error {
	account, err := runtime.Prompt.Text("Account ID (for example, team-primary): ")
	if err != nil {
		return invocation.Prompted("The provider ID prompt", err)
	}
	if !domain.ValidIdentifier(account) {
		return fmt.Errorf("invalid account ID %q; use letters, numbers, dots, hyphens, or underscores", account)
	}
	label, err := runtime.Prompt.Text("Provider display name: ")
	if err != nil {
		return invocation.Prompted("The provider name prompt", err)
	}
	clientSpecs := runtime.Clients.Admission().Specs()
	clientChoices := make([]terminal.Choice, 0, len(clientSpecs))
	for _, spec := range clientSpecs {
		clientChoices = append(clientChoices, terminal.Choice{Value: spec.ID, Label: spec.Label})
	}
	client, err := runtime.Prompt.Select("Client for the first Route: ", clientChoices)
	if err != nil {
		return invocation.Prompted("The client prompt", err)
	}
	spec, ok := runtime.Clients.Admission().Spec(client)
	if !ok || len(spec.EndpointProtocols) == 0 {
		return fmt.Errorf("unsupported setup client %q", client)
	}
	protocol := spec.EndpointProtocols[0]
	if len(spec.EndpointProtocols) > 1 {
		protocolChoices := make([]terminal.Choice, 0, len(spec.EndpointProtocols))
		for _, candidate := range spec.EndpointProtocols {
			protocolChoices = append(protocolChoices, terminal.Choice{Value: string(candidate), Label: candidate.Name()})
		}
		selected, selectErr := runtime.Prompt.Select("Endpoint protocol: ", protocolChoices)
		if selectErr != nil {
			return invocation.Prompted("The protocol prompt", selectErr)
		}
		protocol = domain.EndpointProtocol(selected)
		if !slices.Contains(spec.EndpointProtocols, protocol) {
			return fmt.Errorf("unsupported %s endpoint protocol %q", spec.Label, selected)
		}
	}
	request := setupRequest{Provider: account, Label: label, Client: client, Protocol: protocol, PromptToken: true}
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
		return invocation.Prompted("The endpoint prompt", err)
	}
	request.Model, err = runtime.Prompt.Text("Route ID (for example, team-model): ")
	if err != nil {
		return invocation.Prompted("The model name prompt", err)
	}
	request.UpstreamModel, err = runtime.Prompt.Text("Upstream model ID: ")
	if err != nil {
		return invocation.Prompted("The model ID prompt", err)
	}
	return runSetup(ctx, runtime, request)
}
