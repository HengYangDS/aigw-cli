package setup

import (
	"strings"
	"testing"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/terminal"
)

// Every refusal of a team file names the entry of the file to change, never
// `aigw check`, which checks clients and would not read the file.
func TestARefusedTeamFileNamesTheEntryToChange(t *testing.T) {
	const provider = "[[providers]]\nid = \"a\"\nlabel = \"A\"\nendpoints = {responses = \"https://a.test/v1\"}\n"
	for _, test := range []struct{ name, file, entry string }{
		{"an undefined field", "version = 9\n[[providers]]\nid = \"a\"\nbogus = 1\n", "the field `providers.bogus` on line 4"},
		{"a credential field", "version = 9\n" + provider + "token = \"x\"\n", "the field `providers.token`"},
		{"an invalid provider ID", "version = 9\n[[providers]]\nid = \"A b\"\nlabel = \"A\"\nendpoints = {responses = \"https://a.test/v1\"}\n", "provider `A b`"},
		{"an empty label", "version = 9\n[[providers]]\nid = \"a\"\nendpoints = {responses = \"https://a.test/v1\"}\n", "provider `a`"},
		{"an invalid endpoint", "version = 9\n[[providers]]\nid = \"a\"\nlabel = \"A\"\nendpoints = {responses = \"ftp://a.test\"}\n", "provider `a`"},
		{"an unknown protocol", "version = 9\n" + provider + "[[providers.models]]\nid = \"m\"\nprotocols = {bogus = []}\n", "model `a/m`"},
		{"an unknown capability", "version = 9\n" + provider + "[[providers.models]]\nid = \"m\"\nprotocols = {responses = [\"bogus\"]}\n", "model `a/m`"},
		{"an invalid family", "version = 9\n" + provider + "[[providers.models]]\nid = \"m\"\nfamily = \"a b\"\nprotocols = {responses = []}\n", "model `a/m`"},
		{"a family without a label", "version = 9\n" + provider + "[[families]]\nid = \"f\"\n", "family `f`"},
		{"a recommendation of an unknown client", "version = 9\n" + provider + "[recommendations.nobody]\nprimary = {model = \"a/m\"}\n", "the `nobody` recommendation"},
		{"a recommendation of an unknown model", "version = 9\n" + provider + "[recommendations.codex]\nprimary = {model = \"a/m\"}\n", "the `codex` recommendation"},
		{"a model on a protocol without an endpoint", "version = 9\n" + provider + "[[providers.models]]\nid = \"m\"\nprotocols = {chat = []}\n", "model `a/m`"},
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

// A team file whose model would share a label with another model of its
// provider is refused, naming where to correct it: a model that only the
// local configuration holds is relabelled or removed by its own command, and
// two of the file's own models in the file.
func TestATeamFileThatSharesALabelNamesWhereToCorrectIt(t *testing.T) {
	const file = "version = 9\n[[providers]]\nid = \"a\"\nlabel = \"A\"\nendpoints = {responses = \"https://a.test/v1\"}\n" +
		"[[providers.models]]\nid = \"new-id\"\nfamily = \"f\"\nprotocols = {responses = []}\n" +
		"[[families]]\nid = \"f\"\nlabel = \"Shared\"\n"
	home := newGuidedHome(t)
	home.runtime.Interactive = false
	cfg, err := home.runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	// A client keeps the model that the file replaced; its family has the
	// label of the file's.
	cfg.Providers["a"] = domain.Provider{Label: "A", Endpoints: domain.Endpoints{Responses: "https://a.test/v1"}}
	cfg.Families["old-id"] = domain.Family{Label: "Shared"}
	old := domain.Model{Provider: "a", ID: "old-id", Family: "old-id", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolResponses: {}}}
	cfg.Models[old.Ref()] = old
	if err := home.runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	err = home.run(t, &guidedPrompt{}, "--from", writeTeamFile(t, file), "--keep-local", "--dry-run")
	problem := terminal.ProblemFor(err)
	want := "Give a/old-id a label of its own with `aigw model edit a/old-id --label <label>`, or remove it with `aigw model remove a/old-id`, then run the same command again."
	if err == nil || problem.Fix != want || !strings.Contains(problem.Title, "a/new-id") {
		t.Fatalf("refusal = %v; fix %q, title %q", err, problem.Fix, problem.Title)
	}
	// The fix it names works on the local configuration as it is.
	cfg, err = home.runtime.Config.Load()
	if err != nil {
		t.Fatalf("the local configuration no longer loads: %v", err)
	}
	old.Label = "A · Old"
	cfg.Models[old.Ref()] = old
	if err := home.runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := home.run(t, &guidedPrompt{}, "--from", writeTeamFile(t, file), "--dry-run"); err != nil {
		t.Fatalf("the file after the fix = %v", err)
	}
	// Two of the file's own models are corrected in the file.
	twice := file + "[[providers]]\nid = \"b\"\nlabel = \"B\"\nendpoints = {responses = \"https://b.test/v1\"}\n"
	twice = strings.Replace(twice, "[[families]]", "[[providers.models]]\nid = \"other-id\"\nfamily = \"f\"\nprotocols = {responses = []}\n\n[[families]]", 1)
	err = home.run(t, &guidedPrompt{}, "--from", writeTeamFile(t, twice), "--dry-run")
	if problem := terminal.ProblemFor(err); err == nil || problem.Fix != "Give a/new-id or a/other-id a label of its own in the team file, then run the same command again." {
		t.Fatalf("two models of the file = %v; fix %q", err, problem.Fix)
	}
}
