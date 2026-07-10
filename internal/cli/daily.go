package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/presentation"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/secrets"
)

func newAddCommand(app *App) *cobra.Command {
	var label, openAIURL, anthropicURL string
	var tokenStdin bool
	cmd := &cobra.Command{
		Use:   "add <profile>",
		Short: "添加一个服务及其 Token",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			name := args[0]
			if !domain.ValidProfileName(name) {
				return fmt.Errorf("invalid profile name %q; use letters, numbers, dot, dash, or underscore", name)
			}
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			if _, exists := cfg.Profiles[name]; exists {
				return fmt.Errorf("profile %q already exists; use `aigw profile edit %s` or `aigw rotate %s`", name, name, name)
			}
			if label == "" {
				label = name
			}
			profile := domain.Profile{Label: label, Endpoints: domain.Endpoints{
				OpenAIResponses: strings.TrimRight(openAIURL, "/"),
				Anthropic:       strings.TrimRight(anthropicURL, "/"),
			}}
			token, err := app.readToken(tokenStdin, true)
			if err != nil {
				return err
			}
			cfg.Profiles[name] = profile
			if cfg.Routes.Default == "" {
				cfg.Routes.Default = name
			}
			if err := cfg.Validate(); err != nil {
				return err
			}
			if err := app.Secrets.Set(name, token); err != nil {
				return err
			}
			if err := app.Config.Save(cfg); err != nil {
				_ = app.Secrets.Delete(name)
				return err
			}
			r := renderer(app)
			r.Title("AIGW", "服务已添加")
			r.Section("服务")
			r.Row("名称", label)
			r.Row("Profile", name)
			r.Status(presentation.OK, "系统密钥", "已安全保存")
			r.Next("aigw check")
			return nil
		},
	}
	cmd.Flags().StringVar(&label, "label", "", "human-readable provider label")
	cmd.Flags().StringVar(&openAIURL, "openai-url", "", "OpenAI Responses base URL")
	cmd.Flags().StringVar(&anthropicURL, "anthropic-url", "", "Anthropic base URL")
	cmd.Flags().BoolVar(&tokenStdin, "token-stdin", false, "read one token line from stdin")
	return cmd
}

func newUseCommand(app *App) *cobra.Command {
	var client string
	var all bool
	cmd := &cobra.Command{
		Use:   "use <profile>",
		Short: "切换当前使用的 AI 服务",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if all && client != "" {
				return fmt.Errorf("--all and --for cannot be used together")
			}
			if client != "" && client != domain.ClientClaude && client != domain.ClientCodex {
				return fmt.Errorf("--for must be claude or codex")
			}
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			before := cloneConfig(cfg)
			name := ""
			if len(args) == 1 {
				name = args[0]
			} else {
				if !app.Interactive {
					return fmt.Errorf("profile is required outside an interactive terminal; run `aigw use <profile>`")
				}
				name, err = chooseProfile(app, cfg, "选择要使用的 AI 服务：")
				if err != nil {
					return err
				}
			}
			if _, ok := cfg.Profiles[name]; !ok {
				return fmt.Errorf("unknown profile %q; inspect `aigw profile list`", name)
			}
			addedToken := false
			if !app.Secrets.Has(name) {
				if !app.Interactive {
					return fmt.Errorf("profile %q has no token; repair with `aigw rotate %s`", name, name)
				}
				profile := cfg.Profiles[name]
				profile.ID = name
				token, err := app.Prompt.Secret("请粘贴 " + profile.Label + " Token：")
				if err != nil {
					return err
				}
				if err := verifyCredential(context.Background(), app, profile, token); err != nil {
					return fmt.Errorf("Token 验证失败：%w", err)
				}
				if err := app.Secrets.Set(name, token); err != nil {
					return err
				}
				addedToken = true
			}
			switch {
			case all:
				cfg.Routes.Default = name
				cfg.Routes.Overrides = map[string]string{}
			case client != "":
				cfg.Routes.Overrides[client] = name
			default:
				cfg.Routes.Default = name
			}
			if err := commitConfigAndSync(context.Background(), app, before, cfg, "route"); err != nil {
				if addedToken {
					_ = app.Secrets.Delete(name)
				}
				return err
			}
			r := renderer(app)
			r.Title("AIGW", "服务已切换")
			r.Section("当前选择")
			r.Row("服务", cfg.Profiles[name].Label)
			scope := "默认路由"
			if client != "" {
				scope = title(client)
			} else if all {
				scope = "全部客户端"
			}
			r.Row("作用范围", scope)
			r.Success("客户端配置已同步")
			r.Next("aigw check")
			return nil
		},
	}
	cmd.Flags().StringVar(&client, "for", "", "set only claude or codex")
	cmd.Flags().BoolVar(&all, "all", false, "set default and clear client overrides")
	return cmd
}

