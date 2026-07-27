package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/manifest"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/presentation"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/secrets"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/shims"
)

type setupRequest struct {
	From, Account, Profile, Label string
	OpenAIURL, AnthropicURL       string
	Client, Model                 string
	TokenStdin                    bool
	PromptToken                   bool
}

func newSetupCommand(app *App) *cobra.Command {
	request := setupRequest{}
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Complete first-time setup",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			request.From = strings.TrimSpace(request.From)
			if cmd.Flags().Changed("from") {
				if request.From == "" {
					return fmt.Errorf("--from requires a team manifest path")
				}
				for _, name := range []string{"account", "profile", "label", "openai-url", "anthropic-url", "for", "model"} {
					if !cmd.Flags().Changed(name) {
						continue
					}
					return fmt.Errorf("--from cannot be combined with --account, --profile, --label, --openai-url, --anthropic-url, --for, or --model")
				}
				return runTeamSetup(cmd.Context(), app, request)
			}
			if app.Interactive && request.Account == "" && request.Profile == "" && request.Label == "" && request.OpenAIURL == "" && request.AnthropicURL == "" && request.Client == "" && request.Model == "" && !request.TokenStdin {
				cfg, err := app.Config.Load()
				if err != nil {
					return err
				}
				if len(cfg.Profiles) > 0 {
					return fmt.Errorf("AIGW is already configured; run `aigw add` to add an account, `aigw profile add` to add a model profile, or `aigw status` to inspect current state")
				}
				return runWizard(cmd.Context(), app)
			}
			return runSetup(cmd.Context(), app, request)
		},
	}
	cmd.Flags().StringVar(&request.From, "from", "", "Set up all profiles from a token-free team manifest")
	cmd.Flags().StringVar(&request.Account, "account", "", "Account ID; defaults to --profile")
	cmd.Flags().StringVar(&request.Profile, "profile", "", "First profile ID")
	cmd.Flags().StringVar(&request.Label, "label", "", "Provider display name")
	cmd.Flags().StringVar(&request.OpenAIURL, "openai-url", "", "OpenAI Responses base URL")
	cmd.Flags().StringVar(&request.AnthropicURL, "anthropic-url", "", "Anthropic base URL")
	cmd.Flags().StringVar(&request.Client, "for", "", "Client for the first profile: claude or codex")
	cmd.Flags().StringVar(&request.Model, "model", "", "Upstream model ID for --for")
	cmd.Flags().BoolVar(&request.TokenStdin, "token-stdin", false, "Read one token line from standard input")
	return cmd
}

