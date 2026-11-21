package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/cli"
)

// pendingHermes leaves Hermes with a stored selection that waits for its
// Token, with no choice in play: AIGW wrote claude-opus-5-5, the Token of
// dmxapi is lost, and a rollback restores gpt-6.2-sol, which cannot be
// written. Hermes's file and AIGW's record of it both show claude-opus-5-5.
func pendingHermes(t *testing.T) (*cli.App, *bytes.Buffer, string) {
	t.Helper()
	app, output, _, target := shippedHermesAppWith(t, true)
	if err := cli.Execute(app, []string{"use", "hermes", "dmxapi/claude-opus-5-5", "--protocol", "anthropic"}); err != nil {
		t.Fatalf("select: %v\n%s", err, output)
	}
	if err := app.Secrets.Delete("dmxapi"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"config", "rollback", "--last-change", "--json"}); err != nil {
		t.Fatalf("rollback: %v\n%s", err, output)
	}
	if got := selection(t, app, hermes.ID).Model.String(); got != "dmxapi/gpt-6.2-sol" {
		t.Fatalf("the rollback saved %s", got)
	}
	if provider, model := hermesProjectedSelection(t, target); provider != "aigw-dmxapi-anthropic" || model != "claude-opus-5-5" {
		t.Fatalf("the rollback wrote Hermes's file without a Token: %s %s", provider, model)
	}
	return app, output, target
}

