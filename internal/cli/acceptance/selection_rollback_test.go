package cli_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/domain"
)

// restoredSelection is one entry of a rollback document's selections.
type restoredSelection struct {
	Client   string `json:"client"`
	Model    string `json:"model"`
	Replaces string `json:"replaces"`
}

// A rollback restores each client's selection as of the restored
// configuration, a model chosen in the client since then included. It is the
// one command besides `aigw use` that replaces such a choice, so its preview
// and its result name the selection that it restores and the one that this
// replaces, and neither reports the choice as followed.
func TestRollbackNamesTheSelectionsThatItRestores(t *testing.T) {
	app, output, target := codexChoiceApp(t)
	if err := cli.Execute(app, []string{"client", "edit", codex.ID, "--request-retries", "2"}); err != nil {
		t.Fatalf("change Codex's binding: %v\n%s", err, output)
	}
	chooseInCodex(t, target, "gpt-sol", "gpt-terra")
	want := []restoredSelection{{Client: "codex", Model: "gateway/gpt-sol", Replaces: "gateway/gpt-terra"}}

	before := ownedFileSnapshot(t, filepath.Dir(target), app.Config.Path())
	preview := []string{"config", "rollback", "--last-change", "--dry-run"}
	output.Reset()
	if err := cli.Execute(app, preview); err != nil ||
		!strings.Contains(output.String(), "Selections\n") || !strings.Contains(output.String(), "gateway/gpt-sol, in place of gateway/gpt-terra") ||
		strings.Contains(output.String(), "Followed selection") {
		t.Fatalf("%v: %v\n%s", preview, err, output)
	}
	for _, command := range [][]string{append(preview, "--json"), {"config", "rollback", "--last-change", "--json"}} {
		output.Reset()
		if err := cli.Execute(app, command); err != nil {
			t.Fatalf("%v: %v\n%s", command, err, output)
		}
		var result struct {
			Selections []restoredSelection `json:"selections"`
			Followed   []followed          `json:"followed_selections"`
		}
		if err := json.Unmarshal(output.Bytes(), &result); err != nil || !slices.Equal(result.Selections, want) || len(result.Followed) != 0 {
			t.Fatalf("%v restores %+v and follows %+v: %v\n%s", command, result.Selections, result.Followed, err, output)
		}
		if slices.Contains(command, "--dry-run") {
			if after := ownedFileSnapshot(t, filepath.Dir(target), app.Config.Path()); !equalSnapshots(after, before) {
				t.Fatal("the rollback preview changed client files or the configuration")
			}
		}
	}
	written := string(readFile(t, target))
	if !strings.Contains(written, `model = "gpt-sol" # managed by AIGW`) || strings.Contains(written, "request_max_retries") {
		t.Fatalf("the rollback did not restore Codex's file:\n%s", written)
	}
	if binding := selection(t, app, codex.ID); binding.Model != domain.NewModelRef("gateway", "gpt-sol") || binding.RetryPolicy != nil {
		t.Fatalf("the rollback did not restore Codex's binding: %#v", binding)
	}
	syncAndCheckPass(t, app, output, codex.ID)
}

// A rollback that would write over a selection that AIGW does not follow is
// refused as every other command is: with the step that restores the recorded
// selection, and nothing written.
func TestRollbackPassesOnTheStepOfASelectionThatIsNotFollowed(t *testing.T) {
	app, output, target := codexChoiceApp(t)
	if err := cli.Execute(app, []string{"use", codex.ID, "gateway/gpt-terra"}); err != nil {
		t.Fatalf("select another model: %v\n%s", err, output)
	}
	chooseInCodex(t, target, "gpt-terra", "gpt-ghost")
	refusedWithRestoringCommand(t, app, output, codex.ID, filepath.Dir(target), "aigw use codex gateway/gpt-terra", [][]string{
		{"config", "rollback", "--last-change", "--dry-run", "--json"},
		{"config", "rollback", "--last-change", "--json"},
	})
}

// A rollback does not write the file of a client that waits for its Token,
// so a model chosen in that client stays in its file, and the rollback names
// for that client only a stored selection that it changes: here none.
func TestRollbackLeavesAChoiceOfAClientThatWaitsForItsToken(t *testing.T) {
	app, output, _, target := shippedHermesAppWith(t, true)
	if err := cli.Execute(app, []string{"provider", "edit", "ucloud", "--label", "Other"}); err != nil {
		t.Fatalf("change the configuration: %v\n%s", err, output)
	}
	if err := app.Secrets.Delete("dmxapi"); err != nil {
		t.Fatal(err)
	}
	chosen := hermesNativeSelection(t, target, "aigw-dmxapi-anthropic", "claude-opus-5-5", nil)
	for _, command := range [][]string{
		{"config", "rollback", "--last-change", "--dry-run", "--json"},
		{"config", "rollback", "--last-change", "--json"},
	} {
		output.Reset()
		if err := cli.Execute(app, command); err != nil {
			t.Fatalf("%v: %v\n%s", command, err, output)
		}
		var result struct {
			Selections []restoredSelection `json:"selections"`
		}
		if err := json.Unmarshal(output.Bytes(), &result); err != nil || slices.ContainsFunc(result.Selections, func(entry restoredSelection) bool { return entry.Client == hermes.ID }) {
			t.Fatalf("%v names a selection of Hermes that it does not restore: %+v, %v\n%s", command, result.Selections, err, output)
		}
	}
	if !bytes.Equal(readFile(t, target), chosen) {
		t.Fatal("the rollback wrote the file of a client that waits for its Token")
	}
}
