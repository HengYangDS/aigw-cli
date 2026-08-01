package onboarding

import (
	"context"
	"errors"
	"testing"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/prompt"
)

type scriptedWizardPrompt struct {
	texts      []string
	textCalls  int
	failTextAt int
	selectErr  error
}

func (candidate *scriptedWizardPrompt) Secret(string) (string, error) { return "", nil }
func (candidate *scriptedWizardPrompt) Text(string) (string, error) {
	candidate.textCalls++
	if candidate.failTextAt == candidate.textCalls {
		return "", errors.New("text failed")
	}
	if candidate.textCalls <= len(candidate.texts) {
		return candidate.texts[candidate.textCalls-1], nil
	}
	return "", nil
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
