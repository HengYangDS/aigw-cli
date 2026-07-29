package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/config"
)

type wizardCoveragePrompt struct {
	texts      []string
	textCalls  int
	failTextAt int
	selectErr  error
}

func (prompt *wizardCoveragePrompt) Secret(string) (string, error) { return "", nil }
func (prompt *wizardCoveragePrompt) Text(string) (string, error) {
	prompt.textCalls++
	if prompt.failTextAt == prompt.textCalls {
		return "", errors.New("text failed")
	}
	if prompt.textCalls <= len(prompt.texts) {
		return prompt.texts[prompt.textCalls-1], nil
	}
	return "", nil
}
func (prompt *wizardCoveragePrompt) Select(string, []Choice) (string, error) {
	if prompt.selectErr != nil {
		return "", prompt.selectErr
	}
	return "codex", nil
}

func TestOutputLocalizationMalformedAndSpecificBranches(t *testing.T) {
	for input, want := range map[string]string{
		`profile "one" references unknown account "missing"`:        `profile "one" references unknown account "missing"`,
		`account "one" has no Anthropic endpoint`:                   `account "one" has no Anthropic endpoint`,
		`account "one" has no OpenAI Responses endpoint`:            `account "one" has no OpenAI Responses endpoint`,
		`validate config: unsupported config version 3; expected 2`: `unsupported configuration version: found 3, expected 2`,
	} {
		if got := localizeCobraError(input); got != want {
			t.Errorf("localizeCobraError(%q) = %q, want %q", input, got, want)
		}
	}

	if _, ok := cobraUnknownCommand(`unknown command "" for "aigw"`); ok {
		t.Fatal("empty unknown command accepted")
	}
	if _, ok := cobraUnknownFlag("unknown flag: --two words"); ok {
		t.Fatal("spaced flag accepted")
	}
	if _, _, _, ok := runtimeProfileClientMismatch(`profile "one" is for codex`); ok {
		t.Fatal("malformed mismatch accepted")
	}
	if _, _, ok := runtimeProfileUnknownAccount(`profile "one" references unknown account ""`); ok {
		t.Fatal("empty account accepted")
	}
	if _, _, ok := unsupportedConfigVersion("validate config: unsupported config version x; expected 2"); ok {
		t.Fatal("nonnumeric version accepted")
	}
	if _, _, ok := unsupportedConfigVersion("validate config: unsupported config version 3; expected x"); ok {
		t.Fatal("nonnumeric expected version accepted")
	}
	if got := mentionedAIGWCommand("run `aigw repair without terminator"); got != "" {
		t.Fatalf("unterminated command = %q", got)
	}
	if healthImpact(1) != "The configured AI client is unavailable." || !strings.Contains(healthImpact(3), "3 configured") {
		t.Fatal("health impact pluralization is incorrect")
	}
}

func TestRootLoadHiddenClaudeHelpAndCompletionBranches(t *testing.T) {
	t.Run("root load", func(t *testing.T) {
		out := &bytes.Buffer{}
		app := &App{Config: config.NewStore(t.TempDir()), Out: out, Err: out}
		root := NewRoot(app)
		root.SetArgs(nil)
		if err := root.Execute(); err == nil {
			t.Fatal("expected root config load error")
		}
	})

	t.Run("hidden Claude", func(t *testing.T) {
		out := &bytes.Buffer{}
		app := &App{Config: config.NewStore(t.TempDir()), Out: out, Err: out}
		root := NewRoot(app)
		command, _, err := root.Find([]string{"__run-claude"})
		if err != nil {
			t.Fatal(err)
		}
		if err := command.RunE(command, []string{"--arg"}); err == nil {
			t.Fatal("expected Claude preflight error")
		}
	})

	t.Run("ungrouped help", func(t *testing.T) {
		out := &bytes.Buffer{}
		app := &App{Out: out, Err: out}
		command := &cobra.Command{Use: "custom", Short: "custom help"}
		command.AddCommand(
			&cobra.Command{Use: "zeta", Short: "Z", Run: func(*cobra.Command, []string) {}},
			&cobra.Command{Use: "alpha", Short: "A", Run: func(*cobra.Command, []string) {}},
		)
		command.Flags().String("visible", "", "visible option")
		command.Flags().String("secret", "", "hidden option")
		_ = command.Flags().MarkHidden("secret")
		renderCommandHelp(app, command)
		text := out.String()
		if !strings.Contains(text, "Commands") || strings.Index(text, "alpha") > strings.Index(text, "zeta") || strings.Contains(text, "hidden option") {
			t.Fatalf("help = %q", text)
		}
	})

	t.Run("completions", func(t *testing.T) {
		for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
			root := &cobra.Command{Use: "sample"}
			root.SetOut(&bytes.Buffer{})
			command := newCompletionCommand(root)
			if err := command.RunE(command, []string{shell}); err != nil {
				t.Fatalf("%s completion: %v", shell, err)
			}
		}
		root := &cobra.Command{Use: "sample"}
		command := newCompletionCommand(root)
		if err := command.RunE(command, []string{"other"}); err == nil {
			t.Fatal("expected unsupported shell error")
		}
	})
}

func TestWizardPromptFailureBranches(t *testing.T) {
	tests := []struct {
		name   string
		prompt *wizardCoveragePrompt
	}{
		{name: "account read", prompt: &wizardCoveragePrompt{failTextAt: 1}},
		{name: "invalid account", prompt: &wizardCoveragePrompt{texts: []string{"bad id"}}},
		{name: "label read", prompt: &wizardCoveragePrompt{texts: []string{"account"}, failTextAt: 2}},
		{name: "client select", prompt: &wizardCoveragePrompt{texts: []string{"account", "Label"}, selectErr: errors.New("select failed")}},
		{name: "endpoint read", prompt: &wizardCoveragePrompt{texts: []string{"account", "Label"}, failTextAt: 3}},
		{name: "profile read", prompt: &wizardCoveragePrompt{texts: []string{"account", "Label", "https://one.test"}, failTextAt: 4}},
		{name: "model read", prompt: &wizardCoveragePrompt{texts: []string{"account", "Label", "https://one.test", "profile"}, failTextAt: 5}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := runWizard(context.Background(), &App{Prompt: test.prompt}); err == nil {
				t.Fatal("expected wizard failure")
			}
		})
	}
}