func runSetup(ctx context.Context, app *App, request setupRequest) error {
	cfg, err := app.Config.Load()
	if err != nil {
		return err
	}
	if len(cfg.Profiles) > 0 {
		return fmt.Errorf("AIGW is already configured; run `aigw add` to add an account, `aigw profile add` to add a model profile, or `aigw status` to inspect current state")
	}
	before := cloneConfig(cfg)
	request.Profile = strings.TrimSpace(request.Profile)
	request.Account = strings.TrimSpace(request.Account)
	if request.Profile == "" {
		return fmt.Errorf("--profile is required; for example: `aigw setup --account team-gateway --profile gpt-5.6 --for codex --model gpt-5.6 --openai-url https://gateway.example/v1`")
	}
	if request.Account == "" {
		request.Account = request.Profile
	}
	if !domain.ValidProfileName(request.Account) {
		return fmt.Errorf("Invalid account ID %q; use letters, numbers, dots, hyphens, or underscores", request.Account)
	}
	if !domain.ValidProfileName(request.Profile) {
		return fmt.Errorf("Invalid profile ID %q; use letters, numbers, dots, hyphens, or underscores", request.Profile)
	}
	if request.Label == "" {
		request.Label = request.Account
	}
	endpoints := domain.Endpoints{
		OpenAIResponses: strings.TrimRight(strings.TrimSpace(request.OpenAIURL), "/"),
		Anthropic:       strings.TrimRight(strings.TrimSpace(request.AnthropicURL), "/"),
	}
	models := domain.Models{}
	switch request.Client {
	case "":
		if request.Model != "" {
			return fmt.Errorf("--model requires --for claude or --for codex")
		}
	case domain.ClientClaude:
		if endpoints.Anthropic == "" {
			return fmt.Errorf("--for claude requires --anthropic-url")
		}
		if strings.TrimSpace(request.Model) == "" {
			return fmt.Errorf("--for claude requires --model")
		}
		models[request.Client] = strings.TrimSpace(request.Model)
	case domain.ClientCodex:
		if endpoints.OpenAIResponses == "" {
			return fmt.Errorf("--for codex requires --openai-url")
		}
		if strings.TrimSpace(request.Model) == "" {
			return fmt.Errorf("--for codex requires --model")
		}
		models[request.Client] = strings.TrimSpace(request.Model)
	default:
		return fmt.Errorf("--for must be claude or codex; run `aigw setup --help`")
	}
	account := domain.Account{Label: request.Label, Endpoints: endpoints}
	profile := domain.Profile{Label: request.Label, Account: request.Account, Client: request.Client, Models: models}
	cfg.Accounts[request.Account] = account
	cfg.Profiles[request.Profile] = profile
	cfg.Routes.Default = request.Profile
	if err := cfg.Validate(); err != nil {
		return err
	}
	token, secretAlreadyManaged, err := setupToken(app, request)
	if err != nil {
		return err
	}
	account.ID = request.Account
	validationClients := []string{}
	if request.Client != "" {
		validationClients = append(validationClients, request.Client)
	}
	if err := verifyCredential(ctx, app, account, token, validationClients...); err != nil {
		return fmt.Errorf("Token validation failed: %w", err)
	}

	discovered, err := discoveredResult(app)
	if err != nil {
		return err
	}
	discoveredClaude := discovered.ClaudeExecutable
	discoveredCodex := discovered.CodexExecutable
	discoveredTargets := discovered.AutoManagedCodexTargets()
	if runtime, _, resolveErr := cfg.ResolveRuntime(domain.ClientClaude, ""); resolveErr == nil && discoveredClaude != "" && runtime.Endpoint != "" {
		cfg.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true, Executable: discoveredClaude}
	}
	if runtime, _, resolveErr := cfg.ResolveRuntime(domain.ClientCodex, ""); resolveErr == nil && discoveredCodex != "" && len(discoveredTargets) > 0 && runtime.Endpoint != "" {
		cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: discoveredCodex, Targets: discoveredTargets}
	}

	r := renderer(app)
	r.Title("AIGW", "First-time setup")
	r.Section("Service")
	r.Row("Account", request.Account)
	r.Row("Profile", request.Profile)
	r.Row("model", profile.ModelFor(request.Client))
	r.Status(presentation.OK, "API Token", "Validated")
	if !secretAlreadyManaged {
		if err := app.Secrets.Set(request.Account, token); err != nil {
			return err
		}
	}
	claudeEnabled := cfg.Adapters[domain.ClientClaude].Enabled
	if claudeEnabled {
		if _, err := app.Shims.EnableClaude(); err != nil {
			if !secretAlreadyManaged {
				_ = app.Secrets.Delete(request.Account)
			}
			return err
		}
	}
	if err := commitConfigAndSync(ctx, app, before, cfg, "setup"); err != nil {
		rollbackSetup(app, request.Account, claudeEnabled, !secretAlreadyManaged)
		return fmt.Errorf("Client configuration failed and was rolled back: %w", err)
	}

	r.Section("Clients")
	if cfg.Adapters[domain.ClientClaude].Enabled {
		r.Status(presentation.OK, "Claude", "Configured")
	} else {
		r.Status(presentation.Info, "Claude", "Not configured")
	}
	if cfg.Adapters[domain.ClientCodex].Enabled {
		r.Status(presentation.OK, "Codex", "Configured")
	} else {
		r.Status(presentation.Info, "Codex", "Not configured")
	}
	r.Success("Ready. You can add more model profiles for this account.")
	r.Next("aigw check")
	return nil
}

type teamSetupCredential struct {
	account     string
	token       string
	previous    string
	hadPrevious bool
	write       bool
}

