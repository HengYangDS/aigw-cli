package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/presentation"
)

type modelRow struct {
	Profile string
	Account string
	Client  string
	Model   string
	Reach   string
}

type catalogModel struct {
	ID       string   `json:"id"`
	Profiles []string `json:"profiles,omitempty"`
}

type catalogAccount struct {
	ID              string         `json:"id"`
	Label           string         `json:"label"`
	Source          string         `json:"source"`
	SecretAvailable bool           `json:"secret_available"`
	Status          string         `json:"status"`
	Models          []catalogModel `json:"models"`
}

type catalogOutput struct {
	Accounts []catalogAccount `json:"accounts"`
}

func newModelsCommand(app *App) *cobra.Command {
	return &cobra.Command{Use: "models", Short: "检查模型 Profile 是否被网关列出", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := app.Config.Load()
		if err != nil {
			return err
		}
		if len(cfg.Profiles) == 0 {
			return problem("尚未配置", "尚未创建任何服务 Profile。", "没有可检查的模型或网关目录。", "aigw setup", fmt.Errorf("not configured"))
		}
		modelSets := map[string]map[string]bool{}
		for accountName, account := range cfg.Accounts {
			if account.Endpoints.OpenAIResponses == "" {
				continue
			}
			if !app.Secrets.Has(accountName) {
				continue
			}
			token, err := app.Secrets.Get(accountName)
			if err != nil {
				continue
			}
			models, err := fetchModelSet(cmd.Context(), app.HTTP, account, token)
			if err == nil {
				modelSets[accountName] = models
			}
		}
		rows := []modelRow{}
		for _, name := range sortedProfileNames(cfg) {
			profile := cfg.Profiles[name]
			for _, item := range []struct{ client, model string }{{domain.ClientCodex, profile.ModelFor(domain.ClientCodex)}, {domain.ClientClaude, profile.ModelFor(domain.ClientClaude)}} {
				if item.model == "" {
					continue
				}
				reach := "未知"
				if set, ok := modelSets[profile.Account]; ok {
					if set[item.model] {
						reach = "可达"
					} else {
						reach = "不可达"
					}
				}
				rows = append(rows, modelRow{Profile: name, Account: profile.Account, Client: item.client, Model: item.model, Reach: reach})
			}
		}
		r := renderer(app)
		r.Title("AIGW", "模型可达性")
		r.Section("Profiles")
		for _, row := range rows {
			state := presentation.Info
			if row.Reach == "可达" {
				state = presentation.OK
			} else if row.Reach == "不可达" {
				state = presentation.Fail
			}
			r.StatusLine(state, "Profile", row.Profile)
			r.Detail(fmt.Sprintf("%s · %s · %s · Account %s", title(row.Client), row.Model, row.Reach, row.Account))
		}
		if len(rows) == 0 {
			r.Status(presentation.Info, "模型", "未配置模型 Profile")
		}
		r.Next("aigw use")
		return nil
	}}
}

func newCatalogCommand(app *App) *cobra.Command {
	var jsonMode, all bool
	cmd := &cobra.Command{Use: "catalog", Short: "发现各 Account 的已认证模型目录（默认紧凑摘要）", Args: cobra.NoArgs}
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		if jsonMode && all {
			return fmt.Errorf("--all cannot be combined with --json; JSON already contains the complete catalog")
		}
		cfg, err := app.Config.Load()
		if err != nil {
			return err
		}
		if len(cfg.Profiles) == 0 {
			if jsonMode {
				enc := json.NewEncoder(app.Out)
				enc.SetIndent("", "  ")
				return enc.Encode(catalogOutput{Accounts: []catalogAccount{}})
			}
			return problem("尚未配置", "尚未创建任何服务 Profile。", "没有可用于发现模型目录的 Account 或 Token。", "aigw setup", fmt.Errorf("not configured"))
		}
		result := catalogOutput{Accounts: make([]catalogAccount, 0, len(cfg.Accounts))}
		for _, accountName := range sortedAccountNames(cfg) {
			account := cfg.Accounts[accountName]
			entry := catalogAccount{ID: accountName, Label: account.Label, Source: "openai_responses", SecretAvailable: app.Secrets.Has(accountName), Models: []catalogModel{}}
			switch {
			case account.Endpoints.OpenAIResponses == "":
				entry.Status = "openai_responses_unavailable"
			case !entry.SecretAvailable:
				entry.Status = "token_unavailable"
			default:
				token, getErr := app.Secrets.Get(accountName)
				if getErr != nil {
					entry.Status = "token_unavailable"
					break
				}
				ids, fetchErr := fetchModelIDs(cmd.Context(), app.HTTP, account, token)
				if fetchErr != nil {
					entry.Status = "request_failed"
					break
				}
				entry.Status = "ok"
				for _, id := range ids {
					entry.Models = append(entry.Models, catalogModel{ID: id, Profiles: configuredProfilesForModel(cfg, accountName, id)})
				}
			}
			result.Accounts = append(result.Accounts, entry)
		}
		if jsonMode {
			enc := json.NewEncoder(app.Out)
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		}
		r := renderer(app)
		r.Title("AIGW", "已认证模型目录")
		for _, account := range result.Accounts {
			r.Section(account.Label + " · " + account.ID)
			if account.Status != "ok" {
				r.Status(presentation.Warn, "目录", catalogStatusText(account.Status))
				continue
			}
			if len(account.Models) == 0 {
				r.Status(presentation.Info, "模型", "上游返回空目录")
				continue
			}
			renderCatalogAccount(r, account, all)
		}
		r.Next("aigw profile add <profile> --account <account> --for <claude|codex> --model <model>")
		return nil
	}
	cmd.Flags().BoolVar(&jsonMode, "json", false, "输出机器可读 JSON")
	cmd.Flags().BoolVar(&all, "all", false, "显示完整模型目录")
	return cmd
}

