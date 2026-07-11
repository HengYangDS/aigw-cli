package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
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
		RunE: func(cmd *cobra.Command, args []string) error {
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
			account := domain.Account{Label: label, Endpoints: domain.Endpoints{
				OpenAIResponses: strings.TrimRight(openAIURL, "/"),
				Anthropic:       strings.TrimRight(anthropicURL, "/"),
			}}
			profile := domain.Profile{Label: label, Account: name}
			token, err := app.readToken(tokenStdin, true)
			if err != nil {
				return err
			}
			cfg.Accounts[name] = account
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
		RunE: func(cmd *cobra.Command, args []string) error {
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
			accountName, providerAccount, err := accountForInput(cfg, name)
			if err != nil {
				return err
			}
			addedToken := false
			if !app.Secrets.Has(accountName) {
				if !app.Interactive {
					return fmt.Errorf("account %q has no token; repair with `aigw rotate %s`", accountName, accountName)
				}
				token, err := app.Prompt.Secret("请粘贴 " + providerAccount.Label + " Token：")
				if err != nil {
					return err
				}
				providerAccount.ID = accountName
				if err := verifyCredential(context.Background(), app, providerAccount, token); err != nil {
					return fmt.Errorf("Token 验证失败：%w", err)
				}
				if err := app.Secrets.Set(accountName, token); err != nil {
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
			if err := commitConfigAndSync(cmd.Context(), app, before, cfg, "route"); err != nil {
				if addedToken {
					_ = app.Secrets.Delete(accountName)
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
		Use:   "rotate [account]",
		Short: "更新当前 Account 的 Token",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			name := cfg.Routes.Default
			if len(args) == 1 {
				name = args[0]
			}
			accountName, account, err := accountForInput(cfg, name)
			if err != nil {
				return err
			}
			oldToken, oldErr := app.Secrets.Get(accountName)
			if oldErr != nil && !errors.Is(oldErr, secrets.ErrNotFound) {
				return oldErr
			}
			var token string
			if tokenStdin {
				token, err = app.readToken(true, false)
			} else if app.Interactive {
				token, err = app.Prompt.Secret("请粘贴 " + account.Label + " Token：")
			} else {
				return fmt.Errorf("token input requires a terminal; pipe it to `aigw rotate %s --token-stdin`", accountName)
			}
			if err != nil {
				return err
			}
			account.ID = accountName
			if err := verifyCredential(context.Background(), app, account, token); err != nil {
				return fmt.Errorf("Token 验证失败：%w", err)
			}
			if err := app.Secrets.Set(accountName, token); err != nil {
				return err
			}
			if err := syncAdapters(cmd.Context(), app, cfg); err != nil {
				var rollbackErr error
				if errors.Is(oldErr, secrets.ErrNotFound) {
					rollbackErr = app.Secrets.Delete(accountName)
				} else {
					rollbackErr = app.Secrets.Set(accountName, oldToken)
				}
				if rollbackErr == nil {
					rollbackErr = syncAdapters(cmd.Context(), app, cfg)
				}
				if rollbackErr != nil {
					return fmt.Errorf("token sync failed: %w; rollback also failed: %v", err, rollbackErr)
				}
				return fmt.Errorf("token sync failed and was rolled back: %w", err)
			}
			if err := bindCodexAuthentication(cmd.Context(), app, cfg); err != nil {
				var rollbackErr error
				if errors.Is(oldErr, secrets.ErrNotFound) {
					rollbackErr = app.Secrets.Delete(accountName)
				} else {
					rollbackErr = app.Secrets.Set(accountName, oldToken)
				}
				if rollbackErr == nil {
					rollbackErr = syncAdapters(cmd.Context(), app, cfg)
					if rollbackErr == nil {
						rollbackErr = bindCodexAuthentication(cmd.Context(), app, cfg)
					}
				}
				if rollbackErr != nil {
					return fmt.Errorf("token authentication failed: %w; rollback also failed: %v", err, rollbackErr)
				}
				return fmt.Errorf("token authentication failed and was rolled back: %w", err)
			}
			r := renderer(app)
			r.Title("AIGW", "Token 已更新")
			r.Section("服务")
			r.Row("账户", account.Label)
			r.Row("Account", accountName)
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
	Profile          string `json:"profile,omitempty"`
	Inherited        bool   `json:"inherited"`
	SecretAvailable  bool   `json:"secret_available"`
	EndpointReady    bool   `json:"endpoint_ready"`
	AdapterReady     bool   `json:"adapter_ready"`
	AdapterIssue     string `json:"adapter_issue,omitempty"`
	NeedsSelection   bool   `json:"needs_selection,omitempty"`
	SuggestedProfile string `json:"suggested_profile,omitempty"`
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
		runtime, inherited, resolveErr := cfg.ResolveRuntime(client, "")
		if resolveErr != nil {
			suggested := firstProfileForClient(cfg, client)
			result.Routes[client] = routeStatus{Inherited: true, NeedsSelection: suggested != "", SuggestedProfile: suggested}
			continue
		}
		adapterReady, adapterIssue := adapterRouteReady(app, cfg, client, runtime)
		result.Routes[client] = routeStatus{Profile: runtime.ProfileID, Inherited: inherited, SecretAvailable: app.Secrets.Has(runtime.AccountID), EndpointReady: runtime.Endpoint != "", AdapterReady: adapterReady, AdapterIssue: adapterIssue}
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
	accountName := current.Account
	account := cfg.Accounts[accountName]
	r.Row("当前 Profile", current.Label)
	r.Row("Profile", result.Default)
	r.Row("Account", accountName)
	if current.ModelFor(domain.ClientCodex) != "" {
		r.Row("Codex 模型", current.ModelFor(domain.ClientCodex))
	}
	if current.ModelFor(domain.ClientClaude) != "" {
		r.Row("Claude 模型", current.ModelFor(domain.ClientClaude))
	}
	r.Row("已配置服务", fmt.Sprintf("%d", result.Profiles))
	r.Section("客户端")
	attention := false
	selectionCommand := ""
	for _, client := range []string{domain.ClientClaude, domain.ClientCodex} {
		route := result.Routes[client]
		if route.NeedsSelection {
			state := presentation.Warn
			message := "未选择 " + title(client) + " Profile"
			if route.SuggestedProfile != "" {
				cmd := "aigw use " + route.SuggestedProfile + " --for " + client
				message += " · " + cmd
				if selectionCommand == "" {
					selectionCommand = cmd
				}
			}
			r.Status(state, title(client), message)
			attention = true
			continue
		}
		mode := "单独指定"
		if route.Inherited {
			mode = "继承默认"
		}
		readiness := route.Profile + " · " + mode + " · 已就绪"
		state := presentation.OK
		if !route.SecretAvailable || !route.EndpointReady || !route.AdapterReady {
			readiness = route.Profile + " · " + mode + " · 需要处理"
			if route.AdapterIssue != "" {
				readiness = route.Profile + " · " + mode + " · " + route.AdapterIssue
			}
			state = presentation.Warn
			attention = true
		}
		r.Status(state, title(client), readiness)
	}
	r.Section("账户诊断")
	if account.AccountProbe != nil && app.Accounts.Has(accountName) {
		r.Status(presentation.OK, "精确余额", "已启用")
	} else if account.AccountProbe != nil {
		r.Status(presentation.Warn, "精确余额", "未启用")
		r.Detail("aigw account connect " + accountName)
	} else {
		r.Status(presentation.Info, "精确余额", "服务商未提供探针")
	}
	if selectionCommand != "" {
		r.Next(selectionCommand)
	} else if attention {
		r.Next("aigw repair")
	} else {
		r.Next("aigw check")
	}
	return nil
}

// adapterRouteReady checks all local conditions that make an enabled adapter
// usable by the selected route. It is deliberately read-only and never starts
// or reloads a client process.
func adapterRouteReady(app *App, cfg domain.Config, client string, runtime domain.Runtime) (bool, string) {
	adapter := cfg.Adapters[client]
	if !adapter.Enabled {
		return false, title(client) + " adapter 未启用"
	}
	if adapter.Executable == "" {
		return false, title(client) + " 可执行文件未配置"
	}
	switch client {
	case domain.ClientClaude:
		ready, err := app.Shims.ClaudeShimReady()
		if err != nil {
			return false, "Claude shim 无法读取"
		}
		if !ready {
			return false, "Claude shim 缺失"
		}
		active, err := app.Shims.ClaudeActivationReady()
		if err != nil {
			return false, "Claude PATH 激活无法读取"
		}
		if !active {
			return false, "Claude PATH 激活缺失"
		}
	case domain.ClientCodex:
		if len(adapter.Targets) == 0 {
			return false, "Codex 配置目标缺失"
		}
		for _, target := range adapter.Targets {
			if err := adapters.ValidateCodexConfig(target, runtime); err != nil {
				return false, "Codex 配置投影漂移：" + err.Error()
			}
		}
	}
	return true, ""
}

func codexModelsEndpoint(endpoint string) string {
	endpoint = strings.TrimRight(endpoint, "/")
	if strings.HasSuffix(endpoint, "/models") {
		return endpoint
	}
	return endpoint + "/models"
}

func firstProfileForClient(cfg domain.Config, client string) string {
	for _, name := range sortedProfileNames(cfg) {
		profile := cfg.Profiles[name]
		if profile.Client != "" && profile.Client != client {
			continue
		}
		if profile.ModelFor(client) != "" {
			return name
		}
		if account, ok := cfg.Accounts[profile.Account]; ok {
			if _, err := account.EndpointFor(client); err == nil {
				return name
			}
		}
	}
	return ""
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
				runtime, _, err := cfg.ResolveRuntime(target, profileName)
				if err != nil {
					return err
				}
				endpoint := runtime.Endpoint
				if endpoint == "" {
					if client == "" {
						continue
					}
					return fmt.Errorf("profile %q has no %s endpoint", runtime.ProfileID, title(target))
				}
				testURL := endpoint
				if target == domain.ClientCodex {
					testURL = codexModelsEndpoint(endpoint)
				}
				ctx, cancel := context.WithTimeout(cmd.Context(), 12*time.Second)
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, testURL, nil)
				if err != nil {
					cancel()
					return err
				}
				accountName := runtime.AccountID
				token, err := app.Secrets.Get(accountName)
				if err != nil {
					cancel()
					return fmt.Errorf("account %q token unavailable: %w; repair with `aigw rotate %s`", accountName, err, accountName)
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
					return fmt.Errorf("%s authentication rejected (HTTP %d); repair with `aigw rotate %s`", target, resp.StatusCode, accountName)
				}
				if resp.StatusCode < 200 || resp.StatusCode >= 400 {
					return fmt.Errorf("%s endpoint returned HTTP %d", target, resp.StatusCode)
				}
				r.Status(presentation.OK, title(target), fmt.Sprintf("%s · HTTP %d", runtime.ProfileID, resp.StatusCode))
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

func newVerifyCommand(app *App) *cobra.Command {
	var client, profileName string
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "执行一次最小真实请求，验证模型协议链路",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			clients := []string{}
			switch client {
			case domain.ClientClaude, domain.ClientCodex:
				clients = []string{client}
			case "all":
				if profileName != "" {
					return fmt.Errorf("--profile cannot be used with --for all")
				}
				clients = []string{domain.ClientClaude, domain.ClientCodex}
			default:
				return fmt.Errorf("--for must be claude, codex, or all")
			}
			cfg, err := app.Config.Load()
			if err != nil {
				return err
			}
			if client == "all" {
				if err := validateFullVerificationReadiness(app, cfg); err != nil {
					return err
				}
			}
			r := renderer(app)
			r.Title("AIGW", "真实协议验证")
			r.Section("最小请求")
			r.Detail("会产生一次最小模型请求；不会修改客户端配置或重启客户端。")
			for _, target := range clients {
				runtime, _, err := cfg.ResolveRuntime(target, profileName)
				if err != nil {
					return err
				}
				accountName := runtime.AccountID
				token, err := app.Secrets.Get(accountName)
				if err != nil {
					return fmt.Errorf("account %q token unavailable: %w; repair with `aigw rotate %s`", accountName, err, accountName)
				}
				ctx, cancel := context.WithTimeout(cmd.Context(), 25*time.Second)
				if target == domain.ClientCodex {
					err = verifyCodexResponse(ctx, app, runtime, token)
				} else {
					err = verifyClaudeInvocation(ctx, app, cfg, runtime, token)
				}
				cancel()
				if err != nil {
					return err
				}
				r.Status(presentation.OK, title(target), runtime.ProfileID+" · 已完成")
			}
			if client == "all" {
				if err := app.Config.SaveVerifiedCheckpoint(cfg, clients); err != nil {
					return err
				}
				r.Detail("已更新最近一次全链路验证检查点。")
			}
			r.Next("aigw doctor")
			return nil
		},
	}
	cmd.Flags().StringVar(&client, "for", "", "verify claude, codex, or all")
	cmd.Flags().StringVar(&profileName, "profile", "", "verify one profile without changing routes")
	return cmd
}

const verificationSentinel = "AIGW_OK"
const verificationResponseLimit = 256 * 1024

type verificationResponse struct {
	Status     string `json:"status"`
	OutputText string `json:"output_text"`
	Output     []struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
}

func validateFullVerificationReadiness(app *App, cfg domain.Config) error {
	claude := cfg.Adapters[domain.ClientClaude]
	if !claude.Enabled || claude.Executable == "" {
		return fmt.Errorf("full verification requires an enabled Claude adapter; run `aigw repair`")
	}
	ready, err := app.Shims.ClaudeShimReady()
	if err != nil {
		return fmt.Errorf("inspect Claude shim: %w", err)
	}
	if !ready {
		return fmt.Errorf("full verification requires the AIGW-managed Claude shim; run `aigw repair`")
	}
	codex := cfg.Adapters[domain.ClientCodex]
	if !codex.Enabled || codex.Executable == "" || len(codex.Targets) == 0 {
		return fmt.Errorf("full verification requires an enabled Codex adapter with at least one target; run `aigw repair`")
	}
	runtime, _, err := cfg.ResolveRuntime(domain.ClientCodex, "")
	if err != nil {
		return fmt.Errorf("resolve Codex route for full verification: %w", err)
	}
	for _, target := range codex.Targets {
		if err := adapters.ValidateCodexConfig(target, runtime); err != nil {
			return fmt.Errorf("full verification requires a synchronized Codex target %s: %w; run `aigw sync`", target, err)
		}
	}
	return nil
}

func verifyCodexResponse(ctx context.Context, app *App, runtime domain.Runtime, token string) error {
	endpoint := runtime.Endpoint
	model := runtime.Model
	if model == "" {
		return fmt.Errorf("profile %q has no Codex model", runtime.ProfileID)
	}
	body, err := json.Marshal(map[string]any{
		"model":             model,
		"input":             "Reply with exactly: AIGW_OK",
		"max_output_tokens": 16,
		"store":             false,
	})
	if err != nil {
		return fmt.Errorf("encode Codex verification request: %w", err)
	}
	requestURL := strings.TrimRight(endpoint, "/")
	if !strings.HasSuffix(requestURL, "/responses") {
		requestURL += "/responses"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("Codex model request failed: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, verificationResponseLimit+1))
	if err != nil {
		return fmt.Errorf("read Codex verification response: %w", err)
	}
	if len(responseBody) > verificationResponseLimit {
		return fmt.Errorf("Codex verification response exceeded %d bytes", verificationResponseLimit)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("Codex model authentication rejected (HTTP %d); repair with `aigw rotate %s`", resp.StatusCode, runtime.AccountID)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("Codex model request returned HTTP %d", resp.StatusCode)
	}
	if !hasVerificationSentinel(responseBody) {
		return fmt.Errorf("Codex model response did not return the required verification sentinel")
	}
	return nil
}

func hasVerificationSentinel(data []byte) bool {
	var response verificationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return false
	}
	if response.Status != "" && response.Status != "completed" {
		return false
	}
	if strings.TrimSpace(response.OutputText) == verificationSentinel {
		return true
	}
	for _, output := range response.Output {
		for _, content := range output.Content {
			if (content.Type == "output_text" || content.Type == "text") && strings.TrimSpace(content.Text) == verificationSentinel {
				return true
			}
		}
	}
	return false
}

func verifyClaudeInvocation(ctx context.Context, app *App, cfg domain.Config, runtime domain.Runtime, token string) error {
	adapter := cfg.Adapters[domain.ClientClaude]
	if !adapter.Enabled || adapter.Executable == "" {
		return fmt.Errorf("Claude adapter is not enabled; run `aigw repair`")
	}
	ready, err := app.Shims.ClaudeShimReady()
	if err != nil {
		return err
	}
	if !ready {
		return fmt.Errorf("Claude shim is missing; run `aigw repair`")
	}
	plan, err := adapters.ClaudePlan(adapter.Executable, []string{"--print", "Reply with exactly: AIGW_OK"}, os.Environ(), runtime, token)
	if err != nil {
		return err
	}
	// Verification must capture the bounded child output. Interactive Claude
	// launches still replace AIGW through the normal adapter path.
	plan.Replace = false
	runner, ok := app.Runner.(CaptureRunner)
	if !ok {
		return fmt.Errorf("Claude verification runner is unavailable")
	}
	output, err := runner.RunCapture(ctx, plan)
	if err != nil {
		return fmt.Errorf("Claude minimal request failed: %w", err)
	}
	if strings.TrimSpace(string(output)) != verificationSentinel {
		return fmt.Errorf("Claude model response did not return the required verification sentinel")
	}
	return nil
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
			r.Success("客户端配置已对齐；未改动认证")
			r.Next("aigw check")
			return nil
		},
	}
}

func newRollbackCommand(app *App) *cobra.Command {
	var lastChange bool
	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "回退到最近一次完整验证配置，或上一次配置",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			current, err := app.Config.Load()
			if err != nil {
				return err
			}
			restored := domain.Config{}
			source := ""
			if !lastChange {
				checkpoint, checkpointErr := app.Config.LoadVerifiedCheckpoint()
				if checkpointErr == nil {
					restored = checkpoint.Config
					source = "最近一次完整验证配置"
				} else if !errors.Is(checkpointErr, os.ErrNotExist) {
					return checkpointErr
				}
			}
			if source == "" {
				restored, err = app.Config.LoadBackup()
				if err != nil {
					if errors.Is(err, os.ErrNotExist) {
						return fmt.Errorf("no verified checkpoint or previous configuration backup is available")
					}
					return err
				}
				source = "上一次配置"
			}
			if err := commitConfigAndSync(cmd.Context(), app, current, restored, "rollback"); err != nil {
				return err
			}
			r := renderer(app)
			r.Title("AIGW", "已安全回退")
			r.Section("恢复来源")
			r.Row("配置", source)
			r.Success("路由与客户端投影已恢复；未重启客户端。")
			r.Next("aigw doctor")
			return nil
		},
	}
	cmd.Flags().BoolVar(&lastChange, "last-change", false, "restore only the immediately previous config backup")
	return cmd
}