func runTeamSetup(ctx context.Context, app *App, request setupRequest) error {
	data, err := os.ReadFile(request.From)
	if err != nil {
		return fmt.Errorf("Failed to read team manifest: %w", err)
	}
	team, err := manifest.Parse(data)
	if err != nil {
		return err
	}
	cfg, err := app.Config.Load()
	if err != nil {
		return err
	}
	if len(cfg.Profiles) > 0 {
		return fmt.Errorf("AIGW is already configured; run `aigw config import %s` to merge a reviewed team manifest", request.From)
	}
	before := cloneConfig(cfg)
	cfg, err = manifest.Merge(cfg, team)
	if err != nil {
		return err
	}

	accountNames := importedAccountNames(team)
	discovered, err := discoveredResult(app)
	if err != nil {
		return err
	}
	discoveredClaude := discovered.ClaudeExecutable
	discoveredCodex := discovered.CodexExecutable
	discoveredTargets := discovered.AutoManagedCodexTargets()
	if runtime, _, resolveErr := cfg.ResolveRuntime(domain.ClientCodex, ""); resolveErr == nil && runtime.Endpoint != "" && discoveredCodex != "" && len(discoveredTargets) > 1 {
		return fmt.Errorf("team setup found multiple auto-managed Codex targets; automatic native credential binding is not atomic across targets, so reduce the admitted target set before setup or import the manifest without first-time client binding")
	}
	for _, accountName := range accountNames {
		if len(configuredClientsForAccount(cfg, accountName)) == 0 {
			return fmt.Errorf("Account %q is not referenced by any team profile; remove it or add an explicit client profile before setup", accountName)
		}
	}
	credentials, err := collectTeamSetupCredentials(app, cfg, accountNames, request.TokenStdin)
	if err != nil {
		return err
	}
	for _, credential := range credentials {
		if err := verifyTeamSetupCredential(ctx, app, cfg, credential.account, credential.token, discoveredClaude); err != nil {
			return fmt.Errorf("Token validation failed for Account %q: %w", credential.account, err)
		}
	}

	if runtime, _, resolveErr := cfg.ResolveRuntime(domain.ClientClaude, ""); resolveErr == nil && discoveredClaude != "" && runtime.Endpoint != "" {
		cfg.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true, Executable: discoveredClaude}
	}
	if runtime, _, resolveErr := cfg.ResolveRuntime(domain.ClientCodex, ""); resolveErr == nil && discoveredCodex != "" && len(discoveredTargets) > 0 && runtime.Endpoint != "" {
		cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: discoveredCodex, Targets: discoveredTargets}
	}

	claudeEnabled := cfg.Adapters[domain.ClientClaude].Enabled
	launcherBefore := shims.ClaudeStateSnapshot{}
	launcherAfter := shims.ClaudeStateSnapshot{}
	if claudeEnabled {
		launcherBefore, err = app.Shims.CaptureClaudeState()
		if err != nil {
			return err
		}
	}
	written, err := writeTeamSetupCredentials(app, credentials)
	if err != nil {
		return err
	}
	if claudeEnabled {
		if _, err := app.Shims.EnableClaude(); err != nil {
			if rollbackErr := rollbackTeamSetupCredentials(app, credentials, written); rollbackErr != nil {
				return fmt.Errorf("enable Claude adapter: %w; credential rollback also failed: %v", err, rollbackErr)
			}
			return err
		}
		launcherAfter, err = app.Shims.CaptureClaudeState()
		if err != nil {
			if rollbackErr := rollbackTeamSetupCredentials(app, credentials, written); rollbackErr != nil {
				return fmt.Errorf("capture Claude launcher postimage: %w; credential rollback also failed: %v", err, rollbackErr)
			}
			return fmt.Errorf("capture Claude launcher postimage: %w", err)
		}
	}
	if err := commitConfigAndSync(ctx, app, before, cfg, "team setup"); err != nil {
		var launcherRollbackErr error
		if claudeEnabled {
			launcherRollbackErr = app.Shims.RestoreClaudeState(launcherBefore, launcherAfter)
		}
		if rollbackErr := rollbackTeamSetupCredentials(app, credentials, written); rollbackErr != nil {
			return fmt.Errorf("team setup failed: %w; credential rollback also failed: %v", err, rollbackErr)
		}
		if launcherRollbackErr != nil {
			return fmt.Errorf("team setup failed: %w; Claude launcher rollback also failed: %v", err, launcherRollbackErr)
		}
		return fmt.Errorf("Team setup failed and credentials were rolled back: %w", err)
	}

	r := renderer(app)
	r.Title("AIGW", "Team setup")
	r.Section("Team configuration")
	r.Row("Accounts", fmt.Sprintf("%d", len(accountNames)))
	r.Row("Model profiles", fmt.Sprintf("%d", len(team.Profiles)))
	r.Row("Default profile", cfg.Routes.Default)
	r.Section("Credentials")
	for _, name := range accountNames {
		r.Status(presentation.OK, name, "Token validated")
	}
	r.Section("Clients")
	if cfg.Adapters[domain.ClientClaude].Enabled {
		r.Status(presentation.OK, "Claude", "Configured")
	} else {
		r.Status(presentation.Info, "Claude", "Not selected")
	}
	if cfg.Adapters[domain.ClientCodex].Enabled {
		r.Status(presentation.OK, "Codex", "Configured")
	} else {
		r.Status(presentation.Info, "Codex", "Not selected")
	}
	r.Success("Team configuration saved; Tokens remain in system secret storage")
	r.Next("aigw check")
	return nil
}