func renderCatalogAccount(r *presentation.Renderer, account catalogAccount, all bool) {
	configured := make([]catalogModel, 0, len(account.Models))
	for _, model := range account.Models {
		if len(model.Profiles) > 0 {
			configured = append(configured, model)
		}
	}
	r.Row("模型总数", fmt.Sprintf("%d 个模型", len(account.Models)))
	r.Row("已配置", fmt.Sprintf("%d 个已配置", len(configured)))
	if all {
		for _, model := range account.Models {
			state, detail := catalogModelDisplay(model)
			r.StatusLine(state, "模型", model.ID)
			r.Detail(detail)
		}
		return
	}
	for _, model := range configured {
		_, detail := catalogModelDisplay(model)
		r.Status(presentation.OK, "模型", model.ID)
		r.Detail(detail)
	}
	if remaining := len(account.Models) - len(configured); remaining > 0 {
		r.Detail(fmt.Sprintf("另有 %d 个未配置模型；完整目录：aigw catalog --all", remaining))
	}
}

func catalogModelDisplay(model catalogModel) (presentation.State, string) {
	if len(model.Profiles) == 0 {
		return presentation.Info, "未配置"
	}
	return presentation.OK, "Profile " + strings.Join(model.Profiles, ", ")
}

func configuredProfilesForModel(cfg domain.Config, accountName, model string) []string {
	profiles := []string{}
	for name, profile := range cfg.Profiles {
		if profile.Account != accountName {
			continue
		}
		for _, configuredModel := range profile.Models {
			if configuredModel == model {
				profiles = append(profiles, name)
				break
			}
		}
	}
	sort.Strings(profiles)
	return profiles
}

func catalogStatusText(status string) string {
	switch status {
	case "openai_responses_unavailable":
		return "未配置 OpenAI Responses 端点"
	case "token_unavailable":
		return "Token 不可用"
	case "request_failed":
		return "目录请求失败；未修改配置"
	default:
		return status
	}
}

func fetchModelSet(parent context.Context, client HTTPDoer, account domain.Account, token string) (map[string]bool, error) {
	ids, err := fetchModelIDs(parent, client, account, token)
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, id := range ids {
		set[id] = true
	}
	return set, nil
}

func fetchModelIDs(parent context.Context, client HTTPDoer, account domain.Account, token string) ([]string, error) {
	endpoint := strings.TrimRight(account.Endpoints.OpenAIResponses, "/")
	if strings.HasSuffix(endpoint, "/v1") {
		endpoint += "/models"
	} else if !strings.HasSuffix(endpoint, "/models") {
		endpoint += "/models"
	}
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("models endpoint returned HTTP %d", resp.StatusCode)
	}
	ids, err := parseModelIDs(body)
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func parseModelIDs(data []byte) ([]string, error) {
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	dataField, exists := payload["data"]
	if !exists {
		return nil, fmt.Errorf("models payload is missing data")
	}
	items, ok := dataField.([]any)
	if !ok {
		return nil, fmt.Errorf("models payload data is not an array")
	}
	ids := []string{}
	seen := map[string]bool{}
	for _, item := range items {
		id := ""
		switch typed := item.(type) {
		case string:
			id = typed
		case map[string]any:
			for _, key := range []string{"id", "model", "name"} {
				if value, ok := typed[key].(string); ok && value != "" {
					id = value
					break
				}
			}
		}
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids, nil
}
