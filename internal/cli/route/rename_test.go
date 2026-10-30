package route

import (
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"

	"aigw-cli/internal/store"

	"context"
	"errors"

	"github.com/spf13/cobra"

	"strings"
	"testing"
)

func TestRouteRenamePreservesInvocationAndDomainErrors(t *testing.T) {
	for _, constructor := range []struct {
		name string
		new  func(invocation.Context) *cobra.Command
	}{
		{name: "route", new: newRenameCommand},
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
			runtime := invocation.Context{Clients: clients.Registry(), Interactive: true, Prompt: selectionPrompt{}}
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
			runtime.Config = store.NewStore(t.TempDir(), clients.Admission())
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
