package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/manifest"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/presentation"
)

func newProfileCommand(app *App) *cobra.Command {
	root := &cobra.Command{Use: "profile", Short: "管理服务 Profile"}
	root.AddCommand(newProfileListCommand(app), newProfileShowCommand(app), newProfileEditCommand(app), newProfileRenameCommand(app), newProfileRemoveCommand(app))
	return root
}

func newProfileListCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "List provider profiles", Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			names := sortedProfileNames(cfg)
			r := renderer(app)
			r.Title("AIGW", "服务列表")
			r.Section("Profiles")
			for _, name := range names {
				state := presentation.Info
				stateText := "可用"
				if name == cfg.Routes.Default {
					state = presentation.OK
					stateText = "当前"
				}
				accountName := cfg.Profiles[name].Account
				if accountName == "" {
					accountName = name
				}
				secret := "缺少 Token"
				if app.Secrets.Has(accountName) {
					secret = "Token 可用"
				}
				r.StatusLine(state, "Profile", name)
				r.Detail(cfg.Profiles[name].Label + " · " + stateText + " · Account " + accountName + " · " + secret)
			}
			r.Next("aigw use")
			return nil
		},
	}
}

func newProfileShowCommand(app *App) *cobra.Command {
	var jsonMode bool
	cmd := &cobra.Command{
		Use: "show <profile>", Short: "Show non-secret profile metadata", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			profile, ok := cfg.Profiles[args[0]]
			if !ok {
				return fmt.Errorf("unknown profile %q", args[0])
			}
			accountName := profile.Account
			if accountName == "" {
				accountName = args[0]
			}
			account := cfg.Accounts[accountName]
			if jsonMode {
				return json.NewEncoder(app.Out).Encode(map[string]any{"id": args[0], "label": profile.Label, "account": accountName, "models": profile.Models, "endpoints": account.Endpoints, "secret_available": app.Secrets.Has(accountName)})
			}
			r := renderer(app)
			r.Title("AIGW", "服务详情")
			r.Section("Profile")
			r.Row("ID", args[0])
			r.Row("名称", profile.Label)
			r.Row("Account", accountName)
			if profile.ModelFor(domain.ClientCodex) != "" {
				r.Row("Codex 模型", profile.ModelFor(domain.ClientCodex))
			}
			if profile.ModelFor(domain.ClientClaude) != "" {
				r.Row("Claude 模型", profile.ModelFor(domain.ClientClaude))
			}
			if account.Endpoints.OpenAIResponses != "" {
				r.Row("OpenAI", account.Endpoints.OpenAIResponses)
			}
			if account.Endpoints.Anthropic != "" {
				r.Row("Anthropic", account.Endpoints.Anthropic)
			}
			secretState := presentation.Warn
			secretText := "缺失"
			if app.Secrets.Has(accountName) {
				secretState = presentation.OK
				secretText = "可用"
			}
			r.Status(secretState, "系统密钥", secretText)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonMode, "json", false, "emit machine-readable JSON")
	return cmd
}

func newProfileEditCommand(app *App) *cobra.Command {
	var label string
	cmd := &cobra.Command{
		Use: "edit <profile>", Short: "Edit a runtime profile label", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if label == "" {
				return fmt.Errorf("nothing to edit; provide --label; use `aigw account edit <account>` for endpoints")
			}
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			before := cloneConfig(cfg)
			profile, ok := cfg.Profiles[args[0]]
			if !ok {
				return fmt.Errorf("unknown profile %q", args[0])
			}
			if label != "" {
				profile.Label = label
			}
			cfg.Profiles[args[0]] = profile
			if err := commitConfigAndSync(context.Background(), app, before, cfg, "profile"); err != nil {
				return err
			}
			r := renderer(app)
			r.Title("AIGW", "服务已更新")
			r.Row("Profile", args[0])
			r.Success("客户端配置已同步")
			r.Next("aigw check")
			return nil
		},
	}
	cmd.Flags().StringVar(&label, "label", "", "new display label")
	return cmd
}

func newProfileRenameCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use: "rename <old> <new>", Short: "Rename a runtime profile", Args: cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			oldName, newName := args[0], args[1]
			if !domain.ValidProfileName(newName) {
				return fmt.Errorf("invalid new profile name %q", newName)
			}
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			before := cloneConfig(cfg)
			profile, ok := cfg.Profiles[oldName]
			if !ok {
				return fmt.Errorf("unknown profile %q", oldName)
			}
			if _, exists := cfg.Profiles[newName]; exists {
				return fmt.Errorf("profile %q already exists", newName)
			}
			delete(cfg.Profiles, oldName)
			cfg.Profiles[newName] = profile
			if cfg.Routes.Default == oldName {
				cfg.Routes.Default = newName
			}
			for client, name := range cfg.Routes.Overrides {
				if name == oldName {
					cfg.Routes.Overrides[client] = newName
				}
			}
			if err := commitConfigAndSync(context.Background(), app, before, cfg, "profile rename"); err != nil {
				return err
			}
			r := renderer(app)
			r.Title("AIGW", "Profile 已重命名")
			r.Row("原 Profile", oldName)
			r.Row("新 Profile", newName)
			r.Row("Account", profile.Account)
			r.Success("Account Token 保持原位，路由已同步")
			return nil
		},
	}
}

func newProfileRemoveCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use: "remove <profile>", Short: "Remove an unused runtime profile", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			before := cloneConfig(cfg)
			name := args[0]
			profile, ok := cfg.Profiles[name]
			if !ok {
				return fmt.Errorf("unknown profile %q", name)
			}
			if cfg.Routes.Default == name {
				return fmt.Errorf("profile %q is active as the default route; switch with `aigw use <other>` first", name)
			}
			for client, route := range cfg.Routes.Overrides {
				if route == name {
					return fmt.Errorf("profile %q is active for %s; reset with `aigw route reset %s` first", name, client, client)
				}
			}
			delete(cfg.Profiles, name)
			if err := commitConfigAndSync(context.Background(), app, before, cfg, "profile remove"); err != nil {
				return err
			}
			r := renderer(app)
			r.Title("AIGW", "Profile 已移除")
			r.Row("Profile", name)
			r.Row("Account", profile.Account)
			r.Success("Account 与 Token 保持原位")
			return nil
		},
	}
}

func newRouteCommand(app *App) *cobra.Command {
	root := &cobra.Command{Use: "route", Short: "管理客户端路由"}
	root.AddCommand(
		&cobra.Command{Use: "list", Short: "List resolved routes", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error { return runStatus(cmd, app, false) }},
		&cobra.Command{Use: "reset <claude|codex>", Short: "Restore one client to default inheritance", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			client := args[0]
			if client != domain.ClientClaude && client != domain.ClientCodex {
				return fmt.Errorf("route must be claude or codex")
			}
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			before := cloneConfig(cfg)
			delete(cfg.Routes.Overrides, client)
			if err := commitConfigAndSync(cmd.Context(), app, before, cfg, "route reset"); err != nil {
				return err
			}
			r := renderer(app)
			r.Title("AIGW", "路由已重置")
			r.Row("客户端", title(client))
			r.Success("现在继承默认服务")
			r.Next("aigw check")
			return nil
		}},
	)
	return root
}

func newAdapterCommand(app *App) *cobra.Command {
	root := &cobra.Command{Use: "adapter", Short: "管理 Claude/Codex 适配器"}
	root.AddCommand(newAdapterListCommand(app), newAdapterDiscoverCommand(app), newAdapterEnableCommand(app), newAdapterAuthCommand(app), newAdapterDisableCommand(app))
	return root
}

func newAdapterListCommand(app *App) *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List adapter state", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		cfg, err := app.Config.Load()
		if err != nil {
			return err
		}
		r := renderer(app)
		r.Title("AIGW", "客户端适配")
		r.Section("Adapters")
		for _, name := range []string{domain.ClientClaude, domain.ClientCodex} {
			adapter := cfg.Adapters[name]
			state := presentation.Info
			stateText := "未启用"
			if adapter.Enabled {
				state = presentation.OK
				stateText = "已启用"
			}
			r.Status(state, title(name), stateText)
			if adapter.Executable != "" {
				r.Detail(adapter.Executable)
			}
		}
		return nil
	}}
}

func newAdapterDiscoverCommand(app *App) *cobra.Command {
	return &cobra.Command{Use: "discover", Short: "Find installed Claude and Codex executables", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		r := renderer(app)
		r.Title("AIGW", "客户端发现")
		r.Section("已安装客户端")
		for _, name := range []string{domain.ClientClaude, domain.ClientCodex} {
			path, err := exec.LookPath(name)
			if err != nil {
				r.Status(presentation.Info, title(name), "未发现")
				continue
			}
			r.Status(presentation.OK, title(name), path)
		}
		return nil
	}}
}

