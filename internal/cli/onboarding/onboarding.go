// Package onboarding owns first-time configuration setup and its interactive
// command projection.
package onboarding

import (
	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/credential"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/prompt"
	"aigw-cli/internal/secrets"
	"context"
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"strings"
)

type Request struct {
	From, Account, Profile, Label string
	OpenAIURL, AnthropicURL       string
	Client, Model                 string
	TokenStdin                    bool
	PromptToken                   bool
}

func NewCommand(runtime invocation.Context) *cobra.Command {
	request := Request{}
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Complete first-time setup",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			request.From = strings.TrimSpace(request.From)
			if cmd.Flags().Changed("from") {
				if request.From == "" {
					return fmt.Errorf("--from requires a configuration manifest path")
				}
				for _, name := range []string{"account", "profile", "label", "openai-url", "anthropic-url", "for", "model"} {
					if !cmd.Flags().Changed(name) {
						continue
					}
					return fmt.Errorf("--from cannot be combined with --account, --profile, --label, --openai-url, --anthropic-url, --for, or --model")
				}
				return runManifestSetup(cmd.Context(), runtime, request)
			}
			if runtime.Interactive && request.Account == "" && request.Profile == "" && request.Label == "" && request.OpenAIURL == "" && request.AnthropicURL == "" && request.Client == "" && request.Model == "" && !request.TokenStdin {
				cfg, err := runtime.Config.Load()
				if err != nil {
					return err
				}
				if len(cfg.Profiles) > 0 {
					return fmt.Errorf("AIGW is already configured; run `aigw add` to add an account, `aigw profile add` to add a model profile, or `aigw status` to inspect current state")
				}
				return RunWizard(cmd.Context(), runtime)
			}
			return runSetup(cmd.Context(), runtime, request)
		},
	}
	cmd.Flags().StringVar(&request.From, "from", "", "Set up all profiles from a token-free configuration manifest")
	cmd.Flags().StringVar(&request.Account, "account", "", "Account ID; defaults to --profile")
	cmd.Flags().StringVar(&request.Profile, "profile", "", "First profile ID")
	cmd.Flags().StringVar(&request.Label, "label", "", "Provider display name")
	cmd.Flags().StringVar(&request.OpenAIURL, "openai-url", "", "OpenAI Responses base URL")
	cmd.Flags().StringVar(&request.AnthropicURL, "anthropic-url", "", "Anthropic base URL")
	cmd.Flags().StringVar(&request.Client, "for", "", "Client for the first profile: "+configuration.AdmittedClientUsage())
	cmd.Flags().StringVar(&request.Model, "model", "", "Upstream model ID for --for")
	cmd.Flags().BoolVar(&request.TokenStdin, "token-stdin", false, "Read one token line from standard input")
	return cmd
}

type setupPlan struct {
	request           Request
	before            configuration.Config
	config            configuration.Config
	account           configuration.Account
	profile           configuration.Profile
	validationClients []string
}

func runSetup(ctx context.Context, runtime invocation.Context, request Request) error {
	cfg, err := runtime.Config.Load()
	if err != nil {
		return err
	}
	plan, err := planSetup(cfg, request)
	if err != nil {
		return err
	}
	token, secretAlreadyManaged, err := setupToken(runtime, plan.request)
	if err != nil {
		return err
	}
	if err := credential.Validate(ctx, runtime.HTTP, plan.account, token, plan.validationClients...); err != nil {
		return fmt.Errorf("Token validation failed: %w", err)
	}

	discovered, err := invocation.Discover(runtime)
	if err != nil {
		return err
	}
	discoveredClaude := discovered.Executable(configuration.ClientClaude)
	discoveredCodex := discovered.Executable(configuration.ClientCodex)
	discoveredTargets := discovered.AutoManagedCodexTargets()
	if _, _, resolveErr := plan.config.ResolveRuntime(configuration.ClientClaude, ""); resolveErr == nil && discoveredClaude != "" {
		plan.config.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: discoveredClaude}
	}
	if _, _, resolveErr := plan.config.ResolveRuntime(configuration.ClientCodex, ""); resolveErr == nil && discoveredCodex != "" && len(discoveredTargets) > 0 {
		plan.config.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: discoveredCodex, Targets: discoveredTargets}
	}

	renderSetupService(runtime, plan)
	if !secretAlreadyManaged {
		if err := runtime.Secrets.Set(plan.request.Account, token); err != nil {
			return err
		}
	}
	if err := invocation.Synchronizer(runtime).Commit(ctx, plan.before, plan.config, "setup"); err != nil {
		rollbackSetup(runtime, plan.request.Account, !secretAlreadyManaged)
		return fmt.Errorf("Client configuration failed and was rolled back: %w", err)
	}
	renderSetupClients(runtime, plan.config)
	return nil
}