const codexAuthenticationTimeout = 20 * time.Second

func syncAdapters(_ context.Context, app *App, cfg domain.Config) error {
	if adapter := cfg.Adapters[domain.ClientCodex]; adapter.Enabled {
		runtime, _, err := cfg.ResolveRuntime(domain.ClientCodex, "")
		if err != nil {
			return err
		}
		for _, target := range adapter.Targets {
			if err := adapters.SyncCodexConfig(target, runtime); err != nil {
				return err
			}
		}
	}
	return nil
}

// bindCodexAuthentication updates Codex's native credential store. It is
// intentionally separate from config sync so a model-only switch cannot
// start a second Codex process or disturb a running desktop session.
func bindCodexAuthentication(ctx context.Context, app *App, cfg domain.Config) error {
	adapter := cfg.Adapters[domain.ClientCodex]
	if !adapter.Enabled {
		return nil
	}
	if adapter.Executable == "" || app.Runner == nil {
		return fmt.Errorf("Codex authentication requires an enabled adapter executable")
	}
	runtime, _, err := cfg.ResolveRuntime(domain.ClientCodex, "")
	if err != nil {
		return err
	}
	accountName := runtime.AccountID
	token, err := app.Secrets.Get(accountName)
	if err != nil {
		return fmt.Errorf("Codex route token unavailable: %w", err)
	}
	for _, target := range adapter.Targets {
		plan, err := adapters.CodexLoginPlan(adapter.Executable, filepath.Dir(target), token)
		if err != nil {
			return err
		}
		targetCtx, cancel := context.WithTimeout(ctx, codexAuthenticationTimeout)
		err = app.Runner.Run(targetCtx, plan)
		cancel()
		if err != nil {
			return err
		}
	}
	return nil
}