// F1, Hermes, with no earlier choice in play: the person picks in Hermes the
// model that AIGW names as selected, a command commits, the person picks the
// earlier model again, the Token arrives.
func TestRecordAPickOfTheStoredSelectionIsRecordedHermes(t *testing.T) {
	app, output, target := pendingHermes(t)
	hermesNativeSelection(t, target, "aigw-dmxapi-responses", "gpt-6.2-sol", nil)
	record := readFile(t, target+".aigw-state.json")
	_ = recRun(t, app, output, "status")
	if err := recRun(t, app, output, "sync", "--json"); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(readFile(t, target+".aigw-state.json"), record) {
		t.Errorf("a committed transaction left AIGW's record behind the model that Hermes's file shows")
	}
	hermesNativeSelection(t, target, "aigw-dmxapi-anthropic", "claude-opus-5-5", nil)
	if err := app.Secrets.Set("dmxapi", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	_ = recRun(t, app, output, "status")
	if err := recRun(t, app, output, "sync", "--json"); err != nil {
		t.Fatal(err)
	}
	if provider, model := hermesProjectedSelection(t, target); provider != "aigw-dmxapi-anthropic" || model != "claude-opus-5-5" {
		t.Errorf("synchronization wrote %s %s over the model chosen last", provider, model)
	}
}

// F1, Codex, one file, with no earlier choice in play.
func TestRecordAPickOfTheStoredSelectionIsRecordedCodex(t *testing.T) {
	app, output, targets := recCodexFiles(t, 1)
	if err := cli.Execute(app, []string{"use", "codex", "gateway/gpt-terra"}); err != nil {
		t.Fatalf("select: %v\n%s", err, output)
	}
	if err := app.Secrets.Delete("gateway"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"config", "rollback", "--last-change", "--json"}); err != nil {
		t.Fatalf("rollback: %v\n%s", err, output)
	}
	recCodexState(t, app, targets, "pending")
	recPick(t, targets[0], "gpt-sol")
	if err := recRun(t, app, output, "sync", "--json"); err != nil {
		t.Fatal(err)
	}
	recCodexState(t, app, targets, "after the pick of the stored model and sync")
	if recorded := recCodexRecorded(t, targets[0]); recorded != "gpt-sol" {
		t.Errorf("a committed transaction left AIGW's record at %s while Codex's file shows gpt-sol", recorded)
	}
	recPick(t, targets[0], "gpt-terra")
	if err := app.Secrets.Set("gateway", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	_ = recRun(t, app, output, "status")
	if err := recRun(t, app, output, "sync", "--json"); err != nil {
		t.Fatal(err)
	}
	recCodexState(t, app, targets, "after the pick of terra, the Token and sync")
	if !strings.Contains(recCodexModel(t, targets[0]), "gpt-terra") {
		t.Errorf("synchronization wrote %s over the model chosen last (gpt-terra)", recCodexModel(t, targets[0]))
	}
}

// Which commands bring the record up when the file shows the stored
// selection and the record another model.
func TestRecordWhatBringsTheRecordUp(t *testing.T) {
	app, output, target := pendingHermes(t)
	hermesNativeSelection(t, target, "aigw-dmxapi-responses", "gpt-6.2-sol", nil)
	record := readFile(t, target+".aigw-state.json")
	moved := func() bool { return !bytes.Equal(readFile(t, target+".aigw-state.json"), record) }
	if err := recRun(t, app, output, "client", "edit", "hermes", "--via", "http://127.0.0.1:8792", "--json"); err != nil {
		t.Fatal(err)
	}
	if !moved() {
		t.Error("a committed transaction without the Token left the record behind the selection that Hermes's file shows")
	}
	if err := recRun(t, app, output, "use", "claude", "ucloud/claude-opus-5-5", "--json"); err != nil {
		t.Fatal(err)
	}
	t.Logf("no Token, use claude: record moved = %t", moved())
	if err := app.Secrets.Set("dmxapi", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	if err := recRun(t, app, output, "provider", "edit", "ucloud", "--label", "Other", "--json"); err != nil {
		t.Fatal(err)
	}
	t.Logf("Token, provider edit ucloud --label: record moved = %t", moved())
	_ = recRun(t, app, output, "check", "hermes", "--depth", "local")
	if err := recRun(t, app, output, "sync", "--json"); err != nil {
		t.Fatal(err)
	}
	t.Logf("Token, sync: record moved = %t", moved())
}

// Angle 2: a record-only write that fails, and a configuration that cannot
// be saved, leave the records and the stored selection as they were.
func TestRecordAFailedRecordOrSaveLeavesNothing(t *testing.T) {
	for _, locked := range []int{0, 1} {
		app, output, targets := recCodexFiles(t, 2)
		if err := app.Secrets.Delete("gateway"); err != nil {
			t.Fatal(err)
		}
		recPick(t, targets[0], "gpt-terra")
		recPick(t, targets[1], "gpt-terra")
		folder := filepath.Dir(targets[locked])
		if err := os.Chmod(folder, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(folder, 0o700) })
		stored := readFile(t, app.Config.Path())
		err := recRun(t, app, output, "provider", "edit", "spare", "--label", "Other", "--json")
		_ = os.Chmod(folder, 0o700)
		recCodexState(t, app, targets, "after the failed record")
		if err == nil {
			t.Errorf("locked=%d: the command succeeded although one record cannot be written", locked)
			continue
		}
		if recCodexRecorded(t, targets[0]) != "gpt-sol" || recCodexRecorded(t, targets[1]) != "gpt-sol" || !bytes.Equal(readFile(t, app.Config.Path()), stored) {
			t.Errorf("locked=%d: a failed record-only write left a record or the stored configuration changed", locked)
		}
		if got := followedIn(t, app, output, "status", "--json"); len(got) != 1 {
			t.Errorf("locked=%d: after the failure status follows %+v", locked, got)
		}
	}
	// The configuration cannot be saved.
	app, output, targets := recCodexFiles(t, 1)
	if err := app.Secrets.Delete("gateway"); err != nil {
		t.Fatal(err)
	}
	recPick(t, targets[0], "gpt-terra")
	folder := filepath.Dir(app.Config.Path())
	if err := os.Chmod(folder, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(folder, 0o700) })
	err := recRun(t, app, output, "provider", "edit", "spare", "--label", "Other", "--json")
	_ = os.Chmod(folder, 0o700)
	recCodexState(t, app, targets, "after the failed save")
	if err == nil || recCodexRecorded(t, targets[0]) != "gpt-sol" || selection(t, app, codex.ID).Model.String() != "gateway/gpt-sol" {
		t.Errorf("a failed save (err=%v) left the record at %s", err, recCodexRecorded(t, targets[0]))
	}
}

