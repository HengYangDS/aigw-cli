// Package setup implements `aigw setup`: guided setup of a provider, a
// client and a model in a terminal, and team setup, which applies a team
// file. Until the command tree's last step it also sets up a first provider
// from flags.
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
	// Providers are the providers whose Tokens team setup reads, and Prune
	// removes the unused local-only models and families.
	Providers   []string
	Prune       bool
	TokenStdin  bool
	PromptToken bool
	JSON        bool
	// DryRun previews setup; apply is the command that applies the preview.
	DryRun bool
	apply  string
}

// NewCommand constructs the setup command and binds it to one invocation context.
func NewCommand(runtime invocation.Context) *cobra.Command {
	request := setupRequest{}
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Set up a provider, a client and a model, or apply a team file",
		Long: "Without --from, ask in a terminal for a provider, an installed client and a model from the\n" +
			"provider's catalog, save them, and check the client; it can run again. With --from, apply a\n" +
			"team file: its models, families and recommendations win, and an existing provider keeps its\n" +
			"endpoints and Token.",
		Example: "aigw setup\n" +
			"aigw setup --from team.toml --dry-run\n" +
			"aigw setup --from team.toml --provider dmxapi --token-stdin",
		Args: cobra.MatchAll(cobra.NoArgs, func(cmd *cobra.Command, _ []string) error {
			request.From = strings.TrimSpace(request.From)
			request.Provider = strings.TrimSpace(request.Provider)
			if cmd.Flags().Changed("from") {
				return admitTeamRequest(cmd, runtime, request)
			}
			if len(request.Providers) > 0 || request.Prune {
				return fmt.Errorf("--provider and --prune apply a team file, so they need --from; run `%s --help`", cmd.CommandPath())
			}
			if !flagDriven(cmd) {
				return admitGuided(runtime, request)
			}
			// Setup with --json never prompts, so the flags need the first
			// Route, before anything is read or created.
			if request.JSON && strings.TrimSpace(request.Model) == "" {
				return fmt.Errorf("--json requires --route or --from; run `%s --help`", cmd.CommandPath())
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
			switch {
			case cmd.Flags().Changed("from"):
				return invocation.CancelledPrompt(cmd, nil, runTeamSetup(cmd, operation, request))
			case !flagDriven(cmd):
				return invocation.CancelledPrompt(cmd, nil, runGuided(cmd, operation))
			}
			return invocation.CancelledPrompt(cmd, nil, runSetup(cmd.Context(), operation, request))
		},
	}
	cmd.Flags().StringVar(&request.From, "from", "", "Apply a team file: its providers, models, families and recommendations")
	cmd.Flags().StringArrayVar(&request.Providers, "provider", nil, "With --from, a provider whose Token to read, at a prompt or from --token-stdin; repeatable")
	cmd.Flags().BoolVar(&request.Prune, "prune", false, "With --from, remove the local models that no client selects and no recommendation names, and unused families")
	cmd.Flags().StringVar(&request.Provider, "account", "", "Account ID; guided setup uses the first Route ID when omitted")
	cmd.Flags().StringVar(&request.Model, "route", "", "First route ID")
	cmd.Flags().StringVar(&request.Label, "label", "", "Provider display name")
	cmd.Flags().StringVar(&request.OpenAIURL, "openai-url", "", "OpenAI Responses base URL")
	cmd.Flags().StringVar(&request.AnthropicURL, "anthropic-url", "", "Anthropic base URL")
	cmd.Flags().StringVar(&request.ChatURL, "chat-url", "", "OpenAI Chat Completions base URL")
	cmd.Flags().StringVar(&request.Client, "for", "", "Client for the first route: "+runtime.Clients.Admission().Usage())
	cmd.Flags().StringVar(&request.UpstreamModel, "upstream-model", "", "Exact provider model ID for --for")
	invocation.BindProtocol(cmd, &request.Protocol, "Endpoint protocol when more than one URL is configured")
	cmd.Flags().BoolVar(&request.TokenStdin, "token-stdin", false, "Read one Token from standard input; with --from, the one --provider's")
	cmd.Flags().BoolVar(&request.JSON, "json", false, "Write the setup result as JSON")
	cmd.Flags().BoolVar(&request.DryRun, "dry-run", false, "Show what setup would change without reading a Token or writing anything")
	for _, name := range []string{"account", "route", "label", "openai-url", "anthropic-url", "chat-url", "for", "upstream-model", "protocol"} {
		cmd.MarkFlagsMutuallyExclusive("from", name)
	}
	invocation.CompleteFlag(cmd, "provider", cobra.NoFileCompletions)
	return cmd
}

// flagDrivenFlags are the flags of setup from flags, which the command tree's
// last step removes.
var flagDrivenFlags = []string{"account", "route", "label", "openai-url", "anthropic-url", "chat-url", "for", "upstream-model", "protocol", "token-stdin"}

// flagDriven reports whether cmd sets up a provider from flags.
func flagDriven(cmd *cobra.Command) bool {
	return slices.ContainsFunc(flagDrivenFlags, cmd.Flags().Changed)
}

// admitGuided refuses guided setup where it cannot ask: it is the one write
// without a preview, it prompts, which --json never does, and it needs a
// terminal. A team file needs none of them, so each refusal names it.
func admitGuided(runtime invocation.Context, request setupRequest) error {
	switch {
	case request.DryRun:
		return errors.New("guided setup is the one write without a preview; preview a team file with `aigw setup --from <file> --dry-run`")
	case request.JSON:
		return errors.New("guided setup asks questions, and --json never prompts; apply a team file with `aigw setup --from <file> --json`")
	case !runtime.Interactive:
		return errors.New("guided setup asks questions in a terminal; without one, apply a team file with `aigw setup --from <file>`")
	}
	return nil
}

// admitTeamRequest refuses, before the team file or standard input is read, a
// team setup whose Token input cannot run: a blank path, an invalid provider
// ID, standard input without exactly one provider or with the read-only
// environment backend, and a named provider whose Token would need a prompt
// where none may appear.
func admitTeamRequest(cmd *cobra.Command, runtime invocation.Context, request setupRequest) error {
	path := cmd.CommandPath()
	if request.From == "" {
		return fmt.Errorf("--from requires a team file path; run `%s --help`", path)
	}
	for _, id := range request.Providers {
		if !domain.ValidIdentifier(id) || id != strings.ToLower(id) {
			return fmt.Errorf("--provider %q is not a provider ID; use lowercase letters, numbers, dots, hyphens or underscores; run `%s --help`", id, path)
		}
	}
	readOnly := secret.IsReadOnly(runtime.Secrets)
	switch {
	case request.TokenStdin && len(request.Providers) != 1:
		return fmt.Errorf("--token-stdin reads one Token, so it needs exactly one --provider; run `%s --help`", path)
	case request.TokenStdin && readOnly:
		return readOnlyTokenInput(request.Providers[0], invocation.Rerun(cmd, nil, []string{"token-stdin"}))
	case len(request.Providers) > 0 && !request.TokenStdin && !request.DryRun && !readOnly && (!runtime.Interactive || request.JSON):
		reason := "a named provider's Token is read from --token-stdin without a terminal or with --json"
		if len(request.Providers) == 1 {
			if line := invocation.Rerun(cmd, nil, nil, "--token-stdin"); line != "" {
				return fmt.Errorf("%s; pipe the Token to `%s`", reason, line)
			}
		}
		return fmt.Errorf("%s, one provider at a time; run `%s --help`", reason, path)
	}
	return nil
}

// flagSchemaVersion is the version of flag-driven setup's JSON document, which
// the command tree's last step removes with flag-driven setup.
const flagSchemaVersion = 2

type manifestSetupImported struct {
	Providers []string `json:"accounts"`
	Models    []string `json:"routes"`
}

// manifestSetupResult is flag-driven setup's JSON document.
type manifestSetupResult struct {
	DryRun             bool                  `json:"dry_run,omitempty"`
	Imported           manifestSetupImported `json:"imported"`
	ConnectedProviders []string              `json:"connected_accounts"`
	Selections         map[string]string     `json:"selections"`
	ProjectedClients   []string              `json:"projected_clients"`
	OnlineVerification string                `json:"online_verification"`
	DeferredActions    []string              `json:"deferred_actions,omitempty"`
	// Preview is what a preview's setup would change beyond the configuration.
	invocation.Preview
	NextAction string `json:"next_action,omitempty"`
	NextDetail string `json:"next_detail,omitempty"`
}

// writeSetupDocument writes an applied setup's document, whose next step is
// a command that a script can run as it stands, or guidance.
func writeSetupDocument(runtime invocation.Context, result manifestSetupResult) error {
	result.NextAction, result.NextDetail = terminal.NextSteps(result.NextAction)
	return terminal.WriteDocument(runtime.Out, flagSchemaVersion, true, result)
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
		return terminal.WriteDocument(runtime.Out, flagSchemaVersion, true, result)
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
