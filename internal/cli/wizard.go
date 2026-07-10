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
	if selected == "" || len(choices) > 1 {
		selected, err = app.Prompt.Select("选择团队 AI 服务：", choices)
		if err != nil {
			return err
		}
	}
	profile := team.Profiles[selected]
	profile.ID = selected
	fmt.Fprintf(app.Out, "欢迎使用 AIGW\n\n✓ 已载入团队配置：%s\n", profile.Label)
	token, err := app.Prompt.Secret("请粘贴 " + profile.Label + " Token：")
	if err != nil {
		return err
	}
	if err := verifyCredential(ctx, app, profile, token); err != nil {
		return fmt.Errorf("Token 验证失败：%w", err)
	}
	fmt.Fprintln(app.Out, "✓ Token 有效")

	discovered := app.Discovery.Discover()
	cfg := domain.NewConfig()
	cfg.Profiles = team.Profiles
	cfg.Routes.Default = selected
	if discovered.ClaudeExecutable != "" && profile.Endpoints.Anthropic != "" {
		cfg.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true, Executable: discovered.ClaudeExecutable}
		fmt.Fprintln(app.Out, "✓ 发现 Claude")
	}
	if discovered.CodexExecutable != "" && len(discovered.CodexTargets) > 0 && profile.Endpoints.OpenAIResponses != "" {
		cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: discovered.CodexExecutable, Targets: discovered.CodexTargets}
		fmt.Fprintf(app.Out, "✓ 发现 Codex（%d 个配置位置）\n", len(discovered.CodexTargets))
	}

	if err := app.Secrets.Set(selected, token); err != nil {
		return err
	}
	claudeEnabled := cfg.Adapters[domain.ClientClaude].Enabled
	if claudeEnabled {
		if _, err := app.Shims.EnableClaude(); err != nil {
			_ = app.Secrets.Delete(selected)
			return err
		}
	}
	if err := app.Config.Save(cfg); err != nil {
		rollbackWizard(app, cfg, selected, claudeEnabled)
		return err
	}
	if err := syncAdapters(ctx, app, cfg); err != nil {
		rollbackWizard(app, cfg, selected, claudeEnabled)
		return fmt.Errorf("客户端配置失败并已 rolled back：%w", err)
	}
	if claudeEnabled {
		fmt.Fprintln(app.Out, "✓ Claude 配置完成")
	}
	if cfg.Adapters[domain.ClientCodex].Enabled {
		fmt.Fprintln(app.Out, "✓ Codex 配置完成")
	}
	fmt.Fprintf(app.Out, "✓ 默认服务：%s\n\n已就绪。现在可以直接使用 claude 或 codex。\n", profile.Label)
	return nil
}

func verifyCredential(ctx context.Context, app *App, profile domain.Profile, token string) error {
	endpoint := profile.Endpoints.OpenAIResponses
	if endpoint == "" {
		endpoint = profile.Endpoints.Anthropic
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
