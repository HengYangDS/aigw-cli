package cli

import (
	"context"
	"fmt"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

// runWizard is deliberately provider-neutral. AIGW never assumes a gateway,
// token slot, URL, or model catalogue for a new user; the user may instead
// import a secret-free team manifest before running this flow.
func runWizard(ctx context.Context, app *App) error {
	account, err := app.Prompt.Text("Account 标识（例如 team-gateway）：")
	if err != nil {
		return err
	}
	if !domain.ValidProfileName(account) {
		return fmt.Errorf("invalid account name %q; use letters, numbers, dot, dash, or underscore", account)
	}
	label, err := app.Prompt.Text("服务名称：")
	if err != nil {
		return err
	}
	client, err := app.Prompt.Select("首次配置的客户端：", []Choice{
		{Value: domain.ClientCodex, Label: "Codex（OpenAI Responses）"},
		{Value: domain.ClientClaude, Label: "Claude（Anthropic）"},
	})
	if err != nil {
		return err
	}
	endpointLabel := "OpenAI Responses URL："
	if client == domain.ClientClaude {
		endpointLabel = "Anthropic URL："
	}
	endpoint, err := app.Prompt.Text(endpointLabel)
	if err != nil {
		return err
	}
	profile, err := app.Prompt.Text("模型 Profile 标识（例如 gpt-5.6-terra）：")
	if err != nil {
		return err
	}
	model, err := app.Prompt.Text("上游模型 ID：")
	if err != nil {
		return err
	}
	request := setupRequest{Account: account, Profile: profile, Label: label, Client: client, Model: model, PromptToken: true}
	if client == domain.ClientCodex {
		request.OpenAIURL = endpoint
	} else {
		request.AnthropicURL = endpoint
	}
	return runSetup(ctx, app, request)
}
