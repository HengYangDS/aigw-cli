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
	account, err := app.Prompt.Text("Account ID (for example, team-gateway): ")
	if err != nil {
		return err
	}
	if !domain.ValidProfileName(account) {
		return fmt.Errorf("invalid account ID %q; use letters, numbers, dots, hyphens, or underscores", account)
	}
	label, err := app.Prompt.Text("Provider display name: ")
	if err != nil {
		return err
	}
	client, err := app.Prompt.Select("Client for the first profile: ", []Choice{
		{Value: domain.ClientCodex, Label: "Codex (OpenAI Responses)"},
		{Value: domain.ClientClaude, Label: "Claude (Anthropic)"},
	})
	if err != nil {
		return err
	}
	endpointLabel := "OpenAI Responses URL: "
	if client == domain.ClientClaude {
		endpointLabel = "Anthropic URL: "
	}
	endpoint, err := app.Prompt.Text(endpointLabel)
	if err != nil {
		return err
	}
	profile, err := app.Prompt.Text("Profile ID (for example, gpt-5.6-terra): ")
	if err != nil {
		return err
	}
	model, err := app.Prompt.Text("Upstream model ID: ")
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
