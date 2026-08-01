// Package catalog owns provider-neutral model discovery and presentation.
package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/secrets"
	"github.com/spf13/cobra"
)

// HTTPDoer executes catalog requests.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// Dependencies are the capabilities required by model commands.
type Dependencies struct {
	Config    configuration.Store
	Secrets   secrets.Store
	HTTP      HTTPDoer
	Out       io.Writer
	Color     bool
	Width     int
	RenderOut io.Writer
}

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

// NewModelsCommand constructs the configured-model availability command.
func NewModelsCommand(deps Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "models",
		Short: "Check whether configured models are listed by their gateways",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := deps.Config.Load()
			if err != nil {
				return err
			}
			if len(cfg.Profiles) == 0 {
				return fmt.Errorf("not configured; run `aigw setup`")
			}
			modelSets := map[string]map[string]bool{}
			for accountName, account := range cfg.Accounts {
				if account.Endpoints.OpenAIResponses == "" || !deps.Secrets.Has(accountName) {
					continue
				}
				token, err := deps.Secrets.Get(accountName)
				if err != nil {
					continue
				}
				if models, err := FetchSet(cmd.Context(), deps.HTTP, account, token); err == nil {
					modelSets[accountName] = models
				}
			}
			rows := modelRows(cfg, modelSets)
			r := renderer(deps)
			r.ProductTitle("Model availability")
			r.Section("Service profiles")
			for _, row := range rows {
				state := presentation.Info
				if row.Reach == "Reachable" {
					state = presentation.OK
				} else if row.Reach == "Unavailable" {
					state = presentation.Fail
				}
				r.StatusLine(state, "Profile", row.Profile)
				r.Detail(fmt.Sprintf("%s · %s · %s · account %s", modelTitle(row.Client), row.Model, row.Reach, row.Account))
			}
			if len(rows) == 0 {
				r.Status(presentation.Info, "model", "No model services are configured")
			}
			r.Next("aigw use")
			return r.Err()
		},
	}
}

func modelRows(cfg configuration.Config, modelSets map[string]map[string]bool) []modelRow {
	rows := []modelRow{}
	for _, name := range sortedProfileNames(cfg) {
		profile := cfg.Profiles[name]
		for _, item := range []struct{ client, model string }{
			{configuration.ClientCodex, profile.ModelFor(configuration.ClientCodex)},
			{configuration.ClientClaude, profile.ModelFor(configuration.ClientClaude)},
		} {
			if item.model == "" {
				continue
			}
			reach := "Unknown"
			if set, ok := modelSets[profile.Account]; ok {
				reach = "Unavailable"
				if set[item.model] {
					reach = "Reachable"
				}
			}
			rows = append(rows, modelRow{name, profile.Account, item.client, item.model, reach})
		}
	}
	return rows
}

// NewCatalogCommand constructs authenticated catalog discovery.
func NewCatalogCommand(deps Dependencies) *cobra.Command {
	var jsonMode, all bool
	cmd := &cobra.Command{Use: "catalog", Short: "Discover authenticated model catalogs for each account (compact summary by default)", Args: cobra.NoArgs}
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		if jsonMode && all {
			return fmt.Errorf("--all cannot be used with --json; JSON already includes the complete catalog")
		}
		cfg, err := deps.Config.Load()
		if err != nil {
			return err
		}
		if len(cfg.Profiles) == 0 {
			if jsonMode {
				return writeJSON(deps.Out, catalogOutput{Accounts: []catalogAccount{}})
			}
			return fmt.Errorf("not configured; run `aigw setup`")
		}
		result := discoverCatalog(cmd.Context(), deps, cfg)
		if jsonMode {
			return writeJSON(deps.Out, result)
		}
		r := renderer(deps)
		r.ProductTitle("Authenticated model catalog")
		for _, account := range result.Accounts {
			r.Section(account.Label + " · " + account.ID)
			if account.Status != "ok" {
				r.Status(presentation.Warn, "Catalog", StatusText(account.Status))
				continue
			}
			if len(account.Models) == 0 {
				r.Status(presentation.Info, "model", "Upstream returned an empty catalog")
				continue
			}
			renderCatalogAccount(r, account, all)
		}
		r.Next("aigw profile add <profile> --account <account> --for <claude|codex> --model <model>")
		return r.Err()
	}
	cmd.Flags().BoolVar(&jsonMode, "json", false, "Write machine-readable JSON")
	cmd.Flags().BoolVar(&all, "all", false, "Show the complete model catalog")
	return cmd
}