func planSetup(cfg configuration.Config, request Request) (setupPlan, error) {
	if len(cfg.Profiles) > 0 {
		return setupPlan{}, fmt.Errorf("AIGW is already configured; run `aigw add` to add an account, `aigw profile add` to add a model profile, or `aigw status` to inspect current state")
	}
	plan := setupPlan{request: request, before: cfg.Clone(), config: cfg}
	plan.request.Profile = strings.TrimSpace(plan.request.Profile)
	plan.request.Account = strings.TrimSpace(plan.request.Account)
	if plan.request.Profile == "" {
		return setupPlan{}, fmt.Errorf("--profile is required; for example: `aigw setup --account team-gateway --profile gpt-5.6 --for codex --model gpt-5.6 --openai-url https://gateway.example/v1`")
	}
	if plan.request.Account == "" {
		plan.request.Account = plan.request.Profile
	}
	if !configuration.ValidProfileName(plan.request.Account) {
		return setupPlan{}, fmt.Errorf("Invalid account ID %q; use letters, numbers, dots, hyphens, or underscores", plan.request.Account)
	}
	if !configuration.ValidProfileName(plan.request.Profile) {
		return setupPlan{}, fmt.Errorf("Invalid profile ID %q; use letters, numbers, dots, hyphens, or underscores", plan.request.Profile)
	}
	if plan.request.Label == "" {
		plan.request.Label = plan.request.Account
	}
	endpoints := configuration.Endpoints{
		OpenAIResponses: strings.TrimRight(strings.TrimSpace(plan.request.OpenAIURL), "/"),
		Anthropic:       strings.TrimRight(strings.TrimSpace(plan.request.AnthropicURL), "/"),
	}
	models := configuration.Models{}
	if plan.request.Client == "" {
		if plan.request.Model != "" {
			return setupPlan{}, fmt.Errorf("--model requires --for %s", configuration.AdmittedClientUsage())
		}
	} else {
		spec, ok := configuration.ClientSpecFor(plan.request.Client)
		if !ok {
			return setupPlan{}, fmt.Errorf("--for must be %s; run `aigw setup --help`", configuration.AdmittedClientUsage())
		}
		account := configuration.Account{ID: plan.request.Account, Endpoints: endpoints}
		if _, err := spec.Endpoint(account); err != nil {
			var missing *configuration.RuntimeMissingEndpointError
			if !errors.As(err, &missing) {
				return setupPlan{}, err
			}
			return setupPlan{}, fmt.Errorf("--for %s requires %s", plan.request.Client, setupEndpointFlag(spec.EndpointProtocol))
		}
		if strings.TrimSpace(plan.request.Model) == "" {
			return setupPlan{}, fmt.Errorf("--for %s requires --model", plan.request.Client)
		}
		models[plan.request.Client] = strings.TrimSpace(plan.request.Model)
		plan.validationClients = append(plan.validationClients, plan.request.Client)
	}
	storedAccount := configuration.Account{Label: plan.request.Label, Endpoints: endpoints}
	plan.account = storedAccount
	plan.account.ID = plan.request.Account
	plan.profile = configuration.Profile{Label: plan.request.Label, Account: plan.request.Account, Client: plan.request.Client, Models: models}
	plan.config.Accounts[plan.request.Account] = storedAccount
	plan.config.Profiles[plan.request.Profile] = plan.profile
	plan.config.Routes.Default = plan.request.Profile
	if err := plan.config.Validate(); err != nil {
		return setupPlan{}, err
	}
	return plan, nil
}

