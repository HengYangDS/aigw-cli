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
	domainverification "aigw-cli/internal/verification"
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

func runSetup(ctx context.Context, runtime invocation.Context, request Request) error {
	cfg, err := runtime.Config.Load()
	if err != nil {
		return err
	}
	if len(cfg.Profiles) > 0 {
		return fmt.Errorf("AIGW is already configured; run `aigw add` to add an account, `aigw profile add` to add a model profile, or `aigw status` to inspect current state")
	}
	before := cfg.Clone()
	request.Profile = strings.TrimSpace(request.Profile)
	request.Account = strings.TrimSpace(request.Account)
	if request.Profile == "" {
		return fmt.Errorf("--profile is required; for example: `aigw setup --account team-gateway --profile gpt-5.6 --for codex --model gpt-5.6 --openai-url https://gateway.example/v1`")
	}
	if request.Account == "" {
		request.Account = request.Profile
	}
	if !configuration.ValidProfileName(request.Account) {
		return fmt.Errorf("Invalid account ID %q; use letters, numbers, dots, hyphens, or underscores", request.Account)
	}
	if !configuration.ValidProfileName(request.Profile) {
		return fmt.Errorf("Invalid profile ID %q; use letters, numbers, dots, hyphens, or underscores", request.Profile)
	}
	if request.Label == "" {
		request.Label = request.Account
	}
	endpoints := configuration.Endpoints{
		OpenAIResponses: strings.TrimRight(strings.TrimSpace(request.OpenAIURL), "/"),
		Anthropic:       strings.TrimRight(strings.TrimSpace(request.AnthropicURL), "/"),
	}
	models := configuration.Models{}
	if request.Client == "" {
		if request.Model != "" {
			return fmt.Errorf("--model requires --for %s", configuration.AdmittedClientUsage())
		}
	} else {
		spec, ok := configuration.ClientSpecFor(request.Client)
		if !ok {
			return fmt.Errorf("--for must be %s; run `aigw setup --help`", configuration.AdmittedClientUsage())
		}
		account := configuration.Account{ID: request.Account, Endpoints: endpoints}
		if _, err := spec.Endpoint(account); err != nil {
			var missing *configuration.RuntimeMissingEndpointError
			if !errors.As(err, &missing) {
				return err
			}
			return fmt.Errorf("--for %s requires %s", request.Client, setupEndpointFlag(spec.EndpointProtocol))
		}
		if strings.TrimSpace(request.Model) == "" {
			return fmt.Errorf("--for %s requires --model", request.Client)
		}
		models[request.Client] = strings.TrimSpace(request.Model)
	}
	account := configuration.Account{Label: request.Label, Endpoints: endpoints}
	profile := configuration.Profile{Label: request.Label, Account: request.Account, Client: request.Client, Models: models}
	cfg.Accounts[request.Account] = account
	cfg.Profiles[request.Profile] = profile
	cfg.Routes.Default = request.Profile
	if err := cfg.Validate(); err != nil {
		return err
	}
	token, secretAlreadyManaged, err := setupToken(runtime, request)
	if err != nil {
		return err
	}
	account.ID = request.Account
	validationClients := []string{}
	if request.Client != "" {
		validationClients = append(validationClients, request.Client)
	}
	if err := credential.Validate(ctx, runtime.HTTP, account, token, validationClients...); err != nil {
		return fmt.Errorf("Token validation failed: %w", err)
	}

	discovered, err := invocation.Discover(runtime)
	if err != nil {
		return err
	}
	discoveredClaude := discovered.Executable(configuration.ClientClaude)
	discoveredCodex := discovered.Executable(configuration.ClientCodex)
	discoveredTargets := discovered.AutoManagedCodexTargets()
	if runtime, _, resolveErr := cfg.ResolveRuntime(configuration.ClientClaude, ""); resolveErr == nil && discoveredClaude != "" && runtime.Endpoint != "" {
		cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: discoveredClaude}
	}
	if runtime, _, resolveErr := cfg.ResolveRuntime(configuration.ClientCodex, ""); resolveErr == nil && discoveredCodex != "" && len(discoveredTargets) > 0 && runtime.Endpoint != "" {
		cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: discoveredCodex, Targets: discoveredTargets}
	}

	r := invocation.Renderer(runtime)
	r.ProductTitle("First-time setup")
	r.Section("Service")
	r.Row("Account", request.Account)
	r.Row("Profile", request.Profile)
	r.Row("model", profile.ModelFor(request.Client))
	r.Status(presentation.OK, "API Token", "Validated")
	if !secretAlreadyManaged {
		if err := runtime.Secrets.Set(request.Account, token); err != nil {
			return err
		}
	}
	if err := invocation.Synchronizer(runtime).Commit(ctx, before, cfg, "setup"); err != nil {
		rollbackSetup(runtime, request.Account, !secretAlreadyManaged)
		return fmt.Errorf("Client configuration failed and was rolled back: %w", err)
	}

	r.Section("Clients")
	if cfg.Adapters[configuration.ClientClaude].Enabled {
		r.Status(presentation.OK, "Claude", "Configured")
	} else {
		r.Status(presentation.Info, "Claude", "Not configured")
	}
	if cfg.Adapters[configuration.ClientCodex].Enabled {
		r.Status(presentation.OK, "Codex", "Configured")
	} else {
		r.Status(presentation.Info, "Codex", "Not configured")
	}
	r.Success("Ready. You can add more model profiles for this account.")
	r.Next("aigw check")
	return nil
}

