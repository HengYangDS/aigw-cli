package invocation

import (
	"path/filepath"

	"aigw-cli/internal/app"
	"aigw-cli/internal/adapter/discovery"
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

// TestGivenPathNamesHomeAsTheShellDoes shows that a path inside home prints as
// ~/… with "/" on every platform, and a path outside home, or home itself,
// prints as configured.
func TestGivenPathNamesHomeAsTheShellDoes(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	outside := filepath.Join(t.TempDir(), "elsewhere", "aigw")
	runtime := Context{Dependencies: app.Dependencies{Host: discovery.Host{Home: home}}}
	for _, test := range []struct{ path, want string }{
		{filepath.Join(home, ".local", "bin", "aigw"), "~/.local/bin/aigw"},
		{outside, outside},
		{home, home},
		{"", ""},
	} {
		if got := GivenPath(runtime, test.path); got != test.want {
			t.Errorf("GivenPath(%q) = %q, want %q", test.path, got, test.want)
		}
	}
}