// ZZ3's question: is "drift" after the Token arrives particular to a choice,
// or the state of any Codex selection that waited for its Token?
func TestRecordCodexAfterTheTokenArrives(t *testing.T) {
	// A choice recorded without the Token, one file.
	app, output, targets := recCodexFiles(t, 1)
	if err := app.Secrets.Delete("gateway"); err != nil {
		t.Fatal(err)
	}
	recPick(t, targets[0], "gpt-terra")
	if err := recRun(t, app, output, "sync", "--json"); err != nil {
		t.Fatal(err)
	}
	if err := app.Secrets.Set("gateway", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	recCodexState(t, app, targets, "choice recorded, Token arrived")
	_ = recRun(t, app, output, "status")
	if err := recRun(t, app, output, "check", "codex", "--depth", "local"); err == nil {
		t.Error("a recorded choice whose file is not written yet passed the local check")
	}
	if err := recRun(t, app, output, "sync", "--json"); err != nil {
		t.Errorf("synchronization after the Token: %v", err)
	}
	if err := recRun(t, app, output, "check", "codex", "--depth", "local"); err != nil {
		t.Errorf("the local check after synchronization: %v", err)
	}
	// With the Token all along, the same file state passes the check.
	app, output, targets = recCodexFiles(t, 1)
	recPick(t, targets[0], "gpt-terra")
	if err := recRun(t, app, output, "check", "codex", "--depth", "local"); err != nil {
		t.Errorf("with the Token, the local check of a choice that no command recorded yet: %v", err)
	}
	// No choice: a selection that waited for its Token.
	app, output, targets = recCodexFiles(t, 1)
	if err := cli.Execute(app, []string{"use", "codex", "gateway/gpt-terra"}); err != nil {
		t.Fatal(err)
	}
	if err := app.Secrets.Delete("gateway"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"config", "rollback", "--last-change", "--json"}); err != nil {
		t.Fatal(err)
	}
	if err := app.Secrets.Set("gateway", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	recCodexState(t, app, targets, "no choice, pending selection, Token arrived")
	_ = recRun(t, app, output, "status")
	if err := recRun(t, app, output, "check", "codex", "--depth", "local"); err == nil {
		t.Error("a selection that waited for its Token passed the local check before its files were written")
	}
}

// ZZ11's question, other states of the sibling file that a projection
// refuses: its record is unreadable, or gone.
func TestRecordCodexSiblingThatAProjectionRefuses(t *testing.T) {
	for name, damage := range map[string]func(t *testing.T, target string){
		"record unreadable": func(t *testing.T, target string) { writeFile(t, target+".aigw-state.json", []byte("{"), 0o600) },
		"record gone": func(t *testing.T, target string) {
			if err := os.Remove(target + ".aigw-state.json"); err != nil {
				t.Fatal(err)
			}
		},
	} {
		for _, token := range []bool{true, false} {
			app, output, targets := recCodexFiles(t, 2)
			if !token {
				if err := app.Secrets.Delete("gateway"); err != nil {
					t.Fatal(err)
				}
			}
			recPick(t, targets[0], "gpt-terra")
			if _, err := os.Stat(targets[1] + ".aigw-state.json"); err != nil {
				t.Fatalf("the record of the second file is not beside it: %v", err)
			}
			damage(t, targets[1])
			err := recRun(t, app, output, "provider", "edit", "spare", "--label", "Other", "--json")
			if got, recorded := selection(t, app, codex.ID).Model.String(), recCodexRecorded(t, targets[0]); err != nil || got != "gateway/gpt-sol" || recorded != "gpt-sol" {
				t.Errorf("%s, token=%t: err=%v, saved %s with the record at %s; the choice stays in its file alone while another file of Codex is refused", name, token, err, got, recorded)
			}
		}
	}
}
