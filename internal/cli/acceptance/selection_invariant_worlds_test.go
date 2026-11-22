package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/domain"

	"github.com/pelletier/go-toml/v2"
)

// run executes one command of a world's preparation, which must succeed.
func (w selectionWorld) run(t *testing.T, args ...string) {
	t.Helper()
	w.output.Reset()
	if err := cli.Execute(w.app, args); err != nil {
		t.Fatalf("prepare with %v: %v\n%s", args, err, w.output)
	}
}

// chose notes that a file of the world now shows selection in place of what
// its record names. Two files that chose different models leave AIGW nothing
// that it could record for the client.
func (w selectionWorld) chose(selection string) selectionWorld {
	if w.choice != "" && w.choice != selection {
		w.blocked = "two files select different models"
	}
	w.choice = selection
	return w
}

// losingFile removes the world's file at index, which prevents the record of
// the client: AIGW does not create a file of the client that is missing.
func (w selectionWorld) losingFile(t *testing.T, index int) selectionWorld {
	t.Helper()
	if err := os.Remove(w.files[index]); err != nil {
		t.Fatal(err)
	}
	w.blocked = "a file of the client is missing"
	return w
}

// losingRecord removes AIGW's record of the world's file at index, wherever
// it is kept, which prevents the record of the client: without it AIGW
// cannot tell its own values in the file from the person's.
func (w selectionWorld) losingRecord(t *testing.T, index int) selectionWorld {
	t.Helper()
	path := w.files[index]
	if w.recordOf(t, path) == nil {
		t.Fatalf("%s has no record to remove", path)
	}
	_ = os.Remove(path + adapter.StateSuffix)
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		_ = os.Remove(resolved + adapter.StateSuffix)
	}
	if w.recordOf(t, path) != nil {
		t.Fatalf("the record of %s is still there", path)
	}
	w.blocked = "a record of the client's files is missing"
	return w
}

// blockedWithAChoice accepts the states in which a file chose a model and
// something prevents its record.
func blockedWithAChoice(state selectionWorld) bool {
	return state.blocked != "" && state.choice != ""
}

// codexWorld is Codex on count files that show gateway/gpt-sol with its
// record, after one earlier change that a rollback can undo.
func codexWorld(t *testing.T, count int) selectionWorld {
	t.Helper()
	app, output, targets := recCodexFiles(t, count)
	world := selectionWorld{
		app: app, output: output, client: codex.ID, account: "gateway", files: targets,
		shows:    codexSelects,
		storedAs: func(runtime domain.Runtime) string { return runtime.UpstreamModel },
		// The lines and the provider table that AIGW writes carry these marks.
		keepsAIGWsPart: func(file []byte) bool {
			return bytes.Contains(file, []byte("managed by AIGW")) || bytes.Contains(file, []byte("AIGW managed provider"))
		},
	}
	world.follows = func(t *testing.T, path string) bool {
		t.Helper()
		record := world.recordOf(t, path)
		if record == nil {
			return false
		}
		var state struct {
			ProjectedModel string `json:"projected_model"`
		}
		if err := json.Unmarshal(record, &state); err != nil {
			t.Fatal(err)
		}
		return state.ProjectedModel == world.shows(t, path)
	}
	world.reaches = codexReaches
	world.run(t, "provider", "edit", "spare", "--label", "Earlier")
	world.restored = "gpt-sol"
	return world
}

// codexReaches names the endpoint of the provider entry that AIGW wrote into
// path, which the file's model_provider selects, or "" when the file holds no
// such entry. The endpoint is the runtime's own, so it compares with the
// stored selection's.
func codexReaches(t *testing.T, path string) string {
	t.Helper()
	var document struct {
		ModelProvider  string `toml:"model_provider"`
		ModelProviders map[string]struct {
			BaseURL string `toml:"base_url"`
		} `toml:"model_providers"`
	}
	if err := toml.Unmarshal(readFile(t, path), &document); err != nil || document.ModelProvider == "" {
		return ""
	}
	return document.ModelProviders[document.ModelProvider].BaseURL
}

