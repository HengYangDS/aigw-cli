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

func newModelsCommand(app *App) *cobra.Command {
	return &cobra.Command{Use: "models", Short: "检查模型 Profile 是否被网关列出", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := app.Config.Load()
		if err != nil {
			return err
		}
		if len(cfg.Profiles) == 0 {
			return fmt.Errorf("尚未配置；运行 `aigw setup`")
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
			r.Status(state, row.Profile, fmt.Sprintf("%s · %s · %s", row.Model, title(row.Client), row.Reach))
			r.Detail("Account: " + row.Account)
		}
		if len(rows) == 0 {
			r.Status(presentation.Info, "模型", "未配置模型 Profile")
		}
		r.Next("aigw use")
		return nil
	}}
}

func fetchModelSet(parent context.Context, client HTTPDoer, account domain.Account, token string) (map[string]bool, error) {
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
	set := map[string]bool{}
	for _, id := range ids {
		set[id] = true
	}
	return set, nil
}

func parseModelIDs(data []byte) ([]string, error) {
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	items, _ := payload["data"].([]any)
	ids := []string{}
	for _, item := range items {
		switch typed := item.(type) {
		case string:
			ids = append(ids, typed)
		case map[string]any:
			for _, key := range []string{"id", "model", "name"} {
				if value, ok := typed[key].(string); ok && value != "" {
					ids = append(ids, value)
					break
				}
			}
		}
	}
	sort.Strings(ids)
	return ids, nil
}
