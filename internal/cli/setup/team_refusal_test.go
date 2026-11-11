package setup

import (
	"strings"
	"testing"

	"aigw-cli/internal/terminal"
)

// Every refusal of a team file names the entry of the file to change, never
// `aigw check`, which checks clients and would not read the file.
func TestARefusedTeamFileNamesTheEntryToChange(t *testing.T) {
	const provider = "[[providers]]\nid = \"a\"\nlabel = \"A\"\nendpoints = {responses = \"https://a.test/v1\"}\n"
	for _, test := range []struct{ name, file, entry string }{
		{"an undefined field", "version = 8\n[[providers]]\nid = \"a\"\nbogus = 1\n", "the field `providers.bogus` on line 4"},
		{"a credential field", "version = 8\n" + provider + "token = \"x\"\n", "the field `providers.token`"},
		{"an invalid provider ID", "version = 8\n[[providers]]\nid = \"A b\"\nlabel = \"A\"\nendpoints = {responses = \"https://a.test/v1\"}\n", "provider `A b`"},
		{"an empty label", "version = 8\n[[providers]]\nid = \"a\"\nendpoints = {responses = \"https://a.test/v1\"}\n", "provider `a`"},
		{"an invalid endpoint", "version = 8\n[[providers]]\nid = \"a\"\nlabel = \"A\"\nendpoints = {responses = \"ftp://a.test\"}\n", "provider `a`"},
		{"an unknown protocol", "version = 8\n" + provider + "[[providers.models]]\nid = \"m\"\nprotocols = {bogus = []}\n", "model `a/m`"},
		{"an unknown capability", "version = 8\n" + provider + "[[providers.models]]\nid = \"m\"\nprotocols = {responses = [\"bogus\"]}\n", "model `a/m`"},
		{"an invalid family", "version = 8\n" + provider + "[[providers.models]]\nid = \"m\"\nfamily = \"a b\"\nprotocols = {responses = []}\n", "model `a/m`"},
		{"a family without a label", "version = 8\n" + provider + "[[families]]\nid = \"f\"\n", "family `f`"},
		{"a recommendation of an unknown client", "version = 8\n" + provider + "[recommendations.nobody]\nprimary = {model = \"a/m\"}\n", "the `nobody` recommendation"},
		{"a recommendation of an unknown model", "version = 8\n" + provider + "[recommendations.codex]\nprimary = {model = \"a/m\"}\n", "the `codex` recommendation"},
		{"a model on a protocol without an endpoint", "version = 8\n" + provider + "[[providers.models]]\nid = \"m\"\nprotocols = {chat = []}\n", "model `a/m`"},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := newGuidedHome(t)
			home.runtime.Interactive = false
			err := home.run(t, &guidedPrompt{}, "--from", writeTeamFile(t, test.file), "--dry-run")
			if err == nil {
				t.Fatalf("the file was accepted:\n%s", home.out)
			}
			problem := terminal.ProblemFor(err)
			want := "Correct " + test.entry + " in the team file, then run the same command again."
			if problem.Fix != want {
				t.Fatalf("fix = %q, want %q (title %q)", problem.Fix, want, problem.Title)
			}
			if strings.Contains(problem.Fix, "aigw check") {
				t.Fatalf("the fix names aigw check: %q", problem.Fix)
			}
		})
	}
}