// codexRestoring is codexWorld whose last change selected gpt-sol in place of
// earlier, so that a rollback restores earlier.
func codexRestoring(t *testing.T, count int, earlier string) selectionWorld {
	t.Helper()
	world := codexWorld(t, count)
	world.run(t, "use", "codex", "gateway/"+earlier)
	// The binding that the last change replaces, which a rollback restores.
	restored := selection(t, world.app, codex.ID)
	world.restoredBinding = &restored
	world.run(t, "use", "codex", "gateway/gpt-sol")
	world.restored = earlier
	return world
}

// codexPending leaves Codex with a stored selection that waits for its Token:
// AIGW wrote gpt-terra, the Token is lost, and a rollback restores gpt-sol,
// which cannot be written. Every file and its record show gpt-terra. That
// rollback is the last change, and the configuration that it replaced, which
// it keeps as the backup, selects gpt-terra: so the construction fixes what
// the next rollback restores.
func codexPending(t *testing.T, count int) selectionWorld {
	t.Helper()
	world := codexWorld(t, count)
	world.run(t, "use", "codex", "gateway/gpt-terra")
	world.setToken(t, false)
	world.run(t, "config", "rollback", "--last-change", "--json")
	if got := selection(t, world.app, codex.ID).Model.String(); got != "gateway/gpt-sol" {
		t.Fatalf("the rollback saved %s", got)
	}
	world.restored = "gpt-terra"
	return world
}

// chosenBeforeARollback is a state in which the first file chose gpt-terra
// and a rollback restores earlier; without names the other file to remove.
func chosenBeforeARollback(count int, earlier string, without bool) func(t *testing.T) selectionWorld {
	return func(t *testing.T) selectionWorld {
		world := codexRestoring(t, count, earlier).picking(t, 0, basicString, "gpt-terra")
		if without {
			return world.losingFile(t, 1)
		}
		return world
	}
}

// withRequestRetries is what `aigw client edit codex --request-retries` leaves:
// the stored binding's retry budget gives requests that many retries, and
// leaves the stream's budget as it was.
func withRequestRetries(retries uint64) func(after selectionSnapshot) string {
	return func(after selectionSnapshot) string {
		policy := after.binding.RetryPolicy
		if policy != nil && policy.RequestMaxRetries != nil && *policy.RequestMaxRetries == retries {
			return ""
		}
		return "the client's retry budget does not give requests " + strconv.FormatUint(retries, 10) + " retries"
	}
}

// forwardingTo is what `aigw client edit hermes --via` leaves: the stored
// binding forwards to that address.
func forwardingTo(address string) func(after selectionSnapshot) string {
	return func(after selectionSnapshot) string {
		if after.binding.ForwardingEndpoint == address {
			return ""
		}
		return "the stored selection forwards to " + strconv.Quote(after.binding.ForwardingEndpoint) + ", not to " + strconv.Quote(address)
	}
}

func codexCommands() []selectionCommand {
	return []selectionCommand{
		{name: "another provider", args: []string{"provider", "edit", "spare", "--label", "Other", "--json"}},
		{name: "sync", args: []string{"sync", "--json"}},
		{name: "own setting", args: []string{"client", "edit", "codex", "--request-retries", "2", "--json"}, leaves: withRequestRetries(2)},
		{name: "select a third model", args: []string{"use", "codex", "gateway/gpt-luna", "--json"}, explicit: true, stores: "gateway"},
		{name: "select the stored model", args: []string{"use", "codex", "gateway/gpt-sol", "--json"}, explicit: true, stores: "gateway"},
		{name: "rollback", args: []string{"config", "rollback", "--last-change", "--json"}, explicit: true, restores: true},
		{name: "disable", args: []string{"client", "disable", "codex", "--json"}, withdraws: true},
		// The removal of the model that the stored selection names, beside a
		// choice that cannot be recorded: whatever it does, the sentence
		// holds. The run logs its refusal; no wording is asserted.
		{name: "remove the stored model", args: []string{"model", "remove", "gateway/gpt-sol", "--json"}, where: blockedWithAChoice},
		{name: "sync preview", args: []string{"sync", "--dry-run", "--json"}, preview: true},
		{name: "own setting preview", args: []string{"client", "edit", "codex", "--request-retries", "2", "--dry-run", "--json"}, preview: true},
		{name: "select preview", args: []string{"use", "codex", "gateway/gpt-luna", "--dry-run", "--json"}, preview: true, stores: "gateway"},
		{name: "rollback preview", args: []string{"config", "rollback", "--last-change", "--dry-run", "--json"}, preview: true},
		{name: "disable preview", args: []string{"client", "disable", "codex", "--dry-run", "--json"}, preview: true},
	}
}

