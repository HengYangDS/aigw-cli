package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

// A usage error in a command run with --json prints the JSON failure
// document, wherever --json stands: before the flag or word that Cobra
// refuses, after it, or at the root, which has no --json of its own.
func TestAUsageErrorPrintsTheJSONFailureWherever(t *testing.T) {
	for _, args := range [][]string{
		{"status", "--json", "--bogus"},
		{"status", "--bogus", "--json"},
		{"status", "--bogus", "--json=true"},
		{"status", "extra", "--json"},
		{"status", "--json", "extra"},
		{"--json", "status", "--bogus"},
		{"nothing", "--json"},
		{"--json", "nothing"},
		{"model", "show", "--json", "--bogus"},
	} {
		t.Run(args[0]+" "+args[len(args)-1], func(t *testing.T) {
			app := configuredApp(t)
			out := &bytes.Buffer{}
			app.Out, app.Err = out, out
			if err := Execute(app, args); err == nil {
				t.Fatal("the usage error was not refused")
			}
			var document struct {
				OK         bool   `json:"ok"`
				Error      string `json:"error"`
				NextAction string `json:"next_action"`
			}
			if err := json.Unmarshal(out.Bytes(), &document); err != nil || document.OK || document.Error == "" || document.NextAction == "" {
				t.Fatalf("%v prints %q, want a JSON failure document (%v)", args, out, err)
			}
		})
	}
	// --json=false and a --json after the arguments' end ask for text.
	for _, args := range [][]string{{"status", "--bogus", "--json=false"}, {"status", "--bogus", "--", "--json"}} {
		app := configuredApp(t)
		out := &bytes.Buffer{}
		app.Out, app.Err = out, out
		_ = Execute(app, args)
		if json.Valid(out.Bytes()) {
			t.Errorf("%v printed JSON: %q", args, out)
		}
	}
}
