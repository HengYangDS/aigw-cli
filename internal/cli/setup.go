package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/presentation"
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
		Short: "以参数方式完成首次配置",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if app.Interactive && request.Account == "" && request.Profile == "" && request.Label == "" && request.OpenAIURL == "" && request.AnthropicURL == "" && request.Client == "" && request.Model == "" && !request.TokenStdin {
				cfg, err := app.Config.Load()
				if err != nil {
					return err
				}
				if len(cfg.Profiles) > 0 {
					return fmt.Errorf("AIGW 已完成首次配置；如需新增服务运行 `aigw add`，新增模型运行 `aigw profile add`，查看当前状态运行 `aigw status`")
				}
				return runWizard(cmd.Context(), app)
			}
			return runSetup(cmd.Context(), app, request)
		},
	}
	cmd.Flags().StringVar(&request.Account, "account", "", "Account 标识；默认使用 --profile")
	cmd.Flags().StringVar(&request.Profile, "profile", "", "首个模型 Profile 标识")
	cmd.Flags().StringVar(&request.Label, "label", "", "服务商显示名称")
	cmd.Flags().StringVar(&request.OpenAIURL, "openai-url", "", "OpenAI Responses 基础 URL")
	cmd.Flags().StringVar(&request.AnthropicURL, "anthropic-url", "", "Anthropic 基础 URL")
	cmd.Flags().StringVar(&request.Client, "for", "", "首个 Profile 的客户端：claude 或 codex")
	cmd.Flags().StringVar(&request.Model, "model", "", "--for 对应的上游模型 ID")
	cmd.Flags().BoolVar(&request.TokenStdin, "token-stdin", false, "从标准输入读取一行 Token")
	return cmd
}

func runSetup(ctx context.Context, app *App, request setupRequest) error {
	cfg, err := app.Config.Load()
	if err != nil {
		return err
	}
	if len(cfg.Profiles) > 0 {
		return fmt.Errorf("AIGW 已完成首次配置；如需新增服务运行 `aigw add`，新增模型运行 `aigw profile add`，查看当前状态运行 `aigw status`")
	}
	request.Profile = strings.TrimSpace(request.Profile)
	request.Account = strings.TrimSpace(request.Account)
	if request.Profile == "" {
		return fmt.Errorf("setup needs --profile; example: `aigw setup --account team-gateway --profile gpt-5.6 --for codex --model gpt-5.6 --openai-url https://gateway.example/v1`")
	}
	if request.Account == "" {
		request.Account = request.Profile
	}
	if !domain.ValidProfileName(request.Account) {
		return fmt.Errorf("invalid account name %q; use letters, numbers, dot, dash, or underscore", request.Account)
	}
	if !domain.ValidProfileName(request.Profile) {
		return fmt.Errorf("invalid profile name %q; use letters, numbers, dot, dash, or underscore", request.Profile)
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
		return fmt.Errorf("--for 只能是 claude 或 codex；运行 `aigw setup --help` 查看帮助")
	}
	account := domain.Account{Label: request.Label, Endpoints: endpoints}
	profile := domain.Profile{Label: request.Label, Account: request.Account, Client: request.Client, Models: models}
	cfg.Accounts[request.Account] = account
	cfg.Profiles[request.Profile] = profile
	cfg.Routes.Default = request.Profile
	if err := cfg.Validate(); err != nil {
		return err
	}
	var token string
	if request.PromptToken {
		token, err = app.Prompt.Secret("请粘贴 " + request.Label + " Token：")
	} else {
		token, err = app.readToken(request.TokenStdin, true)
	}
	if err != nil {
		return err
	}
	account.ID = request.Account
	if err := verifyCredential(ctx, app, account, token); err != nil {
		return fmt.Errorf("Token 验证失败：%w", err)
	}

	var discoveredClaude, discoveredCodex string
	var discoveredTargets []string
	if app.Discovery != nil {
		discovered := app.Discovery.Discover()
		discoveredClaude = discovered.ClaudeExecutable
		discoveredCodex = discovered.CodexExecutable
		discoveredTargets = discovered.CodexTargets
	}
	if runtime, _, resolveErr := cfg.ResolveRuntime(domain.ClientClaude, ""); resolveErr == nil && discoveredClaude != "" && runtime.Endpoint != "" {
		cfg.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true, Executable: discoveredClaude}
	}
	if runtime, _, resolveErr := cfg.ResolveRuntime(domain.ClientCodex, ""); resolveErr == nil && discoveredCodex != "" && len(discoveredTargets) > 0 && runtime.Endpoint != "" {
		cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: discoveredCodex, Targets: discoveredTargets}
	}

	r := renderer(app)
	r.Title("AIGW", "首次配置")
	r.Section("服务")
	r.Row("Account", request.Account)
	r.Row("Profile", request.Profile)
	r.Row("模型", profile.ModelFor(request.Client))
	r.Status(presentation.OK, "API Token", "验证通过")
	if err := app.Secrets.Set(request.Account, token); err != nil {
		return err
	}
	claudeEnabled := cfg.Adapters[domain.ClientClaude].Enabled
	if claudeEnabled {
		if _, err := app.Shims.EnableClaude(); err != nil {
			_ = app.Secrets.Delete(request.Account)
			return err
		}
	}
	if err := app.Config.Save(cfg); err != nil {
		rollbackSetup(app, cfg, request.Account, claudeEnabled)
		return err
	}
	if err := syncCodexProjection(ctx, app, cfg); err != nil {
		rollbackSetup(app, cfg, request.Account, claudeEnabled)
		return fmt.Errorf("客户端配置失败并已 rolled back：%w", err)
	}
	if err := bindCodexAuthentication(ctx, app, cfg); err != nil {
		rollbackSetup(app, cfg, request.Account, claudeEnabled)
		return fmt.Errorf("客户端认证失败并已 rolled back：%w", err)
	}

	r.Section("客户端")
	if cfg.Adapters[domain.ClientClaude].Enabled {
		r.Status(presentation.OK, "Claude", "配置完成")
	} else {
		r.Status(presentation.Info, "Claude", "未配置")
	}
	if cfg.Adapters[domain.ClientCodex].Enabled {
		r.Status(presentation.OK, "Codex", "配置完成")
	} else {
		r.Status(presentation.Info, "Codex", "未配置")
	}
	r.Success("已就绪；可添加同一 Account 下的其他模型 Profile")
	r.Next("aigw check")
	return nil
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
		return fmt.Errorf("网关拒绝认证（HTTP %d）", resp.StatusCode)
	}
	if resp.StatusCode >= 500 {
		return fmt.Errorf("网关暂时不可用（HTTP %d）", resp.StatusCode)
	}
	return nil
}

func rollbackSetup(app *App, cfg domain.Config, account string, claudeEnabled bool) {
	if adapter := cfg.Adapters[domain.ClientCodex]; adapter.Enabled {
		for _, target := range adapter.Targets {
			_ = adapters.DisableCodexConfig(target)
		}
	}
	if claudeEnabled {
		_ = app.Shims.DisableClaude()
	}
	_ = app.Secrets.Delete(account)
	_ = os.Remove(app.Config.Path())
	_ = os.Remove(app.Config.Path() + ".bak")
}
