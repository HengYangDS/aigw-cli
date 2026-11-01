package invocation

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestApplyCommandRepeatsTheGivenFlags shows that a preview's apply command
// repeats every given flag, a false boolean with its value and each value of a
// list, without --dry-run and --json, and falls back to prose for a word that
// would need quoting.
func TestApplyCommandRepeatsTheGivenFlags(t *testing.T) {
	command := &cobra.Command{Use: "demo"}
	command.Flags().Bool("dry-run", false, "")
	command.Flags().Bool("json", false, "")
	command.Flags().Bool("on", false, "")
	command.Flags().Bool("off", true, "")
	command.Flags().StringSlice("name", nil, "")
	command.Flags().String("upstream-model", "", "")
	if err := command.ParseFlags([]string{"--on", "--off=false", "--name", "a", "--name", "b", "--upstream-model", "m", "--dry-run", "--json"}); err != nil {
		t.Fatal(err)
	}
	if got, want := ApplyCommand(command, []string{"item"}), "demo item --name a --name b --off=false --on --upstream-model m"; got != want {
		t.Fatalf("ApplyCommand() = %q, want %q", got, want)
	}
	if got := ApplyCommand(command, []string{"two words"}); got != applyInProse {
		t.Fatalf("ApplyCommand() with a quoted word = %q", got)
	}
}
