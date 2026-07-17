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
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/presentation"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/secrets"
)

type setupRequest struct {
	Account, Profile, Label string
	OpenAIURL, AnthropicURL string
	Client, Model           string
	TokenStdin              bool
	PromptToken             bool
}

func newSetupCommand(app *App) *cobra.Command {
	request := setupRequest{}
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Complete first-time setup with flags",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
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
	if err := verifyCredential(ctx, app, account, token); err != nil {
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

func verifyCredential(ctx context.Context, app *App, providerAccount domain.Account, token string) error {
	endpoint := providerAccount.Endpoints.OpenAIResponses
	if endpoint == "" {
		endpoint = providerAccount.Endpoints.Anthropic
	}
	checkCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(checkCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("Gateway rejected authentication (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode >= 500 {
		return fmt.Errorf("Gateway is temporarily unavailable (HTTP %d)", resp.StatusCode)
	}
	return nil
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