func setupEndpointFlag(protocol configuration.EndpointProtocol) string {
	switch protocol {
	case configuration.ProtocolAnthropic:
		return "--anthropic-url"
	case configuration.ProtocolOpenAIResponses:
		return "--openai-url"
	default:
		return "a supported endpoint"
	}
}

type manifestSetupCredential struct {
	account     string
	token       string
	previous    string
	hadPrevious bool
	write       bool
}

func runManifestSetup(ctx context.Context, runtime invocation.Context, request Request) error {
	data, err := os.ReadFile(request.From)
	if err != nil {
		return fmt.Errorf("Failed to read configuration manifest: %w", err)
	}
	manifest, err := configuration.Parse(data)
	if err != nil {
		return err
	}
	cfg, err := runtime.Config.Load()
	if err != nil {
		return err
	}
	if len(cfg.Profiles) > 0 {
		return fmt.Errorf("AIGW is already configured; run `aigw config import %s` to merge a reviewed configuration manifest", request.From)
	}
	before := cfg.Clone()
	cfg, err = configuration.Merge(cfg, manifest)
	if err != nil {
		return err
	}

	accountNames := configuration.ManifestAccountNames(manifest)
	discovered, err := invocation.Discover(runtime)
	if err != nil {
		return err
	}
	discoveredClaude := discovered.Executable(configuration.ClientClaude)
	discoveredCodex := discovered.Executable(configuration.ClientCodex)
	discoveredTargets := discovered.AutoManagedCodexTargets()
	if runtime, _, resolveErr := cfg.ResolveRuntime(configuration.ClientCodex, ""); resolveErr == nil && runtime.Endpoint != "" && discoveredCodex != "" && len(discoveredTargets) > 1 {
		return fmt.Errorf("configuration setup found multiple auto-managed Codex targets; automatic native credential binding is not atomic across targets, so reduce the admitted target set before setup or import the manifest without first-time client binding")
	}
	for _, accountName := range accountNames {
		if len(configuredClientsForAccount(cfg, accountName)) == 0 {
			return fmt.Errorf("Account %q is not referenced by any configuration profile; remove it or add an explicit client profile before setup", accountName)
		}
	}
	credentials, err := collectManifestSetupCredentials(runtime, cfg, accountNames, request.TokenStdin)
	if err != nil {
		return err
	}
	for _, credential := range credentials {
		if err := verifyManifestSetupCredential(ctx, runtime, cfg, credential.account, credential.token, discoveredClaude); err != nil {
			return fmt.Errorf("Token validation failed for Account %q: %w", credential.account, err)
		}
	}

	if runtime, _, resolveErr := cfg.ResolveRuntime(configuration.ClientClaude, ""); resolveErr == nil && discoveredClaude != "" && runtime.Endpoint != "" {
		cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: discoveredClaude}
	}
	if runtime, _, resolveErr := cfg.ResolveRuntime(configuration.ClientCodex, ""); resolveErr == nil && discoveredCodex != "" && len(discoveredTargets) > 0 && runtime.Endpoint != "" {
		cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: discoveredCodex, Targets: discoveredTargets}
	}

	written, err := writeManifestSetupCredentials(runtime, credentials)
	if err != nil {
		return err
	}
	if err := invocation.Synchronizer(runtime).Commit(ctx, before, cfg, "configuration setup"); err != nil {
		if rollbackErr := rollbackManifestSetupCredentials(runtime, credentials, written); rollbackErr != nil {
			return fmt.Errorf("configuration setup failed: %w; credential rollback also failed: %v", err, rollbackErr)
		}
		return fmt.Errorf("Configuration setup failed and credentials were rolled back: %w", err)
	}

	r := invocation.Renderer(runtime)
	r.ProductTitle("Configuration setup")
	r.Section("Configuration manifest")
	r.Row("Accounts", fmt.Sprintf("%d", len(accountNames)))
	r.Row("Model profiles", fmt.Sprintf("%d", len(manifest.Profiles)))
	r.Row("Default profile", cfg.Routes.Default)
	r.Section("Credentials")
	for _, name := range accountNames {
		r.Status(presentation.OK, name, "Token validated")
	}
	r.Section("Clients")
	if cfg.Adapters[configuration.ClientClaude].Enabled {
		r.Status(presentation.OK, "Claude", "Configured")
	} else {
		r.Status(presentation.Info, "Claude", "Not selected")
	}
	if cfg.Adapters[configuration.ClientCodex].Enabled {
		r.Status(presentation.OK, "Codex", "Configured")
	} else {
		r.Status(presentation.Info, "Codex", "Not selected")
	}
	r.Success("Configuration manifest saved; Tokens remain in system secret storage")
	r.Next("aigw check")
	return nil
}

