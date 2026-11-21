package cli_test

import (
	"bytes"
	"os"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/cli"
)

// Spec scenario "Another file of the client cannot be written", with and
// without the Token: the choice stays in the file alone.
func TestRecordCodexOtherFileMissing(t *testing.T) {
	for _, token := range []bool{true, false} {
		app, output, targets := recCodexFiles(t, 2)
		if !token {
			if err := app.Secrets.Delete("gateway"); err != nil {
				t.Fatal(err)
			}
		}
		recPick(t, targets[0], "gpt-terra")
		if err := os.Remove(targets[1]); err != nil {
			t.Fatal(err)
		}
		err := recRun(t, app, output, "provider", "edit", "spare", "--label", "Other", "--json")
		if recorded := recCodexRecorded(t, targets[0]); err != nil || recorded != "gpt-sol" {
			t.Errorf("token=%t: err=%v with the record at %s; the command succeeds and the record stays behind the choice", token, err, recorded)
		}
		if got := selection(t, app, codex.ID).Model.String(); got != "gateway/gpt-sol" {
			t.Errorf("token=%t: with Codex's other file missing, a command about something else saved %s; the spec leaves the choice in the file alone", token, got)
		}
		_ = recRun(t, app, output, "status")
	}
}

// A rollback to a configuration that does not hold the chosen model, with
// and without the Token of the restored selection's provider.
func TestRecordRollbackToAConfigurationWithoutTheChosenModel(t *testing.T) {
	for _, token := range []bool{true, false} {
		app, output, _, target := shippedHermesAppWith(t, true)
		for _, args := range [][]string{
			{"use", "hermes", "dmxapi/zz-new", "--protocol", "responses", "--unlisted"},
		} {
			if err := cli.Execute(app, args); err != nil {
				t.Fatalf("%v: %v\n%s", args, err, output)
			}
		}
		// The backup holds dmxapi/gpt-6.2-sol and no zz-new. Hermes picks
		// zz-new again after another model was written, so that it is a choice.
		if err := cli.Execute(app, []string{"use", "hermes", "dmxapi/gpt-6.2-sol", "--protocol", "responses"}); err != nil {
			t.Fatalf("%v\n%s", err, output)
		}
		if err := cli.Execute(app, []string{"use", "hermes", "dmxapi/claude-opus-5-5", "--protocol", "anthropic"}); err != nil {
			t.Fatalf("%v\n%s", err, output)
		}
		// Now the backup is the configuration that selects gpt-6.2-sol and
		// holds zz-new. Remove zz-new from what the backup will be: not
		// possible by a command, so the choice is of zz-new and the rollback
		// goes to the backup, which selects gpt-6.2-sol.
		if !token {
			if err := app.Secrets.Delete("dmxapi"); err != nil {
				t.Fatal(err)
			}
		}
		chosen := hermesNativeSelection(t, target, "aigw-dmxapi-responses", "zz-new", nil)
		record := readFile(t, target+".aigw-state.json")
		_ = recRun(t, app, output, "config", "rollback", "--last-change", "--dry-run", "--json")
		if !bytes.Equal(readFile(t, target+".aigw-state.json"), record) || !bytes.Equal(readFile(t, target), chosen) {
			t.Errorf("token=%t: the rollback preview wrote", token)
		}
		err := recRun(t, app, output, "config", "rollback", "--last-change", "--json")
		provider, model := hermesProjectedSelection(t, target)
		t.Logf("token=%t: err=%v S=%s file=%s %s recordMoved=%t", token, err, selection(t, app, hermes.ID).Model, provider, model, !bytes.Equal(readFile(t, target+".aigw-state.json"), record))
		_ = recRun(t, app, output, "status")
		if !token {
			if err := app.Secrets.Set("dmxapi", "fixture-token"); err != nil {
				t.Fatal(err)
			}
			_ = recRun(t, app, output, "sync", "--json")
			provider, model = hermesProjectedSelection(t, target)
			t.Logf("after the Token and sync: S=%s file=%s %s", selection(t, app, hermes.ID).Model, provider, model)
			if err := recRun(t, app, output, "check", "hermes", "--depth", "local", "--json"); err != nil {
				t.Errorf("check after the Token: %v", err)
			}
		}
	}
}

// Codex, two files, no Token: a rollback to a third model while one file
// shows a choice; then the Token.
func TestRecordCodexRollbackToAThirdModelWithoutAToken(t *testing.T) {
	app, output, targets := recCodexFiles(t, 2)
	for _, args := range [][]string{{"use", "codex", "gateway/gpt-luna"}, {"use", "codex", "gateway/gpt-sol"}} {
		if err := cli.Execute(app, args); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, output)
		}
	}
	if err := app.Secrets.Delete("gateway"); err != nil {
		t.Fatal(err)
	}
	recPick(t, targets[0], "gpt-terra")
	_ = recRun(t, app, output, "config", "rollback", "--last-change", "--dry-run")
	recCodexState(t, app, targets, "after the rollback preview")
	if err := recRun(t, app, output, "config", "rollback", "--last-change", "--json"); err != nil {
		t.Errorf("rollback: %v", err)
	}
	recCodexState(t, app, targets, "after the rollback")
	_ = recRun(t, app, output, "status")
	// The person picks the model that the record of file A named before.
	recPick(t, targets[0], "gpt-sol")
	_ = recRun(t, app, output, "status")
	if err := app.Secrets.Set("gateway", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	if err := recRun(t, app, output, "sync", "--json"); err != nil {
		t.Errorf("sync after the Token: %v", err)
	}
	recCodexState(t, app, targets, "after the pick of sol, the Token and sync")
	if err := recRun(t, app, output, "check", "codex", "--depth", "local", "--json"); err != nil {
		t.Errorf("check after the Token: %v", err)
	}
}