func newAdapterEnableCommand(app *App) *cobra.Command {
	var executable string
	var targets []string
	cmd := &cobra.Command{Use: "enable <claude|codex>", Short: "Enable a client adapter", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		client := args[0]
		if client != domain.ClientClaude && client != domain.ClientCodex {
			return fmt.Errorf("adapter must be claude or codex")
		}
		if executable == "" {
			return fmt.Errorf("--executable is required; run `aigw adapter discover`")
		}
		if client == domain.ClientCodex && len(targets) == 0 {
			return fmt.Errorf("Codex adapter needs at least one --target config.toml")
		}
		cfg, err := app.Config.Load()
		if err != nil {
			return err
		}
		before := cloneConfig(cfg)
		if current := before.Adapters[client]; current.Enabled {
			return fmt.Errorf("%s adapter is already enabled; disable it before changing its executable or targets", title(client))
		}
		runtime, _, err := cfg.ResolveRuntime(client, "")
		if err != nil {
			return err
		}
		if runtime.Endpoint == "" {
			return fmt.Errorf("profile %q has no %s endpoint", runtime.ProfileID, title(client))
		}
		accountName := runtime.AccountID
		if !app.Secrets.Has(accountName) {
			return fmt.Errorf("account %q has no token; repair with `aigw rotate %s`", accountName, accountName)
		}
		cfg.Adapters[client] = domain.AdapterConfig{Enabled: true, Executable: executable, Targets: append([]string(nil), targets...)}
		if client == domain.ClientClaude {
			if _, err := app.Shims.EnableClaude(); err != nil {
				return err
			}
		}
		if err := app.Config.Save(cfg); err != nil {
			if client == domain.ClientClaude {
				_ = app.Shims.DisableClaude()
			}
			return err
		}
		if err := syncAdapters(cmd.Context(), app, cfg); err == nil && client == domain.ClientCodex {
			err = bindCodexAuthentication(cmd.Context(), app, cfg)
		}
		if err != nil {
			_ = app.Config.Save(before)
			for _, target := range targets {
				_ = adapters.DisableCodexConfig(target)
			}
			if client == domain.ClientClaude {
				_ = app.Shims.DisableClaude()
			}
			return fmt.Errorf("adapter enable failed and was rolled back: %w", err)
		}
		r := renderer(app)
		r.Title("AIGW", "客户端已启用")
		r.Row("客户端", title(client))
		r.Status(presentation.OK, "适配器", "配置完成")
		r.Next("aigw check")
		return nil
	}}
	cmd.Flags().StringVar(&executable, "executable", "", "real client executable path")
	cmd.Flags().StringSliceVar(&targets, "target", nil, "client config path; repeat for multiple Codex homes")
	return cmd
}

func newAdapterAuthCommand(app *App) *cobra.Command {
	return &cobra.Command{Use: "auth codex", Short: "Bind the current Account Token to Codex", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if args[0] != domain.ClientCodex {
			return fmt.Errorf("native credential binding is only required for codex")
		}
		cfg, err := app.Config.Load()
		if err != nil {
			return err
		}
		if !cfg.Adapters[domain.ClientCodex].Enabled {
			return fmt.Errorf("Codex adapter is not enabled; run `aigw adapter enable codex ...` first")
		}
		if err := bindCodexAuthentication(cmd.Context(), app, cfg); err != nil {
			return fmt.Errorf("Codex authentication binding failed: %w", err)
		}
		r := renderer(app)
		r.Title("AIGW", "Codex 认证已绑定")
		r.Success("当前 Account Token 已写入 Codex 原生凭据存储")
		r.Next("aigw doctor")
		return nil
	}}
}

func newAdapterDisableCommand(app *App) *cobra.Command {
	return &cobra.Command{Use: "disable <claude|codex>", Short: "Disable a client adapter and remove owned projections", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		client := args[0]
		if client != domain.ClientClaude && client != domain.ClientCodex {
			return fmt.Errorf("adapter must be claude or codex")
		}
		cfg, err := app.Config.Load()
		if err != nil {
			return err
		}
		before := cloneConfig(cfg)
		adapter, ok := cfg.Adapters[client]
		if !ok || !adapter.Enabled {
			r := renderer(app)
			r.Title("AIGW", "客户端适配")
			r.Status(presentation.Info, title(client), "已经处于未启用状态")
			return nil
		}
		if client == domain.ClientCodex {
			for _, target := range adapter.Targets {
				if err := adapters.DisableCodexConfig(target); err != nil {
					return err
				}
			}
		}
		if client == domain.ClientClaude {
			if err := app.Shims.DisableClaude(); err != nil {
				return err
			}
		}
		delete(cfg.Adapters, client)
		if err := app.Config.Save(cfg); err != nil {
			if client == domain.ClientClaude {
				_, _ = app.Shims.EnableClaude()
			} else {
				_ = syncAdapters(context.Background(), app, before)
			}
			return err
		}
		r := renderer(app)
		r.Title("AIGW", "客户端已停用")
		r.Row("客户端", title(client))
		r.Success("AIGW 所有的投影已安全移除")
		return nil
	}}
}