func newRotateCommand(app *App) *cobra.Command {
	var tokenStdin bool
	cmd := &cobra.Command{
		Use:   "rotate [profile]",
		Short: "更新当前服务的 Token",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			name := cfg.Routes.Default
			if len(args) == 1 {
				name = args[0]
			}
			if _, ok := cfg.Profiles[name]; !ok {
				return fmt.Errorf("unknown profile %q", name)
			}
			oldToken, oldErr := app.Secrets.Get(name)
			if oldErr != nil && !errors.Is(oldErr, secrets.ErrNotFound) {
				return oldErr
			}
			var token string
			if tokenStdin {
				token, err = app.readToken(true, false)
			} else if app.Interactive {
				token, err = app.Prompt.Secret("请粘贴 " + cfg.Profiles[name].Label + " Token：")
			} else {
				return fmt.Errorf("token input requires a terminal; pipe it to `aigw rotate %s --token-stdin`", name)
			}
			if err != nil {
				return err
			}
			profile := cfg.Profiles[name]
			profile.ID = name
			if err := verifyCredential(context.Background(), app, profile, token); err != nil {
				return fmt.Errorf("Token 验证失败：%w", err)
			}
			if err := app.Secrets.Set(name, token); err != nil {
				return err
			}
			if err := syncAdapters(context.Background(), app, cfg); err != nil {
				var rollbackErr error
				if errors.Is(oldErr, secrets.ErrNotFound) {
					rollbackErr = app.Secrets.Delete(name)
				} else {
					rollbackErr = app.Secrets.Set(name, oldToken)
				}
				if rollbackErr == nil {
					rollbackErr = syncAdapters(context.Background(), app, cfg)
				}
				if rollbackErr != nil {
					return fmt.Errorf("token sync failed: %w; rollback also failed: %v", err, rollbackErr)
				}
				return fmt.Errorf("token sync failed and was rolled back: %w", err)
			}
			r := renderer(app)
			r.Title("AIGW", "Token 已更新")
			r.Section("服务")
			r.Row("名称", cfg.Profiles[name].Label)
			r.Status(presentation.OK, "Token", "验证通过并已安全保存")
			r.Success("客户端认证已同步")
			r.Next("aigw check")
			return nil
		},
	}
	cmd.Flags().BoolVar(&tokenStdin, "token-stdin", false, "read one token line from stdin")
	return cmd
}

func chooseProfile(app *App, cfg domain.Config, label string) (string, error) {
	names := sortedProfileNames(cfg)
	choices := make([]Choice, 0, len(names))
	for _, name := range names {
		choices = append(choices, Choice{Value: name, Label: cfg.Profiles[name].Label})
	}
	return app.Prompt.Select(label, choices)
}

type routeStatus struct {
	Profile         string `json:"profile,omitempty"`
	Inherited       bool   `json:"inherited"`
	SecretAvailable bool   `json:"secret_available"`
	EndpointReady   bool   `json:"endpoint_ready"`
}

type statusOutput struct {
	ConfigPath string                 `json:"config_path"`
	Default    string                 `json:"default,omitempty"`
	Routes     map[string]routeStatus `json:"routes"`
	Profiles   int                    `json:"profiles"`
}

