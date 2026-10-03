package onboarding

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/prompt"
)

type scriptedWizardPrompt struct {
	texts       []string
	textPrompts []string
	textCalls   int
	failTextAt  int
	selectErr   error
	selections  []string
	choiceSets  [][]prompt.Choice
}

func (candidate *scriptedWizardPrompt) Secret(string) (string, error) { return "", nil }
func (candidate *scriptedWizardPrompt) Text(message string) (string, error) {
	candidate.textPrompts = append(candidate.textPrompts, message)
	candidate.textCalls++
	if candidate.failTextAt == candidate.textCalls {
		return "", errors.New("text failed")
	}
	if candidate.textCalls <= len(candidate.texts) {
		return candidate.texts[candidate.textCalls-1], nil
	}
	return "", nil
}

func TestWizardUsesEndpointNeutralAccountExample(t *testing.T) {
	prompt := &scriptedWizardPrompt{texts: []string{"team-primary"}, failTextAt: 2}

	if err := RunWizard(context.Background(), invocation.Context{Prompt: prompt}); err == nil {
		t.Fatal("expected wizard failure after the account prompt")
	}
	if len(prompt.textPrompts) == 0 || prompt.textPrompts[0] != "Account ID (for example, team-primary): " {
		t.Fatalf("account prompt = %q", prompt.textPrompts)
	}
	if strings.Contains(strings.ToLower(prompt.textPrompts[0]), "gateway") {
		t.Fatalf("account prompt assumes a gateway: %q", prompt.textPrompts[0])
	}
}

func (candidate *scriptedWizardPrompt) Select(_ string, choices []prompt.Choice) (string, error) {
	candidate.choiceSets = append(candidate.choiceSets, slices.Clone(choices))
	if candidate.selectErr != nil {
		return "", candidate.selectErr
	}
	if len(candidate.selections) != 0 {
		selected := candidate.selections[0]
		candidate.selections = candidate.selections[1:]
		return selected, nil
	}
	return "codex", nil
}

func TestWizardOffersAdmittedClientsAndHermesProtocols(t *testing.T) {
	prompt := &scriptedWizardPrompt{
		texts:      []string{"team", "Team", "https://messages.test"},
		selections: []string{configuration.ClientHermes, string(configuration.ProtocolAnthropic)},
		failTextAt: 4,
	}
	if err := RunWizard(context.Background(), invocation.Context{Prompt: prompt}); err == nil {
		t.Fatal("expected wizard to stop at the Route prompt")
	}
	if len(prompt.choiceSets) != 2 {
		t.Fatalf("wizard choices = %#v; want client and protocol", prompt.choiceSets)
	}
	clients := make([]string, 0, len(prompt.choiceSets[0]))
	for _, choice := range prompt.choiceSets[0] {
		clients = append(clients, choice.Value)
	}
	if !slices.Equal(clients, configuration.AdmittedClientIDs()) || !slices.Contains(clients, configuration.ClientClaudeDesktop) || !slices.Contains(clients, configuration.ClientHermes) {
		t.Fatalf("wizard clients = %#v", clients)
	}
	protocols := make([]string, 0, len(prompt.choiceSets[1]))
	for _, choice := range prompt.choiceSets[1] {
		protocols = append(protocols, choice.Value)
	}
	if !slices.Equal(protocols, []string{string(configuration.ProtocolOpenAIResponses), string(configuration.ProtocolAnthropic), string(configuration.ProtocolOpenAIChatCompletions)}) {
		t.Fatalf("Hermes protocols = %#v", protocols)
	}
	if len(prompt.textPrompts) != 4 || prompt.textPrompts[2] != "Anthropic Messages URL: " || strings.Contains(strings.ToLower(prompt.textPrompts[3]), "gpt-5.6") {
		t.Fatalf("wizard text prompts = %#v", prompt.textPrompts)
	}
}

func TestWizardPromptFailureBranches(t *testing.T) {
	tests := []struct {
		name   string
		prompt *scriptedWizardPrompt
	}{
		{name: "account read", prompt: &scriptedWizardPrompt{failTextAt: 1}},
		{name: "invalid account", prompt: &scriptedWizardPrompt{texts: []string{"bad id"}}},
		{name: "label read", prompt: &scriptedWizardPrompt{texts: []string{"account"}, failTextAt: 2}},
		{name: "client select", prompt: &scriptedWizardPrompt{texts: []string{"account", "Label"}, selectErr: errors.New("select failed")}},
		{name: "endpoint read", prompt: &scriptedWizardPrompt{texts: []string{"account", "Label"}, failTextAt: 3}},
		{name: "route read", prompt: &scriptedWizardPrompt{texts: []string{"account", "Label", "https://one.test"}, failTextAt: 4}},
		{name: "model read", prompt: &scriptedWizardPrompt{texts: []string{"account", "Label", "https://one.test", "route"}, failTextAt: 5}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := RunWizard(context.Background(), invocation.Context{Prompt: test.prompt}); err == nil {
				t.Fatal("expected wizard failure")
			}
		})
	}
}
