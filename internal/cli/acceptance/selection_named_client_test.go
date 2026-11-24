package cli_test

import (
	"os"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
)

// Ninth review, probe 1. The table's second row: a client whose file is
// refused keeps its choice in the file alone, with the stored selection and
// the record as they were, "with or without a Token". The regression
// TestRecordCodexOtherFileMissing holds it for a command that does not name
// Codex. These commands name Codex (sync names every client; client edit
// names it): what do they save while another file of Codex is missing?
func TestSelectionANamedCommandWithAnotherCodexFileMissing(t *testing.T) {
	t.Parallel()
	for _, command := range [][]string{
		{"sync", "--json"},
		{"client", "edit", "codex", "--request-retries", "2", "--json"},
	} {
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
			err := recRun(t, app, output, command...)
			stored, recorded := selection(t, app, codex.ID).Model.String(), recCodexRecorded(t, targets[0])
			t.Logf("%v token=%t: err=%v S=%s F(A)=[%s] R(A)=%s", command, token, err, stored, recCodexModel(t, targets[0]), recorded)
			if stored != "gateway/gpt-sol" && recorded == "gpt-sol" {
				t.Errorf("%v token=%t (err=%v): the stored selection moved to %s while AIGW's record of the file stayed at %s; S and R move to F only together", command, token, err, stored, recorded)
			}
		}
	}
}

// Ninth review, probe 1, the consequence: after that synchronization the
// person returns to the earlier model in the same file. The record still names
// that model, so nothing sees a choice, and the synchronization after the
// Token writes the saved model over it.
func TestSelectionAPickAfterANamedCommandWithAFileMissingIsWrittenOver(t *testing.T) {
	t.Parallel()
	app, output, targets := recCodexFiles(t, 2)
	if err := app.Secrets.Delete("gateway"); err != nil {
		t.Fatal(err)
	}
	recPick(t, targets[0], "gpt-terra")
	sibling := readFile(t, targets[1])
	if err := os.Remove(targets[1]); err != nil {
		t.Fatal(err)
	}
	if err := recRun(t, app, output, "sync", "--json"); err != nil {
		t.Logf("sync with the other file missing: %v", err)
	}
	recCodexState(t, app, targets[:1], "after sync with file B missing")
	// File B comes back as AIGW wrote it; the person picks gpt-sol again in A.
	writeFile(t, targets[1], sibling, 0o600)
	recPick(t, targets[0], "gpt-sol")
	if got := followedIn(t, app, output, "status", "--json"); len(got) != 0 {
		t.Logf("status follows %+v", got)
	}
	if err := app.Secrets.Set("gateway", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	if err := recRun(t, app, output, "sync", "--json"); err != nil {
		t.Fatalf("sync after the Token: %v", err)
	}
	recCodexState(t, app, targets, "after the pick of sol, the Token and sync")
	if !strings.Contains(recCodexModel(t, targets[0]), "gpt-sol") {
		t.Errorf("synchronization wrote %s over the model chosen last in file A (gpt-sol); S=%s", recCodexModel(t, targets[0]), selection(t, app, codex.ID).Model)
	}
}

// Ninth review, probe 1, the other states of a sibling file that a projection
// refuses (its record gone, or unreadable), under a command that names Codex.
func TestSelectionANamedCommandWithAnotherCodexRecordDamaged(t *testing.T) {
	t.Parallel()
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
			damage(t, targets[1])
			err := selRun(t, app, output, "sync", "--json")
			stored, recorded := selection(t, app, codex.ID).Model.String(), recCodexRecorded(t, targets[0])
			t.Logf("%s token=%t: sync err=%v S=%s F(A)=[%s] R(A)=%s", name, token, err, stored, recCodexModel(t, targets[0]), recorded)
			if stored != "gateway/gpt-sol" && recorded == "gpt-sol" {
				t.Errorf("%s token=%t (err=%v): the stored selection moved to %s while AIGW's record of the file stayed at %s", name, token, err, stored, recorded)
			}
		}
	}
}

// Ninth review, probe 1 and 8 together: a rollback while another file of
// Codex is missing. To a third model, it names Codex: what becomes of the
// record of the file that shows the choice? To the chosen model itself, it
// does not change Codex's binding against the working configuration: does it
// save the selection of the configuration that it restores?
func TestSelectionARollbackWithAnotherCodexFileMissing(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name     string
		prepare  [][]string
		restores string
	}{
		{"to a third model", [][]string{{"use", "codex", "gateway/gpt-luna"}, {"use", "codex", "gateway/gpt-sol"}}, "gateway/gpt-luna"},
		{"to the chosen model", [][]string{{"use", "codex", "gateway/gpt-terra"}, {"use", "codex", "gateway/gpt-sol"}}, "gateway/gpt-terra"},
	} {
		for _, token := range []bool{true, false} {
			app, output, targets := recCodexFiles(t, 2)
			for _, args := range scenario.prepare {
				if err := selRun(t, app, output, args...); err != nil {
					t.Fatalf("%v: %v\n%s", args, err, output)
				}
			}
			if !token {
				if err := app.Secrets.Delete("gateway"); err != nil {
					t.Fatal(err)
				}
			}
			recPick(t, targets[0], "gpt-terra")
			if err := os.Remove(targets[1]); err != nil {
				t.Fatal(err)
			}
			err := selRun(t, app, output, "config", "rollback", "--last-change", "--json")
			stored, recorded := selection(t, app, codex.ID).Model.String(), recCodexRecorded(t, targets[0])
			t.Logf("%s token=%t: err=%v S=%s F(A)=[%s] R(A)=%s\n%.600s", scenario.name, token, err, stored, recCodexModel(t, targets[0]), recorded, output)
			switch {
			case err != nil && stored == "gateway/gpt-sol" && recorded == "gpt-sol":
				// Refused with nothing changed.
			case err == nil && stored != scenario.restores:
				t.Errorf("%s token=%t: the rollback succeeded and saved %s for Codex; the configuration that it restores selects %s", scenario.name, token, stored, scenario.restores)
			case err == nil && !strings.Contains(recCodexModel(t, targets[0]), `"`+recorded+`"`):
				t.Errorf("%s token=%t: the rollback saved %s and left the record of file A at %s while the file shows %s", scenario.name, token, stored, recorded, recCodexModel(t, targets[0]))
			}
		}
	}
}
