package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"time"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/catalog"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/presentation"
)

func runWizard(ctx context.Context, app *App) error {
	team, err := catalog.Team()
	if err != nil {
		return err
	}
	names := make([]string, 0, len(team.Profiles))
	for name := range team.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	choices := make([]Choice, 0, len(names))
	for _, name := range names {
		choices = append(choices, Choice{Value: name, Label: team.Profiles[name].Label})
	}
	selected := team.RecommendedDefault
	if selected == "" {
		selected, err = app.Prompt.Select("选择团队 AI 服务：", choices)
		if err != nil {
			return err
		}
	}
	profile := team.Profiles[selected]
	profile.ID = selected
	providerAccount := team.Accounts[profile.Account]
	providerAccount.ID = profile.Account
	r := renderer(app)
	r.Title("AIGW", "首次配置")
	r.Section("团队服务")
	r.Status(presentation.OK, "配置目录", "已载入")
	r.Row("服务", profile.Label)
	token, err := app.Prompt.Secret("请粘贴 " + profile.Label + " Token：")
	if err != nil {
		return err
	}
	if err := verifyCredential(ctx, app, providerAccount, token); err != nil {
		return fmt.Errorf("Token 验证失败：%w", err)
	}
	r.Section("验证")
	r.Status(presentation.OK, "API Token", "有效")

	discovered := app.Discovery.Discover()
	cfg := domain.NewConfig()
	cfg.Accounts = team.Accounts
	cfg.Profiles = team.Profiles
	cfg.Routes.Default = selected
	if discovered.ClaudeExecutable != "" && providerAccount.Endpoints.Anthropic != "" {
		cfg.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true, Executable: discovered.ClaudeExecutable}
	}
	if discovered.CodexExecutable != "" && len(discovered.CodexTargets) > 0 && providerAccount.Endpoints.OpenAIResponses != "" {
		cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: discovered.CodexExecutable, Targets: discovered.CodexTargets}
	}
	r.Section("客户端")
	if cfg.Adapters[domain.ClientClaude].Enabled {
		r.Status(presentation.OK, "Claude", "已发现")
	} else {
		r.Status(presentation.Info, "Claude", "未发现")
	}
	if cfg.Adapters[domain.ClientCodex].Enabled {
		r.Status(presentation.OK, "Codex", fmt.Sprintf("已发现 · %d 个配置位置", len(discovered.CodexTargets)))
	} else {
		r.Status(presentation.Info, "Codex", "未发现")
	}

	if err := app.Secrets.Set(providerAccount.ID, token); err != nil {
		return err
	}
	claudeEnabled := cfg.Adapters[domain.ClientClaude].Enabled
	if claudeEnabled {
		if _, err := app.Shims.EnableClaude(); err != nil {
			_ = app.Secrets.Delete(providerAccount.ID)
			return err
		}
	}
	if err := app.Config.Save(cfg); err != nil {
		rollbackWizard(app, cfg, providerAccount.ID, claudeEnabled)
		return err
	}
	if err := syncAdapters(ctx, app, cfg); err != nil {
		rollbackWizard(app, cfg, providerAccount.ID, claudeEnabled)
		return fmt.Errorf("客户端配置失败并已 rolled back：%w", err)
	}
	if err := bindCodexAuthentication(ctx, app, cfg); err != nil {
		rollbackWizard(app, cfg, providerAccount.ID, claudeEnabled)
		return fmt.Errorf("客户端认证失败并已 rolled back：%w", err)
	}
	r.Section("完成")
	if claudeEnabled {
		r.Status(presentation.OK, "Claude", "配置完成")
	}
	if cfg.Adapters[domain.ClientCodex].Enabled {
		r.Status(presentation.OK, "Codex", "配置完成")
	}
	r.Row("默认服务", profile.Label)
	r.Success("已就绪，可以直接使用 claude 或 codex")
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

func rollbackWizard(app *App, cfg domain.Config, profile string, claudeEnabled bool) {
	if adapter := cfg.Adapters[domain.ClientCodex]; adapter.Enabled {
		for _, target := range adapter.Targets {
			_ = adapters.DisableCodexConfig(target)
		}
	}
	if claudeEnabled {
		_ = app.Shims.DisableClaude()
	}
	_ = app.Secrets.Delete(profile)
	_ = os.Remove(app.Config.Path())
	_ = os.Remove(app.Config.Path() + ".bak")
}
