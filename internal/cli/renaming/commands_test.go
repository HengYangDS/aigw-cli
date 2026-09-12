package renaming

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"

	"github.com/spf13/cobra"
)

func renameRuntime(t *testing.T) invocation.Context {
	t.Helper()
	store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
	cfg := configuration.NewConfig()
	cfg.Accounts["old"] = configuration.Account{Label: "Old", Endpoints: configuration.Endpoints{OpenAIResponses: "https://old.test/v1"}}
	cfg.Profiles["old"] = configuration.Profile{Label: "Old", Account: "old", Client: configuration.ClientCodex, Model: "gpt"}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	secretStore := secrets.NewMemoryStore()
	diagnostics, err := secrets.NewDiagnosticCredentialStore(secretStore)
	if err != nil {
		t.Fatal(err)
	}
	return invocation.Context{Config: store, Secrets: secretStore, Accounts: diagnostics, Out: &bytes.Buffer{}}
}

func TestRenameCommandsPreserveInvocationAndDomainErrors(t *testing.T) {
	for _, constructor := range []struct {
		name string
		new  func(invocation.Context) *cobra.Command
	}{
		{name: "profile", new: NewProfileCommand},
		{name: "account", new: NewAccountCommand},
	} {
		for _, test := range []struct {
			name string
			args []string
			want string
		}{
			{name: "missing IDs", want: "requires <old> <new>"},
			{name: "unknown source", args: []string{"missing", "new"}, want: "Unknown"},
		} {
			t.Run(constructor.name+"/"+test.name, func(t *testing.T) {
				command := constructor.new(renameRuntime(t))
				command.SilenceErrors, command.SilenceUsage = true, true
				command.SetArgs(test.args)
				if err := command.Execute(); err == nil || !strings.Contains(err.Error(), test.want) {
					t.Fatalf("error = %v, want %q", err, test.want)
				}
			})
		}
		t.Run(constructor.name+"/argument admission", func(t *testing.T) {
			runtime := invocation.Context{Interactive: true, Prompt: selectionPrompt{}}
			for _, args := range [][]string{nil, {"old"}, {"old", "new.id-1"}} {
				if err := constructor.new(runtime).ValidateArgs(args); err != nil {
					t.Fatalf("valid interactive input %q rejected: %v", args, err)
				}
			}
			for _, args := range [][]string{{"bad id"}, {"old", "\t"}, {"old", "new", "extra"}} {
				if err := constructor.new(runtime).ValidateArgs(args); err == nil {
					t.Fatalf("invalid interactive input %q admitted", args)
				}
			}
			runtime.Prompt = nil
			if err := constructor.new(runtime).ValidateArgs([]string{"old", "new"}); err != nil {
				t.Fatalf("explicit identifiers require no prompt: %v", err)
			}
			if err := constructor.new(runtime).ValidateArgs(nil); err == nil {
				t.Fatal("incomplete interactive invocation without a prompt admitted")
			}
		})
		t.Run(constructor.name+"/interactive error", func(t *testing.T) {
			runtime := renameRuntime(t)
			want := errors.New("selection failed")
			runtime.Interactive = true
			runtime.Prompt = selectionPrompt{selectErr: want}
			command := constructor.new(runtime)
			command.SilenceErrors, command.SilenceUsage = true, true
			if err := command.Execute(); !errors.Is(err, want) {
				t.Fatalf("error = %v, want selection failure", err)
			}
		})
		t.Run(constructor.name+"/unreadable configuration", func(t *testing.T) {
			runtime := renameRuntime(t)
			runtime.Config = configuration.NewStore(t.TempDir())
			command := constructor.new(runtime)
			command.SilenceErrors, command.SilenceUsage = true, true
			command.SetArgs([]string{"old", "new"})
			if err := command.Execute(); err == nil {
				t.Fatal("unreadable configuration was accepted")
			}
		})
		t.Run(constructor.name+"/cancellation", func(t *testing.T) {
			command := constructor.new(renameRuntime(t))
			command.SilenceErrors, command.SilenceUsage = true, true
			command.SetArgs([]string{"old", "new"})
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			if err := command.ExecuteContext(ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("error = %v, want cancellation", err)
			}
		})
	}
}

func TestAccountFinalizationRequiresExplicitIntent(t *testing.T) {
	for _, args := range [][]string{
		{"old", "--finalize"},
		{"old", "--finalize=true"},
		{"old", "new", "--confirm-api-token-rotation"},
		{"old", "new", "--confirm-account-probe-rotation"},
		{"old", "new", "--finalize=false", "--confirm-api-token-rotation=true"},
	} {
		command := NewAccountCommand(renameRuntime(t))
		command.SilenceErrors, command.SilenceUsage = true, true
		command.PreRun = func(*cobra.Command, []string) {
			t.Fatalf("invalid finalization reached execution: %v", args)
		}
		command.SetArgs(args)
		if err := command.Execute(); err == nil {
			t.Fatalf("incomplete finalization intent accepted: %v", args)
		}
	}
}

func TestInteractiveRenamePreservesConfigurationLoadFailure(t *testing.T) {
	runtime := renameRuntime(t)
	path := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(path, []byte("invalid TOML = ["), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime.Config = configuration.NewStore(path)
	runtime.Interactive = true
	runtime.Prompt = selectionPrompt{}
	if _, _, err := resolveIDs(runtime, "account", nil); err == nil {
		t.Fatal("invalid configuration was accepted for interactive selection")
	}
}
