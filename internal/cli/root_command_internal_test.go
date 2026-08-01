package cli

import (
	configuration "aigw-cli/internal/configuration"
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootLoadHiddenClaudeHelpAndCompletionBranches(t *testing.T) {
	t.Run("root load", func(t *testing.T) {
		out := &bytes.Buffer{}
		app := &App{Config: configuration.NewStore(t.TempDir()), Out: out, Err: out}
		root := NewRoot(app)
		root.SetArgs(nil)
		if err := root.Execute(); err == nil {
			t.Fatal("expected root config load error")
		}
	})

	t.Run("hidden Claude", func(t *testing.T) {
		out := &bytes.Buffer{}
		app := &App{Config: configuration.NewStore(t.TempDir()), Out: out, Err: out}
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