func newStatusCommand(app *App) *cobra.Command {
	var jsonMode bool
	cmd := &cobra.Command{Use: "status", Short: "查看详细状态", Args: cobra.NoArgs}
	cmd.RunE = func(cmd *cobra.Command, _ []string) error { return runStatus(cmd, app, jsonMode) }
	cmd.Flags().BoolVar(&jsonMode, "json", false, "emit machine-readable JSON")
	return cmd
}

func runStatus(_ *cobra.Command, app *App, jsonMode bool) error {
	cfg, err := app.Config.Load()
	if err != nil {
		return err
	}
	result := statusOutput{ConfigPath: app.Config.Path(), Default: cfg.Routes.Default, Profiles: len(cfg.Profiles), Routes: map[string]routeStatus{}}
	for _, client := range []string{domain.ClientClaude, domain.ClientCodex} {
		profile, inherited, resolveErr := cfg.Resolve(client, "")
		if resolveErr != nil {
			result.Routes[client] = routeStatus{Inherited: true}
			continue
		}
		_, endpointErr := profile.EndpointFor(client)
		result.Routes[client] = routeStatus{Profile: profile.ID, Inherited: inherited, SecretAvailable: app.Secrets.Has(profile.ID), EndpointReady: endpointErr == nil}
	}
	if jsonMode {
		enc := json.NewEncoder(app.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}
	if len(cfg.Profiles) == 0 {
		r := renderer(app)
		r.Title("AIGW", "尚未配置")
		r.Section("开始使用")
		r.Text("在交互式终端直接运行 aigw 即可完成配置。")
		r.Next("aigw")
		return nil
	}
	r := renderer(app)
	r.Title("AIGW", "当前状态")
	r.Section("服务")
	current := cfg.Profiles[result.Default]
	r.Row("当前服务", current.Label)
	r.Row("Profile", result.Default)
	r.Row("已配置服务", fmt.Sprintf("%d", result.Profiles))
	r.Section("客户端")
	attention := false
	for _, client := range []string{domain.ClientClaude, domain.ClientCodex} {
		route := result.Routes[client]
		mode := "单独指定"
		if route.Inherited {
			mode = "继承默认"
		}
		readiness := route.Profile + " · " + mode + " · 已就绪"
		state := presentation.OK
		if !route.SecretAvailable || !route.EndpointReady {
			readiness = route.Profile + " · " + mode + " · 需要处理"
			state = presentation.Warn
			attention = true
		}
		r.Status(state, title(client), readiness)
	}
	r.Section("账户诊断")
	if current.AccountProbe != nil && app.Accounts.Has(result.Default) {
		r.Status(presentation.OK, "精确余额", "已启用")
	} else if current.AccountProbe != nil {
		r.Status(presentation.Warn, "精确余额", "未启用")
		r.Detail("aigw account connect")
	} else {
		r.Status(presentation.Info, "精确余额", "服务商未提供探针")
	}
	if attention {
		r.Next("aigw repair")
	} else {
		r.Next("aigw check")
	}
	return nil
}

func newTestCommand(app *App) *cobra.Command {
	var client, profileName string
	cmd := &cobra.Command{
		Use:   "test",
		Short: "测试当前服务端点",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			clients := []string{domain.ClientClaude, domain.ClientCodex}
			if client != "" {
				if client != domain.ClientClaude && client != domain.ClientCodex {
					return fmt.Errorf("--for must be claude or codex")
				}
				clients = []string{client}
			}
			checked := 0
			r := renderer(app)
			r.Title("AIGW", "连接测试")
			r.Section("端点")
			for _, target := range clients {
				profile, _, err := cfg.Resolve(target, profileName)
				if err != nil {
					return err
				}
				endpoint, err := profile.EndpointFor(target)
				if err != nil {
					if client == "" {
						continue
					}
					return err
				}
				ctx, cancel := context.WithTimeout(cmd.Context(), 12*time.Second)
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
				if err != nil {
					cancel()
					return err
				}
				token, err := app.Secrets.Get(profile.ID)
				if err != nil {
					cancel()
					return fmt.Errorf("profile %q token unavailable: %w; repair with `aigw rotate %s`", profile.ID, err, profile.ID)
				}
				req.Header.Set("Authorization", "Bearer "+token)
				resp, err := app.HTTP.Do(req)
				cancel()
				if err != nil {
					return fmt.Errorf("%s endpoint unreachable: %w", target, err)
				}
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
					return fmt.Errorf("%s authentication rejected (HTTP %d); repair with `aigw rotate %s`", target, resp.StatusCode, profile.ID)
				}
				if resp.StatusCode >= 500 {
					return fmt.Errorf("%s endpoint returned HTTP %d", target, resp.StatusCode)
				}
				r.Status(presentation.OK, title(target), fmt.Sprintf("%s · HTTP %d", profile.ID, resp.StatusCode))
				checked++
			}
			if checked == 0 {
				return fmt.Errorf("resolved profiles have no testable client endpoint")
			}
			r.Next("aigw check")
			return nil
		},
	}
	cmd.Flags().StringVar(&client, "for", "", "test only claude or codex")
	cmd.Flags().StringVar(&profileName, "profile", "", "test one profile without changing routes")
	return cmd
}

func newSyncCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "重新同步客户端配置",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			if err := syncAdapters(cmd.Context(), app, cfg); err != nil {
				return err
			}
			r := renderer(app)
			r.Title("AIGW", "同步完成")
			r.Success("客户端配置与认证已刷新")
			r.Next("aigw check")
			return nil
		},
	}
}

func syncAdapters(ctx context.Context, app *App, cfg domain.Config) error {
	if adapter := cfg.Adapters[domain.ClientCodex]; adapter.Enabled {
		profile, _, err := cfg.Resolve(domain.ClientCodex, "")
		if err != nil {
			return err
		}
		token, err := app.Secrets.Get(profile.ID)
		if err != nil {
			return fmt.Errorf("Codex route token unavailable: %w", err)
		}
		for _, target := range adapter.Targets {
			if err := adapters.SyncCodexConfig(target, profile); err != nil {
				return err
			}
			if adapter.Executable != "" && app.Runner != nil {
				plan, err := adapters.CodexLoginPlan(adapter.Executable, filepath.Dir(target), token)
				if err != nil {
					return err
				}
				if err := app.Runner.Run(ctx, plan); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func cloneConfig(cfg domain.Config) domain.Config {
	clone := cfg
	clone.Profiles = make(map[string]domain.Profile, len(cfg.Profiles))
	for name, profile := range cfg.Profiles {
		clone.Profiles[name] = profile
	}
	clone.Routes.Overrides = make(map[string]string, len(cfg.Routes.Overrides))
	for client, profile := range cfg.Routes.Overrides {
		clone.Routes.Overrides[client] = profile
	}
	clone.Adapters = make(map[string]domain.AdapterConfig, len(cfg.Adapters))
	for name, adapter := range cfg.Adapters {
		adapter.Targets = append([]string(nil), adapter.Targets...)
		clone.Adapters[name] = adapter
	}
	return clone
}

func rollbackConfigAndAdapters(ctx context.Context, app *App, before domain.Config) error {
	if err := app.Config.Save(before); err != nil {
		return err
	}
	return syncAdapters(ctx, app, before)
}

func commitConfigAndSync(ctx context.Context, app *App, before, after domain.Config, subject string) error {
	if err := app.Config.Save(after); err != nil {
		return err
	}
	if err := syncAdapters(ctx, app, after); err != nil {
		rollbackErr := rollbackConfigAndAdapters(ctx, app, before)
		if rollbackErr != nil {
			return fmt.Errorf("%s sync failed: %w; rollback also failed: %v", subject, err, rollbackErr)
		}
		return fmt.Errorf("%s sync failed and was rolled back: %w", subject, err)
	}
	return nil
}

func _processPlanCompileGuard(_ adapters.ProcessPlan) {}
