package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/account"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/diagnostics"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/presentation"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/providers"
)

func newCheckCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use: "check", Short: "一键检查配置、Token、客户端与网关", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := app.Config.Load()
			if err != nil {
				return fmt.Errorf("配置无效：%w\n修复：aigw repair", err)
			}
			if len(cfg.Profiles) == 0 {
				return problem("尚未配置", "尚未创建任何服务 Profile。", "无法检查、同步或修复尚不存在的配置。", "aigw setup", fmt.Errorf("not configured"))
			}
			runtime, err := firstCheckRuntime(cfg)
			if err != nil {
				return err
			}
			accountName := runtime.AccountID
			providerAccount := cfg.Accounts[accountName]
			token, err := app.Secrets.Get(accountName)
			if err != nil {
				return fmt.Errorf("系统密钥缺失\n修复：aigw rotate %s", accountName)
			}
			r := renderer(app)
			r.Title("AIGW", "健康检查")
			r.Section("配置")
			r.Status(presentation.OK, "配置文件", "正常")
			r.Row("当前服务", runtime.ProfileLabel)
			r.Status(presentation.OK, "系统密钥", "可用")
			r.Section("客户端")
			clientCount := 0
			for _, client := range domain.AdmittedClientIDs() {
				adapter := cfg.Adapters[client]
				if adapter.Enabled {
					clientRuntime, _, resolveErr := cfg.ResolveRuntime(client, "")
					if resolveErr != nil {
						return problem(title(client)+" 路由未解析", resolveErr.Error(), title(client)+" 无法确定应使用的 Profile。", "aigw use <profile> --for "+client, resolveErr)
					}
					ready, issue := adapterRouteReady(app, cfg, client, clientRuntime)
					if !ready {
						fix := "aigw repair"
						impact := title(client) + " 无法继承 AIGW 的路由、Token 或配置投影。"
						if client == domain.ClientCodex && strings.Contains(issue, "投影") {
							fix = "aigw sync"
							impact = "Codex 可能使用错误模型或端点。"
						}
						return problem(title(client)+" 适配器未就绪", issue, impact, fix, fmt.Errorf("%s adapter not ready", client))
					}
					r.Status(presentation.OK, title(client), "已就绪")
					clientCount++
				} else {
					r.Status(presentation.Info, title(client), "未启用")
				}
			}
			result := diagnostics.Probe(cmd.Context(), app.HTTP, runtime, token)
			if result.Kind != diagnostics.Healthy {
				evidence := result.Detail
				if result.HTTPStatus != 0 {
					evidence = fmt.Sprintf("HTTP %d", result.HTTPStatus)
					if result.Detail != "" {
						evidence += " · " + result.Detail
					}
				}
				return problem(result.Summary, evidence, healthImpact(clientCount), result.Fix, fmt.Errorf("diagnostic kind %s", result.Kind))
			}
			r.Section("网关")
			r.Status(presentation.OK, "API Token", "认证正常")
			if providerAccount.AccountProbe != nil && providers.Supports(providerAccount.AccountProbe.Kind) && app.Accounts.Has(accountName) {
				r.Status(presentation.OK, "精确余额", "已启用")
			} else if providerAccount.AccountProbe != nil && providers.Supports(providerAccount.AccountProbe.Kind) {
				r.Status(presentation.Warn, "精确余额", "未启用")
				r.Detail("aigw account connect " + accountName)
			} else if providerAccount.AccountProbe != nil {
				r.Status(presentation.Info, "精确余额", "当前版本未提供此服务商诊断")
			}
			r.Section("结果")
			r.Success("一切正常")
			if providerAccount.AccountProbe != nil && providers.Supports(providerAccount.AccountProbe.Kind) {
				r.Next("aigw balance " + accountName)
			}
			return nil
		},
	}
}

