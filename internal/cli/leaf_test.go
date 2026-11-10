package cli

import (
	"reflect"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"

	"github.com/spf13/cobra"
)

// leaves returns every command that help lists and that has no subcommand of
// its own, by its path below the root.
func leaves(root *cobra.Command) map[string]*cobra.Command {
	found := map[string]*cobra.Command{}
	var walk func(*cobra.Command)
	walk = func(command *cobra.Command) {
		for _, child := range command.Commands() {
			if !child.IsAvailableCommand() {
				continue
			}
			if child.HasAvailableSubCommands() {
				walk(child)
				continue
			}
			found[strings.TrimPrefix(child.CommandPath(), root.Name()+" ")] = child
		}
	}
	walk(root)
	return found
}

// TestEveryLeafShowsOneToThreeExamples requires one to three examples in the
// help of every leaf command, each a complete command line of that command.
func TestEveryLeafShowsOneToThreeExamples(t *testing.T) {
	commands := leaves(NewRoot(&App{Clients: clients.Registry()}))
	if len(commands) != 32 {
		t.Fatalf("found %d leaf commands, want the designed 32", len(commands))
	}
	for path, command := range commands {
		lines := strings.Split(command.Example, "\n")
		if strings.TrimSpace(command.Example) == "" || len(lines) > 3 {
			t.Errorf("aigw %s shows %d examples, want one to three:\n%s", path, len(lines), command.Example)
			continue
		}
		for _, line := range lines {
			if line != "aigw "+path && !strings.HasPrefix(line, "aigw "+path+" ") {
				t.Errorf("aigw %s shows the example %q, which is not a command line of it", path, line)
			}
		}
	}
}

// TestEveryArgumentAndFilterCompletes requires a declared completion for the
// positional arguments and the filter flags of every leaf command. A new name
// and a free-text query declare that they complete nothing; every other
// argument completes from the local configuration or a fixed list.
func TestEveryArgumentAndFilterCompletes(t *testing.T) {
	// exempt are the leaf commands whose argument is a new provider or model
	// or a free-text query.
	exempt := map[string]bool{"provider add": true, "model add": true, "model list": true, "model search": true}
	nothing := reflect.ValueOf(invocation.CompleteNothing).Pointer()
	filters := []string{"provider", "client", "model", "protocol", "family", "capability", "depth"}
	for path, command := range leaves(NewRoot(&App{Clients: clients.Registry()})) {
		if len(strings.Fields(command.Use)) > 1 {
			complete := command.ValidArgsFunction
			switch {
			case complete == nil && len(command.ValidArgs) == 0:
				t.Errorf("aigw %s: its argument has no completion", path)
			case exempt[path] != (complete != nil && reflect.ValueOf(complete).Pointer() == nothing):
				t.Errorf("aigw %s: only a new name or a query completes nothing, and it always does", path)
			}
		}
		for _, name := range filters {
			if flag := command.Flags().Lookup(name); flag != nil && !flag.Hidden {
				if _, declared := command.GetFlagCompletionFunc(name); !declared {
					t.Errorf("aigw %s --%s has no completion", path, name)
				}
			}
		}
	}
}