// codexOneFileMatrix is Codex on one file. Every spelling of a pick runs as
// a plain choice and as a pick of the stored model beside an earlier record.
func codexOneFileMatrix() selectionMatrix {
	states := map[string]func(t *testing.T) selectionWorld{
		plainState:            func(t *testing.T) selectionWorld { return codexWorld(t, 1) },
		"waits for its Token": func(t *testing.T) selectionWorld { return codexPending(t, 1) },
		"chosen, the record missing": func(t *testing.T) selectionWorld {
			return codexWorld(t, 1).picking(t, 0, basicString, "gpt-terra").losingRecord(t, 0)
		},
		"chosen, a rollback restores a third model":  chosenBeforeARollback(1, "gpt-luna", false),
		"chosen, a rollback restores the chosen one": chosenBeforeARollback(1, "gpt-terra", false),
	}
	for _, spelling := range codexSpellings() {
		states["chosen"+spelling.name] = func(t *testing.T) selectionWorld {
			return codexWorld(t, 1).picking(t, 0, spelling, "gpt-terra")
		}
		states["shows the stored model beside an earlier record"+spelling.name] = func(t *testing.T) selectionWorld {
			return codexPending(t, 1).picking(t, 0, spelling, "gpt-sol")
		}
	}
	return selectionMatrix{client: "codex one file", commands: codexCommands(), states: states}
}

// codexTwoFilesMatrix is Codex on two files. Every spelling of a pick runs as
// a choice in one file and as a pick of the stored model beside an earlier
// record; a pick in both runs in one spelling and with the two files in
// different spellings.
func codexTwoFilesMatrix() selectionMatrix {
	chosenInFirst := func(t *testing.T) selectionWorld {
		return codexWorld(t, 2).picking(t, 0, basicString, "gpt-terra")
	}
	chosenInBoth := func(first, second codexSpelling) func(t *testing.T) selectionWorld {
		return func(t *testing.T) selectionWorld {
			return codexWorld(t, 2).picking(t, 0, first, "gpt-terra").picking(t, 1, second, "gpt-terra")
		}
	}
	states := map[string]func(t *testing.T) selectionWorld{
		plainState:       func(t *testing.T) selectionWorld { return codexWorld(t, 2) },
		"chosen in both": chosenInBoth(basicString, basicString),
		"chosen in both, the first as a literal string":     chosenInBoth(literalString, basicString),
		"chosen in both, the second as a literal string":    chosenInBoth(basicString, literalString),
		"chosen in both, without spaces and with a comment": chosenInBoth(withoutSpaces, withAComment),
		"different picks": func(t *testing.T) selectionWorld {
			return chosenInFirst(t).picking(t, 1, basicString, "gpt-luna")
		},
		"chosen, the other file missing": func(t *testing.T) selectionWorld {
			return chosenInFirst(t).losingFile(t, 1)
		},
		"chosen, the other record missing": func(t *testing.T) selectionWorld {
			return chosenInFirst(t).losingRecord(t, 1)
		},
		"recorded, the other file missing": func(t *testing.T) selectionWorld {
			return codexWorld(t, 2).losingFile(t, 1)
		},
		"chosen, a rollback restores a third model":                          chosenBeforeARollback(2, "gpt-luna", false),
		"chosen, a rollback restores the chosen one":                         chosenBeforeARollback(2, "gpt-terra", false),
		"chosen, the other file missing, a rollback restores a third model":  chosenBeforeARollback(2, "gpt-luna", true),
		"chosen, the other file missing, a rollback restores the chosen one": chosenBeforeARollback(2, "gpt-terra", true),
	}
	for _, spelling := range codexSpellings() {
		states["chosen in one"+spelling.name] = func(t *testing.T) selectionWorld {
			return codexWorld(t, 2).picking(t, 0, spelling, "gpt-terra")
		}
		states["one shows the stored model beside an earlier record"+spelling.name] = func(t *testing.T) selectionWorld {
			return codexPending(t, 2).picking(t, 0, spelling, "gpt-sol")
		}
	}
	return selectionMatrix{client: "codex two files", commands: codexCommands(), states: states}
}