func newAccountCommand(app *App) *cobra.Command {
	root := &cobra.Command{Use: "account", Short: "管理 Account 端点与可选精确诊断"}
	root.AddCommand(
		newAccountEditCommand(app),
		&cobra.Command{Use: "connect [account]", Short: "Bind provider platform credentials for exact balance", Args: cobra.MaximumNArgs(1), RunE: func(_ *cobra.Command, args []string) error {
			if !app.Interactive {
				return fmt.Errorf("account connection requires an interactive terminal")
			}
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			name := cfg.Routes.Default
			if len(args) == 1 {
				name = args[0]
			}
			accountName, providerAccount, err := accountForInput(cfg, name)
			if err != nil {
				return err
			}
			if providerAccount.AccountProbe == nil {
				return fmt.Errorf("account %q does not support exact account diagnostics", accountName)
			}
			if !providers.Supports(providerAccount.AccountProbe.Kind) {
				return fmt.Errorf("exact diagnostics provider %q is not included in this AIGW build", providerAccount.AccountProbe.Kind)
			}
			systemToken, err := app.Prompt.Secret("请粘贴平台系统令牌（不是 API Token）：")
			if err != nil {
				return err
			}
			userID, err := app.Prompt.Text("用户 ID：")
			if err != nil {
				return err
			}
			if err := app.Accounts.Set(accountName, account.Credential{SystemToken: systemToken, UserID: userID}); err != nil {
				return err
			}
			r := renderer(app)
			r.Title("AIGW", "账户诊断已启用")
			r.Section("服务")
			r.Row("名称", providerAccount.Label)
			r.Status(presentation.OK, "系统凭据", "已安全保存")
			r.Next("aigw balance")
			return nil
		}},
		&cobra.Command{Use: "disconnect [account]", Short: "Remove optional provider platform credentials", Args: cobra.MaximumNArgs(1), RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			name := cfg.Routes.Default
			if len(args) == 1 {
				name = args[0]
			}
			accountName, _, err := accountForInput(cfg, name)
			if err != nil {
				return err
			}
			if err := app.Accounts.Delete(accountName); err != nil {
				return err
			}
			r := renderer(app)
			r.Title("AIGW", "账户诊断已关闭")
			r.Success("平台系统凭据已从安全存储移除")
			return nil
		}},
	)
	return root
}

func newBalanceCommand(app *App) *cobra.Command {
	return &cobra.Command{Use: "balance [account]", Short: "查看账户余额与 Token 额度", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := app.Config.Load()
		if err != nil {
			return err
		}
		name := cfg.Routes.Default
		if len(args) == 1 {
			name = args[0]
		}
		accountName, providerAccount, err := accountForInput(cfg, name)
		if err != nil {
			return err
		}
		if providerAccount.AccountProbe == nil {
			return fmt.Errorf("%s 暂不支持精确余额查询", providerAccount.Label)
		}
		if !providers.Supports(providerAccount.AccountProbe.Kind) {
			return fmt.Errorf("精确诊断服务商 %q 未包含在当前 AIGW 版本中；可继续使用 `aigw check` 获取通用诊断", providerAccount.AccountProbe.Kind)
		}
		credential, err := app.Accounts.Get(accountName)
		if err != nil {
			return problem(
				"精确余额诊断尚未启用",
				"缺少 "+accountName+" 的服务商平台查询凭据；API Token 已单独保存在系统密钥存储。",
				"无法区分账户余额、Token 剩余额度、Token 禁用状态和次数限制。",
				"aigw account connect "+accountName,
				err,
			)
		}
		apiToken, err := app.Secrets.Get(accountName)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		defer cancel()
		providerAccount.ID = accountName
		report, err := providers.Probe(ctx, app.HTTP, providerAccount, apiToken, credential)
		if err != nil {
			return err
		}
		status := map[string]string{"enabled": "启用", "disabled": "禁用"}[report.TokenStatus]
		remaining := fmt.Sprintf("￥%.4f", report.TokenRemaining)
		if report.TokenUnlimitedQuota {
			remaining = "无限额度"
		}
		count := fmt.Sprintf("%d 次", report.TokenRemainingCount)
		if report.TokenUnlimitedCount {
			count = "无限次数"
		}
		r := renderer(app)
		r.Title("AIGW", "账户与额度")
		r.Section("账户")
		r.Row("账户", providerAccount.Label)
		r.Row("账户余额", fmt.Sprintf("￥%.4f", report.AccountBalance))
		r.Section("当前 API Token")
		r.Row("名称", report.TokenName)
		state := presentation.OK
		if report.TokenStatus != "enabled" {
			state = presentation.Fail
		}
		r.Status(state, "Token 状态", status)
		r.Row("已用额度", fmt.Sprintf("￥%.4f", report.TokenUsed))
		r.Row("剩余额度", remaining)
		r.Row("剩余次数", count)
		r.Next("aigw check")
		return nil
	}}
}

func newRepairCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use: "repair", Short: "自动发现并修复客户端配置", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			if len(cfg.Profiles) == 0 {
				return problem("尚未配置", "尚未创建任何服务 Profile。", "无法检查、同步或修复尚不存在的配置。", "aigw setup", fmt.Errorf("not configured"))
			}
			before := cloneConfig(cfg)
			discovered := app.Discovery.Discover()
			claudeRuntime, _, claudeRouteErr := cfg.ResolveRuntime(domain.ClientClaude, "")
			codexRuntime, _, codexRouteErr := cfg.ResolveRuntime(domain.ClientCodex, "")
			newClaude := false
			claudeAdapter := cfg.Adapters[domain.ClientClaude]
			claudeExecutable := claudeAdapter.Executable
			if claudeExecutable == "" {
				claudeExecutable = discovered.ClaudeExecutable
			}
			if claudeRouteErr == nil && claudeExecutable != "" && claudeRuntime.Endpoint != "" {
				if !claudeAdapter.Enabled {
					newClaude = true
				}
				cfg.Adapters[domain.ClientClaude] = domain.AdapterConfig{Enabled: true, Executable: claudeExecutable}
				if _, err := app.Shims.EnableClaude(); err != nil {
					return err
				}
			}
			newCodexTargets := []string{}
			if codexRouteErr == nil && discovered.CodexExecutable != "" && len(discovered.CodexTargets) > 0 && codexRuntime.Endpoint != "" {
				cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Executable: discovered.CodexExecutable, Targets: discovered.CodexTargets}
				newCodexTargets = discovered.CodexTargets
			}
			if err := commitConfigAndSync(cmd.Context(), app, before, cfg, "repair"); err != nil {
				for _, target := range newCodexTargets {
					if !contains(before.Adapters[domain.ClientCodex].Targets, target) {
						_ = adapters.DisableCodexConfig(target)
					}
				}
				if newClaude {
					_ = app.Shims.DisableClaude()
				}
				return err
			}
			if cfg.Adapters[domain.ClientCodex].Enabled && !codexProjectionChanged(before, cfg) {
				if err := syncCodexProjection(cmd.Context(), app, cfg); err != nil {
					return fmt.Errorf("repair Codex projection: %w", err)
				}
			}
			r := renderer(app)
			r.Title("AIGW", "修复完成")
			r.Section("处理结果")
			r.Status(presentation.OK, "客户端", "已重新发现")
			r.Status(presentation.OK, "配置", "已同步")
			authentication := "未改动"
			if codexAuthenticationChanged(before, cfg) {
				authentication = "已绑定"
			}
			r.Status(presentation.OK, "认证", authentication)
			r.Next("aigw check")
			return nil
		},
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func newUpdateCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use: "update", Short: "更新到团队最新版本", Args: cobra.NoArgs,
		RunE: func(ctx *cobra.Command, _ []string) error {
			if app.Updater == nil {
				return fmt.Errorf("自动更新不可用；请从团队 GitLab Release 安装最新版")
			}
			result, err := app.Updater.Update(ctx.Context(), Version)
			if err != nil {
				return err
			}
			r := renderer(app)
			r.Title("AIGW", "更新")
			r.Success(result)
			r.Next("aigw check")
			return nil
		},
	}
}

func firstCheckRuntime(cfg domain.Config) (domain.Runtime, error) {
	profile, ok := cfg.Profiles[cfg.Routes.Default]
	if !ok {
		return domain.Runtime{}, fmt.Errorf("当前默认路由没有可测试端点；运行 `aigw use` 选择模型 Profile")
	}
	client := profile.Client
	if client == "" {
		for _, candidate := range domain.AdmittedClientIDs() {
			if _, _, err := cfg.ResolveRuntime(candidate, cfg.Routes.Default); err == nil {
				client = candidate
				break
			}
		}
	}
	if client == "" {
		return domain.Runtime{}, fmt.Errorf("当前默认路由没有可测试端点；运行 `aigw use` 选择模型 Profile")
	}
	runtime, _, err := cfg.ResolveRuntime(client, cfg.Routes.Default)
	if err != nil {
		return domain.Runtime{}, fmt.Errorf("当前默认路由没有可测试端点；运行 `aigw use` 选择模型 Profile")
	}
	return runtime, nil
}
