package use

import (
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/terminal"
)

// Without a terminal, or with --json, a missing argument is refused and
// named, never asked for; an unknown client, a model argument without a
// provider and an unknown provider are refused with what they could mean,
// before any Token is read.
func TestUseRefusesArgumentsBeforeAnyChange(t *testing.T) {
	runtime, _, _ := configuredRuntime(t)
	runtime.Secrets = credentialFreeStore{}
	runtime.Prompt = &promptStub{selected: codex.ID}
	for _, test := range []struct {
		name        string
		interactive bool
		args        []string
		want, fix   string
	}{
		{"nothing without a terminal", false, nil, "aigw use without a terminal needs a <client> and a <provider/model> argument", "aigw use --help"},
		{"a model without a terminal", false, []string{codex.ID}, "aigw use without a terminal needs a <provider/model> argument after the client", "aigw use --help"},
		{"a client without a terminal", false, []string{"gateway/gpt-test"}, "aigw use without a terminal needs a <client> argument before the model", "aigw use --help"},
		{"a model with --json in a terminal", true, []string{codex.ID, "--json"}, "aigw use with --json needs a <provider/model> argument after the client", "aigw use --help"},
		{"too many arguments", true, []string{codex.ID, "gateway/gpt-test", "extra"}, "aigw use takes <client> and <provider/model>, not 3 arguments", "aigw use --help"},
		{"an unknown client", true, []string{"codx", "gateway/gpt-test"}, `unknown client "codx"; did you mean codex?`, "aigw client list"},
		{"a malformed model", true, []string{codex.ID, "/gpt-test"}, "has an empty provider or model ID", "aigw use --help"},
		{"a model without a provider", false, []string{codex.ID, "gpt-test"}, `"gpt-test" names no provider; it could mean gateway/gpt-test`, "aigw model list gpt-test"},
		{"an unknown provider", false, []string{codex.ID, "gatewy/gpt-test"}, `unknown provider "gatewy"; did you mean gateway?`, "aigw provider list"},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime.Interactive = test.interactive
			err := execute(runtime, test.args...)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
			if fix := terminal.ProblemFor(err).Fix; fix != test.fix {
				t.Fatalf("fix = %q, want %q", fix, test.fix)
			}
		})
	}
}