func renderSetupService(runtime invocation.Context, plan setupPlan) {
	r := invocation.Renderer(runtime)
	r.ProductTitle("First-time setup")
	r.Section("Service")
	r.Row("Account", plan.request.Account)
	r.Row("Profile", plan.request.Profile)
	r.Row("model", plan.profile.ModelFor(plan.request.Client))
	r.Status(presentation.OK, "API Token", "Validated")
}

func renderSetupClients(runtime invocation.Context, cfg configuration.Config) {
	r := invocation.Renderer(runtime)
	r.Section("Clients")
	for _, client := range configuration.AdmittedClientIDs() {
		if cfg.Adapters[client].Enabled {
			r.Status(presentation.OK, invocation.Title(client), "Configured")
		} else {
			r.Status(presentation.Info, invocation.Title(client), "Not configured")
		}
	}
	r.Success("Ready. You can add more model profiles for this account.")
	r.Next("aigw check")
}

func setupEndpointFlag(protocol configuration.EndpointProtocol) string {
	if protocol == configuration.ProtocolAnthropic {
		return "--anthropic-url"
	}
	return "--openai-url"
}

// setupToken prefers a credential that was already supplied by the active
// secret backend. This is essential for non-interactive CI/container use with
// AIGW_SECRET_BACKEND=env: the environment store is intentionally read-only,
// so setup must validate and reference its token rather than asking for a
// second copy and attempting to persist it.
func setupToken(runtime invocation.Context, request Request) (token string, alreadyManaged bool, err error) {
	if !request.PromptToken && !request.TokenStdin {
		token, err = runtime.Secrets.Get(request.Account)
		if err == nil {
			return token, true, nil
		}
		if !errors.Is(err, secrets.ErrNotFound) {
			return "", false, err
		}
	}
	if request.PromptToken {
		token, err = runtime.Prompt.Secret("Paste " + request.Label + " token: ")
		return token, false, err
	}
	token, err = invocation.ReadToken(runtime, request.TokenStdin, true)
	return token, false, err
}

func rollbackSetup(runtime invocation.Context, account string, deleteNewSecret bool) {
	if deleteNewSecret {
		_ = runtime.Secrets.Delete(account)
	}
	_ = os.Remove(runtime.Config.Path())
	_ = os.Remove(runtime.Config.Path() + ".bak")
}

// RunWizard is deliberately provider-neutral. AIGW never assumes a gateway,
// token slot, URL, or model catalogue for a new user; the user may instead
// import a secret-free configuration manifest before running this flow.
func RunWizard(ctx context.Context, runtime invocation.Context) error {
	account, err := runtime.Prompt.Text("Account ID (for example, team-gateway): ")
	if err != nil {
		return err
	}
	if !configuration.ValidProfileName(account) {
		return fmt.Errorf("invalid account ID %q; use letters, numbers, dots, hyphens, or underscores", account)
	}
	label, err := runtime.Prompt.Text("Provider display name: ")
	if err != nil {
		return err
	}
	client, err := runtime.Prompt.Select("Client for the first profile: ", []prompt.Choice{
		{Value: configuration.ClientCodex, Label: "Codex (OpenAI Responses)"},
		{Value: configuration.ClientClaude, Label: "Claude (Anthropic)"},
	})
	if err != nil {
		return err
	}
	endpointLabel := "OpenAI Responses URL: "
	if client == configuration.ClientClaude {
		endpointLabel = "Anthropic URL: "
	}
	endpoint, err := runtime.Prompt.Text(endpointLabel)
	if err != nil {
		return err
	}
	profile, err := runtime.Prompt.Text("Profile ID (for example, gpt-5.6-terra): ")
	if err != nil {
		return err
	}
	model, err := runtime.Prompt.Text("Upstream model ID: ")
	if err != nil {
		return err
	}
	request := Request{Account: account, Profile: profile, Label: label, Client: client, Model: model, PromptToken: true}
	if client == configuration.ClientCodex {
		request.OpenAIURL = endpoint
	} else {
		request.AnthropicURL = endpoint
	}
	return runSetup(ctx, runtime, request)
}
