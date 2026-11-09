// Package self implements `aigw self`, which installs, updates, rolls back,
// removes and reports the portable AIGW program itself. It changes no client
// conversation, model or Token; Homebrew owns a program that it installed.
package self

import (
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// NewCommand constructs `aigw self`, the group of the program's own lifecycle
// commands. Like the other groups, bare `aigw self` shows its help.
func NewCommand(runtime invocation.Context) *cobra.Command {
	root := &cobra.Command{Use: "self", Short: "Install, update, roll back, remove or inspect this program"}
	root.AddCommand(newInstallCommand(runtime), newUpdateCommand(runtime), newRollbackCommand(runtime),
		newUninstallCommand(runtime), newStatusCommand(runtime))
	return root
}

// homebrewManaged refuses a program file that a Homebrew receipt owns, naming
// the Homebrew command that does what the operator asked for.
func homebrewManaged(fix, impact string, err error) error {
	return terminal.ProblemError("Homebrew manages this AIGW program", "A Homebrew receipt owns the program file.", impact, fix, err)
}

// programProblem presents a program transition, or its preview, that did not
// complete. An error that states its own fix reaches the user unchanged; any
// other keeps its cause private behind the problem.
func programProblem(title, evidence, impact, fix string, err error) error {
	if terminal.StatesFix(err) {
		return err
	}
	return terminal.ProblemError(title, evidence, impact, fix, err)
}