func collectManifestSetupCredentials(runtime invocation.Context, cfg configuration.Config, accountNames []string, tokenStdin bool) ([]manifestSetupCredential, error) {
	if tokenStdin && len(accountNames) != 1 {
		return nil, fmt.Errorf("--token-stdin cannot assign one Token to a configuration manifest with multiple accounts; run setup interactively or pre-provision each Account Token")
	}
	credentials := make([]manifestSetupCredential, 0, len(accountNames))
	missing := make([]int, 0, len(accountNames))
	for _, name := range accountNames {
		credential := manifestSetupCredential{account: name}
		previous, err := runtime.Secrets.Get(name)
		switch {
		case err == nil:
			credential.token = previous
			credential.previous = previous
			credential.hadPrevious = true
		case errors.Is(err, secrets.ErrNotFound):
			missing = append(missing, len(credentials))
		default:
			return nil, err
		}
		credentials = append(credentials, credential)
	}

	if tokenStdin {
		if secrets.IsReadOnly(runtime.Secrets) {
			return nil, fmt.Errorf("--token-stdin cannot replace a Token in the read-only environment secret backend")
		}
		token, err := invocation.ReadToken(runtime, true, true)
		if err != nil {
			return nil, err
		}
		credentials[0].token = token
		credentials[0].write = true
		return credentials, nil
	}
	if len(missing) == 0 {
		return credentials, nil
	}
	missingNames := make([]string, 0, len(missing))
	for _, index := range missing {
		missingNames = append(missingNames, credentials[index].account)
	}
	if secrets.IsReadOnly(runtime.Secrets) {
		return nil, fmt.Errorf("read-only environment secret backend is missing Tokens for Accounts %s; pre-provision each AIGW_TOKEN_<ACCOUNT> value", strings.Join(missingNames, ", "))
	}
	if !runtime.Interactive {
		return nil, fmt.Errorf("Accounts %s are missing Tokens; run `aigw setup --from <configuration.toml>` in an interactive terminal or pre-provision each Account Token", strings.Join(missingNames, ", "))
	}
	for _, index := range missing {
		account := cfg.Accounts[credentials[index].account]
		token, err := runtime.Prompt.Secret("Paste " + account.Label + " token: ")
		if err != nil {
			return nil, err
		}
		if token == "" {
			return nil, fmt.Errorf("empty Token refused for Account %q", credentials[index].account)
		}
		credentials[index].token = token
		credentials[index].write = true
	}
	return credentials, nil
}