func newConfigCommand(app *App) *cobra.Command {
	root := &cobra.Command{Use: "config", Short: "导入、导出与迁移配置"}
	root.AddCommand(
		&cobra.Command{Use: "path", Short: "Print the local config path", Args: cobra.NoArgs, Run: func(_ *cobra.Command, _ []string) { fmt.Fprintln(app.Out, app.Config.Path()) }},
		&cobra.Command{Use: "export", Short: "Export a secret-free team manifest", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			data, err := manifest.Export(cfg)
			if err != nil {
				return err
			}
			_, err = app.Out.Write(data)
			return err
		}},
		&cobra.Command{Use: "import <team-profiles.toml>", Short: "Merge a secret-free team manifest", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("read team manifest: %w", err)
			}
			team, err := manifest.Parse(data)
			if err != nil {
				return err
			}
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			before := cloneConfig(cfg)
			cfg, err = manifest.Merge(cfg, team)
			if err != nil {
				return err
			}
			if err := commitConfigAndSync(context.Background(), app, before, cfg, "team manifest"); err != nil {
				return err
			}
			accountNames := importedAccountNames(team)
			missing := []string{}
			r := renderer(app)
			r.Title("AIGW", "团队配置已导入")
			r.Row("服务数量", fmt.Sprintf("%d", len(team.Profiles)))
			r.Row("账户数量", fmt.Sprintf("%d", len(accountNames)))
			for _, name := range accountNames {
				if app.Secrets.Has(name) {
					r.Status(presentation.OK, "系统密钥", name+" Token 可用")
					continue
				}
				missing = append(missing, name)
				r.Status(presentation.Warn, name, "需要录入 Token")
			}
			if len(missing) > 0 {
				if len(missing) == 1 {
					r.Next("aigw rotate " + missing[0])
				} else {
					r.Next("aigw rotate <account>")
				}
			} else {
				r.Next("aigw models")
			}
			return nil
		}},
		&cobra.Command{Use: "migrate <legacy-config.json>", Short: "Migrate the pre-product AIGW JSON structure", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("read legacy config: %w", err)
			}
			cfg, err := manifest.MigrateLegacyV2(data)
			if err != nil {
				return err
			}
			if current, loadErr := app.Config.Load(); loadErr == nil && len(current.Profiles) > 0 {
				return fmt.Errorf("current AIGW config is not empty; refusing to overwrite %s", app.Config.Path())
			}
			if err := app.Config.Save(cfg); err != nil {
				return err
			}
			r := renderer(app)
			r.Title("AIGW", "旧配置已迁移")
			r.Row("服务数量", fmt.Sprintf("%d", len(cfg.Profiles)))
			r.Status(presentation.OK, "系统密钥", "保持原位，未复制到配置文件")
			r.Next("aigw repair")
			return nil
		}},
	)
	return root
}

func importedAccountNames(team manifest.Manifest) []string {
	seen := map[string]bool{}
	for name := range team.Accounts {
		seen[name] = true
	}
	for name, profile := range team.Profiles {
		accountName := profile.Account
		if accountName == "" {
			accountName = name
		}
		seen[accountName] = true
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func RunClaude(app *App, args []string) error {
	cfg, err := app.Config.Load()
	if err != nil {
		return err
	}
	adapter := cfg.Adapters[domain.ClientClaude]
	if !adapter.Enabled || adapter.Executable == "" {
		return fmt.Errorf("Claude adapter is disabled; run `aigw adapter enable claude --executable <real-claude>`")
	}
	runtime, _, err := cfg.ResolveRuntime(domain.ClientClaude, "")
	if err != nil {
		return err
	}
	accountName := runtime.AccountID
	token, err := app.Secrets.Get(accountName)
	if err != nil {
		return fmt.Errorf("Claude route token unavailable: %w; repair with `aigw rotate %s`", err, accountName)
	}
	plan, err := adapters.ClaudePlan(adapter.Executable, args, os.Environ(), runtime, token)
	if err != nil {
		return err
	}
	return app.Runner.Run(context.Background(), plan)
}

func sortedProfileNames(cfg domain.Config) []string {
	names := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedAccountNames(cfg domain.Config) []string {
	cfg.Normalize()
	names := make([]string, 0, len(cfg.Accounts))
	for name := range cfg.Accounts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func title(value string) string {
	if value == "" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
}
