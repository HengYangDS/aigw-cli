package cli_test

import (
	"aigw-cli/internal/cli"
	"aigw-cli/internal/terminal"
	"io"
	"strings"
	"testing"
)

// The accessible prompts end without an answer when their input ends, as
// Ctrl-D or a script whose input runs out ends it, instead of choosing the
// first choice: use's client and model prompts, the guided setup and the rename
// selection each say that nothing changed, name their command again and leave
// the configuration as it was.
func TestAccessiblePromptsChangeNothingAtTheEndOfInput(t *testing.T) {
	for _, test := range []struct {
		args       []string
		configured bool
		title, fix string
	}{
		{args: []string{"use"}, configured: true, title: "The client prompt was cancelled", fix: "aigw use"},
		{args: []string{"use", "claude"}, configured: true, title: "The model prompt for Claude was cancelled", fix: "aigw use claude"},
		{args: []string{"setup"}, title: "The provider ID prompt was cancelled", fix: "aigw setup"},
	} {
		t.Run(strings.Join(append([]string{"aigw"}, test.args...), " "), func(t *testing.T) {
			app, _, secretStore, _, _ := testApp(t, "")
			if test.configured {
				if err := app.Config.Save(twoModelConfig()); err != nil {
					t.Fatal(err)
				}
				_ = secretStore.Set("one", "one-token")
				_ = secretStore.Set("two", "two-token")
			}
			before, err := app.Config.CaptureSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			app.Interactive = true
			app.Prompt = terminal.NewPrompter(strings.NewReader(""), io.Discard, true)
			problem := terminal.ProblemFor(cli.Execute(app, test.args))
			if problem.Title != test.title || problem.Impact != "Nothing changed." || problem.Fix != test.fix {
				t.Fatalf("the end of input = %#v", problem)
			}
			after, err := app.Config.CaptureSnapshot()
			if err != nil || !after.Config.Equal(before.Config) {
				t.Fatalf("the end of input changed the configuration: %v", err)
			}
		})
	}
}