func collectTeamSetupCredentials(app *App, cfg domain.Config, accountNames []string, tokenStdin bool) ([]teamSetupCredential, error) {
	if tokenStdin && len(accountNames) != 1 {
		return nil, fmt.Errorf("--token-stdin cannot assign one Token to a team manifest with multiple accounts; run setup interactively or pre-provision each Account Token")
	}
	credentials := make([]teamSetupCredential, 0, len(accountNames))
	missing := make([]int, 0, len(accountNames))
	for _, name := range accountNames {
		credential := teamSetupCredential{account: name}
		previous, err := app.Secrets.Get(name)
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
		if secrets.IsReadOnly(app.Secrets) {
			return nil, fmt.Errorf("--token-stdin cannot replace a Token in the read-only environment secret backend")
		}
		token, err := app.readToken(true, true)
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
	if secrets.IsReadOnly(app.Secrets) {
		return nil, fmt.Errorf("read-only environment secret backend is missing Tokens for Accounts %s; pre-provision each AIGW_TOKEN_<ACCOUNT> value", strings.Join(missingNames, ", "))
	}
	if !app.Interactive {
		return nil, fmt.Errorf("Accounts %s are missing Tokens; run `aigw setup --from <team-profiles.toml>` in an interactive terminal or pre-provision each Account Token", strings.Join(missingNames, ", "))
	}
	for _, index := range missing {
		account := cfg.Accounts[credentials[index].account]
		token, err := app.Prompt.Secret("Paste " + account.Label + " token: ")
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

func writeTeamSetupCredentials(app *App, credentials []teamSetupCredential) ([]int, error) {
	written := make([]int, 0, len(credentials))
	for index, credential := range credentials {
		if !credential.write {
			continue
		}
		if err := app.Secrets.Set(credential.account, credential.token); err != nil {
			if rollbackErr := rollbackTeamSetupCredentials(app, credentials, written); rollbackErr != nil {
				return nil, fmt.Errorf("store Token for Account %q: %w; credential rollback also failed: %v", credential.account, err, rollbackErr)
			}
			return nil, fmt.Errorf("store Token for Account %q: %w", credential.account, err)
		}
		written = append(written, index)
	}
	return written, nil
}

func rollbackTeamSetupCredentials(app *App, credentials []teamSetupCredential, written []int) error {
	var rollbackErr error
	for position := len(written) - 1; position >= 0; position-- {
		credential := credentials[written[position]]
		var err error
		if credential.hadPrevious {
			err = app.Secrets.Set(credential.account, credential.previous)
		} else {
			err = app.Secrets.Delete(credential.account)
		}
		if err != nil {
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("restore Token for Account %q: %w", credential.account, err))
		}
	}
	return rollbackErr
}

func configuredClientsForAccount(cfg domain.Config, accountName string) []string {
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
		if domain.IsAdmittedClient(profile.Client) {
			seen[profile.Client] = true
			specificClient = true
		}
		for client, model := range profile.Models {
			if domain.IsAdmittedClient(client) && strings.TrimSpace(model) != "" {
				seen[client] = true
				specificClient = true
			}
		}
		genericProfile = genericProfile || !specificClient
	}
	if genericProfile {
		account := cfg.Accounts[accountName]
		if account.Endpoints.Anthropic != "" {
			seen[domain.ClientClaude] = true
		}
		if account.Endpoints.OpenAIResponses != "" {
			seen[domain.ClientCodex] = true
		}
	}
	clients := make([]string, 0, len(seen))
	for _, client := range domain.AdmittedClientIDs() {
		if seen[client] {
			clients = append(clients, client)
		}
	}
	return clients
}

func verifyTeamSetupCredential(ctx context.Context, app *App, cfg domain.Config, accountName, token, claudeExecutable string) error {
	account := cfg.Accounts[accountName]
	account.ID = accountName
	for _, client := range configuredClientsForAccount(cfg, accountName) {
		if client == domain.ClientClaude && claudeExecutable != "" {
			if runtime, ok := firstRuntimeForAccountClient(cfg, accountName, client); ok {
				if err := verifyClaudeRuntimeWithExecutable(ctx, app, claudeExecutable, runtime, token); err != nil {
					return err
				}
				continue
			}
		}
		if err := verifyCredential(ctx, app, account, token, client); err != nil {
			return err
		}
	}
	return nil
}

func firstRuntimeForAccountClient(cfg domain.Config, accountName, client string) (domain.Runtime, bool) {
	for _, profileName := range sortedProfileNames(cfg) {
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
	return domain.Runtime{}, false
}

// setupToken prefers a credential that was already supplied by the active
// secret backend. This is essential for non-interactive CI/container use with
// AIGW_SECRET_BACKEND=env: the environment store is intentionally read-only,
// so setup must validate and reference its token rather than asking for a
// second copy and attempting to persist it.
func setupToken(app *App, request setupRequest) (token string, alreadyManaged bool, err error) {
	if !request.PromptToken && !request.TokenStdin {
		token, err = app.Secrets.Get(request.Account)
		if err == nil {
			return token, true, nil
		}
		if !errors.Is(err, secrets.ErrNotFound) {
			return "", false, err
		}
	}
	if request.PromptToken {
		token, err = app.Prompt.Secret("Paste " + request.Label + " token: ")
		return token, false, err
	}
	token, err = app.readToken(request.TokenStdin, true)
	return token, false, err
}

func verifyCredential(ctx context.Context, app *App, providerAccount domain.Account, token string, clients ...string) error {
	if len(clients) == 0 {
		if providerAccount.Endpoints.OpenAIResponses != "" {
			clients = []string{domain.ClientCodex}
		} else {
			clients = []string{domain.ClientClaude}
		}
	}
	seen := map[string]bool{}
	for _, client := range clients {
		if seen[client] {
			continue
		}
		seen[client] = true
		spec, ok := domain.ClientSpecFor(client)
		if !ok {
			return fmt.Errorf("unsupported credential validation client %q", client)
		}
		endpoint, err := providerAccount.EndpointFor(client)
		if err != nil {
			return err
		}
		testURL := endpoint
		switch client {
		case domain.ClientClaude:
			testURL = anthropicModelsEndpoint(endpoint)
		case domain.ClientCodex:
			testURL = codexModelsEndpoint(endpoint)
		}
		checkCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
		req, err := http.NewRequestWithContext(checkCtx, http.MethodGet, testURL, nil)
		if err != nil {
			cancel()
			return err
		}
		switch spec.EndpointProtocol {
		case domain.ProtocolAnthropic:
			req.Header.Set("X-Api-Key", token)
			req.Header.Set("Anthropic-Version", "2023-06-01")
		case domain.ProtocolOpenAIResponses:
			req.Header.Set("Authorization", "Bearer "+token)
		default:
			cancel()
			return fmt.Errorf("%s endpoint protocol %q is unsupported", title(client), spec.EndpointProtocol)
		}
		httpClient := app.HTTP
		if standardClient, ok := app.HTTP.(*http.Client); ok {
			noRedirectClient := *standardClient
			noRedirectClient.CheckRedirect = func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			}
			httpClient = &noRedirectClient
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			cancel()
			return fmt.Errorf("%s endpoint is unreachable: %w", title(client), err)
		}
		if _, err := io.Copy(io.Discard, resp.Body); err != nil {
			_ = resp.Body.Close()
			cancel()
			return fmt.Errorf("read %s endpoint response: %w", title(client), err)
		}
		if err := resp.Body.Close(); err != nil {
			cancel()
			return fmt.Errorf("close %s endpoint response: %w", title(client), err)
		}
		cancel()
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return fmt.Errorf("%s authentication was rejected (HTTP %d)", title(client), resp.StatusCode)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("%s endpoint returned HTTP %d", title(client), resp.StatusCode)
		}
	}
	return nil
}

func anthropicModelsEndpoint(endpoint string) string {
	endpoint = strings.TrimRight(endpoint, "/")
	if strings.HasSuffix(endpoint, "/models") {
		return endpoint
	}
	if strings.HasSuffix(endpoint, "/v1") {
		return endpoint + "/models"
	}
	return endpoint + "/v1/models"
}

func rollbackSetup(app *App, account string, claudeEnabled, deleteNewSecret bool) {
	if claudeEnabled {
		_ = app.Shims.DisableClaude()
	}
	if deleteNewSecret {
		_ = app.Secrets.Delete(account)
	}
	_ = os.Remove(app.Config.Path())
	_ = os.Remove(app.Config.Path() + ".bak")
}