func codexRouteAccount(cfg domain.Config) (string, bool) {
	if !cfg.Adapters[domain.ClientCodex].Enabled {
		return "", false
	}
	runtime, _, err := cfg.ResolveRuntime(domain.ClientCodex, "")
	if err != nil {
		return "", false
	}
	return runtime.AccountID, runtime.AccountID != ""
}

func codexAuthenticationChanged(before, after domain.Config) bool {
	beforeAccount, beforeOK := codexRouteAccount(before)
	afterAccount, afterOK := codexRouteAccount(after)
	return afterOK && (!beforeOK || beforeAccount != afterAccount)
}

func cloneConfig(cfg domain.Config) domain.Config {
	clone := cfg
	clone.Accounts = make(map[string]domain.Account, len(cfg.Accounts))
	for name, account := range cfg.Accounts {
		clone.Accounts[name] = account
	}
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

func rollbackConfigAndAdapters(ctx context.Context, app *App, before domain.Config, rebindNativeAuthentication bool) error {
	if err := app.Config.Save(before); err != nil {
		return err
	}
	if err := syncAdapters(ctx, app, before); err != nil {
		return err
	}
	if rebindNativeAuthentication {
		return bindCodexAuthentication(ctx, app, before)
	}
	return nil
}

func commitConfigAndSync(ctx context.Context, app *App, before, after domain.Config, subject string) error {
	if err := app.Config.Save(after); err != nil {
		return err
	}
	if err := syncAdapters(ctx, app, after); err != nil {
		rollbackErr := rollbackConfigAndAdapters(ctx, app, before, false)
		if rollbackErr != nil {
			return fmt.Errorf("%s sync failed: %w; rollback also failed: %v", subject, err, rollbackErr)
		}
		return fmt.Errorf("%s sync failed and was rolled back: %w", subject, err)
	}
	if codexAuthenticationChanged(before, after) {
		if err := bindCodexAuthentication(ctx, app, after); err != nil {
			rollbackErr := rollbackConfigAndAdapters(ctx, app, before, true)
			if rollbackErr != nil {
				return fmt.Errorf("%s authentication failed: %w; rollback also failed: %v", subject, err, rollbackErr)
			}
			return fmt.Errorf("%s authentication failed and was rolled back: %w", subject, err)
		}
	}
	return nil
}

func _processPlanCompileGuard(_ adapters.ProcessPlan) {}

func accountForInput(cfg domain.Config, name string) (string, domain.Account, error) {
	cfg.Normalize()
	if account, ok := cfg.Accounts[name]; ok {
		account.ID = name
		return name, account, nil
	}
	if profile, ok := cfg.Profiles[name]; ok {
		account, exists := cfg.Accounts[profile.Account]
		if !exists {
			return "", domain.Account{}, fmt.Errorf("profile %q references unknown account %q", name, profile.Account)
		}
		account.ID = profile.Account
		return profile.Account, account, nil
	}
	return "", domain.Account{}, fmt.Errorf("unknown account or profile %q", name)
}
