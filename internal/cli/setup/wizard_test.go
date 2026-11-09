package setup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claudedesktop"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/store"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

type scriptedWizardPrompt struct {
	texts       []string
	textPrompts []string
	textCalls   int
	failTextAt  int
	textErr     error
	selectErr   error
	selections  []string
	choiceSets  [][]terminal.Choice
}

func (candidate *scriptedWizardPrompt) Secret(string) (string, error) { return "", nil }
func (candidate *scriptedWizardPrompt) Text(message string) (string, error) {
	candidate.textPrompts = append(candidate.textPrompts, message)
	candidate.textCalls++
	if candidate.failTextAt == candidate.textCalls {
		if candidate.textErr != nil {
			return "", candidate.textErr
		}
		return "", errors.New("text failed")
	}
	if candidate.textCalls <= len(candidate.texts) {
		return candidate.texts[candidate.textCalls-1], nil
	}
	return "", nil
}

func TestWizardUsesEndpointNeutralAccountExample(t *testing.T) {
	prompt := &scriptedWizardPrompt{texts: []string{"team-primary"}, failTextAt: 2}

	if err := runWizard(context.Background(), invocation.Context{Clients: clients.Registry(), Prompt: prompt}); err == nil {
		t.Fatal("expected wizard failure after the account prompt")
	}
	if len(prompt.textPrompts) == 0 || prompt.textPrompts[0] != "Account ID (for example, team-primary): " {
		t.Fatalf("account prompt = %q", prompt.textPrompts)
	}
	if strings.Contains(strings.ToLower(prompt.textPrompts[0]), "gateway") {
		t.Fatalf("account prompt assumes a gateway: %q", prompt.textPrompts[0])
	}
}

func (candidate *scriptedWizardPrompt) Select(_ string, choices []terminal.Choice) (string, error) {
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
		selections: []string{hermes.ID, string(domain.ProtocolAnthropic)},
		failTextAt: 4,
	}
	if err := runWizard(context.Background(), invocation.Context{Clients: clients.Registry(), Prompt: prompt}); err == nil {
		t.Fatal("expected wizard to stop at the Route prompt")
	}
	if len(prompt.choiceSets) != 2 {
		t.Fatalf("wizard choices = %#v; want client and protocol", prompt.choiceSets)
	}
	offered := make([]string, 0, len(prompt.choiceSets[0]))
	for _, choice := range prompt.choiceSets[0] {
		offered = append(offered, choice.Value)
	}
	if !slices.Equal(offered, clients.Admission().IDs()) || !slices.Contains(offered, claudedesktop.ID) || !slices.Contains(offered, hermes.ID) {
		t.Fatalf("wizard clients = %#v", offered)
	}
	protocols := make([]string, 0, len(prompt.choiceSets[1]))
	for _, choice := range prompt.choiceSets[1] {
		protocols = append(protocols, choice.Value)
	}
	if !slices.Equal(protocols, []string{string(domain.ProtocolOpenAIResponses), string(domain.ProtocolAnthropic), string(domain.ProtocolOpenAIChatCompletions)}) {
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
			if err := runWizard(context.Background(), invocation.Context{Clients: clients.Registry(), Prompt: test.prompt}); err == nil {
				t.Fatal("expected wizard failure")
			}
		})
	}
}

// A wizard prompt that ends without an answer, cancelled or at the end of its
// input, names the prompt, says that nothing changed and names `aigw setup`
// again, which asks again; no configuration is written.
func TestSetupNamesACancelledWizardPrompt(t *testing.T) {
	cancelled := fmt.Errorf("%w: no input received", terminal.ErrInputCancelled)
	for _, test := range []struct {
		title  string
		prompt *scriptedWizardPrompt
	}{
		{title: "The provider ID prompt was cancelled", prompt: &scriptedWizardPrompt{textErr: cancelled, failTextAt: 1}},
		{title: "The provider name prompt was cancelled", prompt: &scriptedWizardPrompt{texts: []string{"team"}, textErr: cancelled, failTextAt: 2}},
		{title: "The client prompt was cancelled", prompt: &scriptedWizardPrompt{texts: []string{"team", "Team"}, selectErr: cancelled}},
		{title: "The client prompt ended without a choice", prompt: &scriptedWizardPrompt{texts: []string{"team", "Team"}, selectErr: terminal.ErrNoChoice}},
		{title: "The endpoint prompt was cancelled", prompt: &scriptedWizardPrompt{texts: []string{"team", "Team"}, textErr: cancelled, failTextAt: 3}},
		{title: "The model ID prompt was cancelled", prompt: &scriptedWizardPrompt{texts: []string{"team", "Team", "https://one.test/v1", "team-model"}, textErr: cancelled, failTextAt: 5}},
	} {
		t.Run(test.title, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "configuration.toml")
			runtime := invocation.Context{Clients: clients.Registry(), Config: store.NewStore(path, clients.Admission()), Interactive: true, Prompt: test.prompt, Out: io.Discard}
			root := &cobra.Command{Use: "aigw"}
			command := NewCommand(runtime)
			root.AddCommand(command)
			root.SetArgs([]string{"setup"})
			root.SilenceErrors, root.SilenceUsage = true, true
			problem := terminal.ProblemFor(root.Execute())
			if problem.Title != test.title || problem.Impact != "Nothing changed." || problem.Fix != "aigw setup" {
				t.Fatalf("a cancelled wizard prompt = %#v", problem)
			}
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("a cancelled wizard prompt wrote the configuration: %v", err)
			}
		})
	}
}