func discoverCatalog(ctx context.Context, deps Dependencies, cfg configuration.Config) catalogOutput {
	result := catalogOutput{Accounts: make([]catalogAccount, 0, len(cfg.Accounts))}
	for _, accountName := range sortedModelAccountNames(cfg) {
		account := cfg.Accounts[accountName]
		entry := catalogAccount{ID: accountName, Label: account.Label, Source: "openai_responses", SecretAvailable: deps.Secrets.Has(accountName), Models: []catalogModel{}}
		switch {
		case account.Endpoints.OpenAIResponses == "":
			entry.Status = "openai_responses_unavailable"
		case !entry.SecretAvailable:
			entry.Status = "token_unavailable"
		default:
			token, err := deps.Secrets.Get(accountName)
			if err != nil {
				entry.Status = "token_unavailable"
				break
			}
			ids, err := FetchIDs(ctx, deps.HTTP, account, token)
			if err != nil {
				entry.Status = "request_failed"
				break
			}
			entry.Status = "ok"
			for _, id := range ids {
				entry.Models = append(entry.Models, catalogModel{ID: id, Profiles: ConfiguredProfiles(cfg, accountName, id)})
			}
		}
		result.Accounts = append(result.Accounts, entry)
	}
	return result
}

func renderCatalogAccount(r *presentation.Renderer, account catalogAccount, all bool) {
	configured := make([]catalogModel, 0, len(account.Models))
	for _, model := range account.Models {
		if len(model.Profiles) > 0 {
			configured = append(configured, model)
		}
	}
	r.Row("Models", fmt.Sprintf("%d models", len(account.Models)))
	r.Row("Configured", fmt.Sprintf("%d configured", len(configured)))
	if all {
		for _, model := range account.Models {
			state, detail := catalogModelDisplay(model)
			r.StatusLine(state, "model", model.ID)
			r.Detail(detail)
		}
		return
	}
	for _, model := range configured {
		_, detail := catalogModelDisplay(model)
		r.Status(presentation.OK, "model", model.ID)
		r.Detail(detail)
	}
	if remaining := len(account.Models) - len(configured); remaining > 0 {
		r.Detail(fmt.Sprintf("%d more models are unconfigured; full catalog: aigw catalog --all", remaining))
	}
}

func catalogModelDisplay(model catalogModel) (presentation.State, string) {
	if len(model.Profiles) == 0 {
		return presentation.Info, "Not configured"
	}
	return presentation.OK, "Configured: " + strings.Join(model.Profiles, ", ")
}

// ConfiguredProfiles returns profiles that select model from account.
func ConfiguredProfiles(cfg configuration.Config, accountName, model string) []string {
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

// StatusText maps machine catalog states to human text.
func StatusText(status string) string {
	switch status {
	case "openai_responses_unavailable":
		return "OpenAI Responses endpoint is not configured"
	case "token_unavailable":
		return "Token unavailable"
	case "request_failed":
		return "Catalog request failed; configuration was not changed"
	default:
		return status
	}
}

// FetchSet fetches the catalog and indexes its model IDs.
func FetchSet(parent context.Context, client HTTPDoer, account configuration.Account, token string) (map[string]bool, error) {
	ids, err := FetchIDs(parent, client, account, token)
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set, nil
}

// FetchIDs fetches and parses an OpenAI-compatible model catalog.
func FetchIDs(parent context.Context, client HTTPDoer, account configuration.Account, token string) ([]string, error) {
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
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("model catalog endpoint returned HTTP %d", resp.StatusCode)
	}
	return ParseIDs(body)
}

// ParseIDs accepts common OpenAI-compatible model item shapes.
func ParseIDs(data []byte) ([]string, error) {
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	dataField, exists := payload["data"]
	if !exists {
		return nil, fmt.Errorf("model catalog response is missing the data field")
	}
	items, ok := dataField.([]any)
	if !ok {
		return nil, fmt.Errorf("model catalog response data field is not an array")
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

func renderer(deps Dependencies) *presentation.Renderer {
	out := deps.Out
	if deps.RenderOut != nil {
		out = deps.RenderOut
	}
	return presentation.NewWithWidth(out, deps.Color, deps.Width)
}

func writeJSON(out io.Writer, value any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func sortedProfileNames(cfg configuration.Config) []string {
	names := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedModelAccountNames(cfg configuration.Config) []string {
	names := make([]string, 0, len(cfg.Accounts))
	for name := range cfg.Accounts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func modelTitle(value string) string {
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}
