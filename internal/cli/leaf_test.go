package cli

import (
	"strings"
	"testing"

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
