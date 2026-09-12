package onboarding

import (
	"context"
	"errors"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/prompt"
)

type scriptedWizardPrompt struct {
	texts       []string
	textPrompts []string
	textCalls   int
	failTextAt  int
	selectErr   error
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
func (candidate *scriptedWizardPrompt) Select(string, []prompt.Choice) (string, error) {
	if candidate.selectErr != nil {
		return "", candidate.selectErr
	}
	return "codex", nil
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
		{name: "profile read", prompt: &scriptedWizardPrompt{texts: []string{"account", "Label", "https://one.test"}, failTextAt: 4}},
		{name: "model read", prompt: &scriptedWizardPrompt{texts: []string{"account", "Label", "https://one.test", "profile"}, failTextAt: 5}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := RunWizard(context.Background(), invocation.Context{Prompt: test.prompt}); err == nil {
				t.Fatal("expected wizard failure")
			}
		})
	}
}
