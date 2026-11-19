package invocation

import (
	"errors"
	"fmt"
	"testing"

	"aigw-cli/internal/terminal"

	"github.com/spf13/cobra"
)

// A cancelled prompt says that nothing changed, and after it what PromptLost
// says the command had obtained and did not keep; an error that is not a
// prompt's passes through PromptLost and CancelledPrompt unchanged.
func TestCancelledPromptSaysWhatTheCommandDidNotKeep(t *testing.T) {
	cmd := &cobra.Command{Use: "setup"}
	cancelled := Prompted("The Token prompt", fmt.Errorf("%w: no input received", terminal.ErrInputCancelled))
	for _, test := range []struct {
		name   string
		err    error
		impact string
	}{
		{"nothing obtained", cancelled, "Nothing changed."},
		{"a Token obtained", PromptLost(cancelled, "No Token that this run accepted was stored."), "Nothing changed. No Token that this run accepted was stored."},
	} {
		t.Run(test.name, func(t *testing.T) {
			problem := terminal.ProblemFor(CancelledPrompt(cmd, nil, test.err))
			if problem.Title != "The Token prompt was cancelled" || problem.Impact != test.impact {
				t.Fatalf("problem %#v", problem)
			}
		})
	}
	other := errors.New("the store failed")
	if got := CancelledPrompt(cmd, nil, PromptLost(other, "lost")); !errors.Is(got, other) || got.Error() != other.Error() {
		t.Fatalf("an error that is not a prompt's became %v", got)
	}
}