func writeManifestSetupCredentials(runtime invocation.Context, credentials []manifestSetupCredential) ([]int, error) {
	written := make([]int, 0, len(credentials))
	for index, credential := range credentials {
		if !credential.write {
			continue
		}
		if err := runtime.Secrets.Set(credential.account, credential.token); err != nil {
			if rollbackErr := rollbackManifestSetupCredentials(runtime, credentials, written); rollbackErr != nil {
				return nil, fmt.Errorf("store Token for Account %q: %w; credential rollback also failed: %v", credential.account, err, rollbackErr)
			}
			return nil, fmt.Errorf("store Token for Account %q: %w", credential.account, err)
		}
		written = append(written, index)
	}
	return written, nil
}

func rollbackManifestSetupCredentials(runtime invocation.Context, credentials []manifestSetupCredential, written []int) error {
	var rollbackErr error
	for position := len(written) - 1; position >= 0; position-- {
		credential := credentials[written[position]]
		var err error
		if credential.hadPrevious {
			err = runtime.Secrets.Set(credential.account, credential.previous)
		} else {
			err = runtime.Secrets.Delete(credential.account)
		}
		if err != nil {
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("restore Token for Account %q: %w", credential.account, err))
		}
	}
	return rollbackErr
}

func configuredClientsForAccount(cfg configuration.Config, accountName string) []string {
	seen := map[string]bool{}
	genericProfile := false
	for profileName, profile := range cfg.Profiles {
		owner := profile.Account
		if owner == "" {
			owner = profileName
		}
		if owner != accountName {
			continue
		}
		specificClient := false
		if configuration.IsAdmittedClient(profile.Client) {
			seen[profile.Client] = true
			specificClient = true
		}
		for client, model := range profile.Models {
			if configuration.IsAdmittedClient(client) && strings.TrimSpace(model) != "" {
				seen[client] = true
				specificClient = true
			}
		}
		genericProfile = genericProfile || !specificClient
	}
	if genericProfile {
		account := cfg.Accounts[accountName]
		if account.Endpoints.Anthropic != "" {
			seen[configuration.ClientClaude] = true
		}
		if account.Endpoints.OpenAIResponses != "" {
			seen[configuration.ClientCodex] = true
		}
	}
	clients := make([]string, 0, len(seen))
	for _, client := range configuration.AdmittedClientIDs() {
		if seen[client] {
			clients = append(clients, client)
		}
	}
	return clients
}

func verifyManifestSetupCredential(ctx context.Context, runtime invocation.Context, cfg configuration.Config, accountName, token, claudeExecutable string) error {
	account := cfg.Accounts[accountName]
	account.ID = accountName
	for _, client := range configuredClientsForAccount(cfg, accountName) {
		if client == configuration.ClientClaude && claudeExecutable != "" {
			if clientRuntime, ok := firstRuntimeForAccountClient(cfg, accountName, client); ok {
				if err := domainverification.VerifyClaudeRuntime(ctx, runtime.Runner, claudeExecutable, clientRuntime, token); err != nil {
					return err
				}
				continue
			}
		}
		if err := credential.Validate(ctx, runtime.HTTP, account, token, client); err != nil {
			return err
		}
	}
	return nil
}

func firstRuntimeForAccountClient(cfg configuration.Config, accountName, client string) (configuration.Runtime, bool) {
	for _, profileName := range cfg.ProfileIDs() {
		profile := cfg.Profiles[profileName]
		owner := profile.Account
		if owner == "" {
			owner = profileName
		}
		if owner != accountName {
			continue
		}
		runtime, _, err := cfg.ResolveRuntime(client, profileName)
		if err == nil && runtime.Model != "" {
			return runtime, true
		}
	}
	return configuration.Runtime{}, false
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
