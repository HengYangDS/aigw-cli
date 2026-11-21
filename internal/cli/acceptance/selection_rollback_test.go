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

// A rollback to another model than the one that the client's file shows,
// while that model's provider has no Token, cannot write the file: it saves
// the restored selection, which waits for the Token as any selection does,
// names it with the chosen model that it replaces, and brings AIGW's record
// of the file to what the file shows. So the model that the record named
// before, chosen in the client again, is kept by the synchronization after
// the Token is stored. Holds the cell chosen/restores of the lane's state
// table.
func TestRollbackOverAChoiceWithoutATokenSavesTheSelectionAndRecordsTheFile(t *testing.T) {
	app, output, _, target := shippedHermesAppWith(t, true)
	if err := cli.Execute(app, []string{"provider", "edit", "ucloud", "--label", "Other"}); err != nil {
		t.Fatalf("change the configuration: %v\n%s", err, output)
	}
	if err := app.Secrets.Delete("dmxapi"); err != nil {
		t.Fatal(err)
	}
	chosen := hermesNativeSelection(t, target, "aigw-dmxapi-anthropic", "claude-opus-5-5", nil)
	record := readFile(t, target+".aigw-state.json")
	want := []restoredSelection{{Client: "hermes", Model: "dmxapi/gpt-6.2-sol", Replaces: "dmxapi/claude-opus-5-5"}}
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
			Followed   []followed          `json:"followed_selections"`
		}
		if err := json.Unmarshal(output.Bytes(), &result); err != nil || len(result.Followed) != 0 {
			t.Fatalf("%v follows %+v: %v\n%s", command, result.Followed, err, output)
		}
		if named := slices.DeleteFunc(result.Selections, func(entry restoredSelection) bool { return entry.Client != hermes.ID }); !slices.Equal(named, want) {
			t.Fatalf("%v restores %+v for Hermes, want %+v\n%s", command, named, want, output)
		}
		if slices.Contains(command, "--dry-run") && !bytes.Equal(readFile(t, target+".aigw-state.json"), record) {
			t.Fatal("the rollback's preview wrote AIGW's record of Hermes's file")
		}
	}
	if !bytes.Equal(readFile(t, target), chosen) {
		t.Fatal("the rollback wrote the file of a client that waits for its Token")
	}
	if binding := selection(t, app, hermes.ID); binding.Model != domain.NewModelRef("dmxapi", "gpt-6.2-sol") {
		t.Fatalf("the rollback saved %s, not the selection that it names as restored", binding.Model)
	}
	if bytes.Equal(readFile(t, target+".aigw-state.json"), record) {
		t.Fatal("AIGW's record of Hermes's file stayed behind the model that the file shows")
	}
	// The person picks the model that the record named before the rollback.
	hermesNativeSelection(t, target, "aigw-dmxapi-responses", "gpt-6.2-sol", nil)
	if err := app.Secrets.Set("dmxapi", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	syncAndCheckPass(t, app, output, hermes.ID)
	if provider, model := hermesProjectedSelection(t, target); provider != "aigw-dmxapi-responses" || model != "gpt-6.2-sol" {
		t.Fatalf("synchronization wrote %s %s over the model chosen last", provider, model)
	}
}
