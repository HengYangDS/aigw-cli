package cli_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
)

// This file holds the regression tests for what the tenth review of version
// ten found for Codex (2026-11-22): a pick that the person writes into
// Codex's file as a TOML literal string, and the removal of the stored model
// beside a choice that cannot be recorded. Each test states its sequence with
// three values: the model that a file selects, the model that AIGW's record
// of the file names, and the selection that AIGW has stored.

// The person writes a pick into one Codex file as a TOML literal string and
// into the other as a basic string.
//
// Before: Codex has two files. Both select gpt-sol, both records name
// gpt-sol, and gateway/gpt-sol is stored. The provider's Token is absent. The
// person selects gpt-terra in both files: in the first as
// `model = 'gpt-terra'`, in the second as `model = "gpt-terra"`.
//
// A command commits: `aigw sync`, an edit of another provider, or an edit of
// Codex's own setting. After it gateway/gpt-terra is stored, both files still
// select gpt-terra, and both records name gpt-terra.
//
// Then the person selects gpt-sol again in the first file, the Token arrives
// and `aigw sync` runs. It follows that choice: gateway/gpt-sol is stored,
// and both files select gpt-sol with records that name it.
//
// The tenth review found that the commit left the first file's record at
// gpt-sol, because AIGW read the literal string as no model. The return to
// gpt-sol in that file was then no choice, and the synchronization wrote
// gpt-terra over it.
func TestReviewTenALiteralStringPickBesideABasicOneIsRecorded(t *testing.T) {
	for _, command := range [][]string{
		{"sync", "--json"},
		{"provider", "edit", "spare", "--label", "Other", "--json"},
		{"client", "edit", "codex", "--request-retries", "2", "--json"},
	} {
		t.Run(strings.Join(command, "_"), func(t *testing.T) {
			world := codexWorld(t, 2)
			world.setToken(t, false)
			world = world.picking(t, 0, literalString, "gpt-terra").picking(t, 1, basicString, "gpt-terra")
			recordedThenPicked(t, world, command, "gpt-terra", "gpt-sol")
		})
	}
}

// The person writes the stored model into Codex's file as a TOML literal
// string, beside a record that names an earlier model.
//
// Before: AIGW wrote gpt-terra into Codex's one file, so the file selects
// gpt-terra and the record names gpt-terra. The Token was lost, and a
// rollback stored gateway/gpt-sol, which waits for the Token. The person
// selects gpt-sol in the file, written as `model = 'gpt-sol'`: the file
// selects the stored model, and the record still names gpt-terra.
//
// A command commits without the Token: `aigw sync`, or an edit of another
// provider. After it gateway/gpt-sol is still stored, the file still selects
// gpt-sol, and the record names gpt-sol.
//
// Then the person selects gpt-terra again, the Token arrives and `aigw sync`
// runs. It follows that choice: gateway/gpt-terra is stored, and the file
// selects gpt-terra with a record that names it.
//
// The tenth review found that the commit left the record at gpt-terra,
// because AIGW read the literal string as no model. The later pick of
// gpt-terra was then no choice, and the synchronization wrote gpt-sol over
// it.
func TestReviewTenALiteralStringPickOfTheStoredModelIsRecorded(t *testing.T) {
	for _, command := range [][]string{
		{"sync", "--json"},
		{"provider", "edit", "spare", "--label", "Other", "--json"},
	} {
		t.Run(strings.Join(command, "_"), func(t *testing.T) {
			world := codexPending(t, 1).picking(t, 0, literalString, "gpt-sol")
			recordedThenPicked(t, world, command, "gpt-sol", "gpt-terra")
		})
	}
}

// recordedThenPicked is the sequence that the two tests above share. The
// command commits without the Token and must leave recorded stored, selected
// by every file and named by every record. Then the person selects picked in
// the first file, the Token arrives, and `aigw sync` must follow that choice:
// picked is stored, selected by every file and named by every record.
func recordedThenPicked(t *testing.T, world selectionWorld, command []string, recorded, picked string) {
	t.Helper()
	commit := runIn(t, world, command...)
	if commit.err != nil {
		t.Fatalf("%v: %v\n%s", command, commit.err, commit.output)
	}
	if !settledOn(commit.after, recorded) {
		t.Errorf("%v committed and did not leave %s stored with every record naming what its file selects:\n  before: %s\n  after:  %s",
			command, recorded, shownLine(commit.before), shownLine(commit.after))
	}
	recPick(t, world.files[0], picked)
	world.setToken(t, true)
	sync := runIn(t, world, "sync", "--json")
	if sync.err != nil {
		t.Fatalf("the person selected %s last; the synchronization with the Token: %v\n%s", picked, sync.err, sync.output)
	}
	if !settledOn(sync.after, picked) {
		t.Errorf("the person selected %s last; after the Token and a synchronization:\n  before: %s\n  after:  %s",
			picked, shownLine(sync.before), shownLine(sync.after))
	}
}

// The person writes a pick into Codex's file as a TOML literal string, with
// the Token and without it.
//
// Before: Codex's one file selects gpt-sol, the record names gpt-sol, and
// gateway/gpt-sol is stored. The person selects gpt-terra, written as
// `model = 'gpt-terra'`: the file selects gpt-terra, and the record and the
// stored selection stay.
//
// `aigw status --json` names gateway/gpt-terra as Codex's model and as a
// followed selection, reports no drift, and changes no file. `aigw sync` then
// succeeds: gateway/gpt-terra is stored, the file selects gpt-terra, and the
// record names gpt-terra.
//
// The tenth review found that the choice was not followed: AIGW read the
// literal string as no model, status reported drift with `aigw sync` as the
// next step, and `aigw sync` refused the file as edited outside AIGW.
func TestReviewTenAPickWrittenAsALiteralStringIsFollowed(t *testing.T) {
	for _, token := range []bool{true, false} {
		t.Run(fmt.Sprintf("token=%t", token), func(t *testing.T) {
			world := codexWorld(t, 1)
			world.setToken(t, token)
			world = world.picking(t, 0, literalString, "gpt-terra")

			status := runIn(t, world, "status", "--json")
			if status.err != nil {
				t.Fatalf("status after the pick: %v\n%s", status.err, status.output)
			}
			if len(status.changed) != 0 {
				t.Errorf("status changed files: %v", status.changed)
			}
			var report struct {
				Followed []followed `json:"followed_selections"`
				Clients  map[string]struct {
					State  string `json:"state"`
					Model  string `json:"model"`
					Detail string `json:"detail"`
				} `json:"clients"`
			}
			status.decode(t, &report)
			if entry := report.Clients[codex.ID]; entry.Model != "gateway/gpt-terra" || entry.State == "invalid" || strings.Contains(entry.Detail, "drift") {
				t.Errorf("status does not name the pick as Codex's model, or reports it as a fault: %+v", entry)
			}
			if !slices.Equal(report.Followed, codexChoice) {
				t.Errorf("status follows %+v; want the pick, %+v", report.Followed, codexChoice)
			}

			sync := runIn(t, world, "sync")
			if sync.err != nil {
				t.Fatalf("`aigw sync` after a pick written as a literal string: %v\n%s", sync.err, sync.output)
			}
			if !settledOn(sync.after, "gpt-terra") {
				t.Errorf("the synchronization did not follow the pick:\n  before: %s\n  after:  %s", shownLine(sync.before), shownLine(sync.after))
			}
		})
	}
}

// The person removes the stored model while a choice in Codex cannot be
// recorded, with the Token and without it.
//
// Before: Codex has two files, and gateway/gpt-sol is stored. The person
// selected gpt-terra in the first file, whose record still names gpt-sol. The
// second file is missing, or AIGW's record of it is missing, so no command
// can record the choice.
//
// `aigw model remove gateway/gpt-sol --json` is refused and changes no file:
// gateway/gpt-sol is still stored, the first file still selects gpt-terra,
// and its record still names gpt-sol. The refusal names the file that keeps
// the choice from being recorded, and a step.
//
// The tenth review found that the refusal was the validator's own text,
// "refuse invalid config", with `aigw check` as its step: it named neither
// the file nor what to do about it. The wording of the refusal is not held
// here, only that it names both.
func TestReviewTenRemovingTheStoredModelBesideAChoiceThatCannotBeRecorded(t *testing.T) {
	for _, state := range []string{"chosen, the other file missing", "chosen, the other record missing"} {
		for _, token := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/token=%t", state, token), func(t *testing.T) {
				world := codexTwoFilesMatrix().states[state](t)
				world.setToken(t, token)
				removal := runIn(t, world, "model", "remove", "gateway/gpt-sol", "--json")
				if removal.err == nil {
					t.Fatalf("the removal of the stored selection's model succeeded:\n  before: %s\n  after:  %s", shownLine(removal.before), shownLine(removal.after))
				}
				if len(removal.changed) != 0 {
					t.Errorf("the refused removal changed files: %v", removal.changed)
				}
				const validator = "refuse invalid config"
				if strings.Contains(removal.err.Error(), validator) || strings.Contains(removal.output, validator) {
					t.Errorf("the refusal is the validator's own text: %v\n%s", removal.err, removal.output)
				}
				var document struct {
					Error      string `json:"error"`
					NextAction string `json:"next_action"`
					NextDetail string `json:"next_detail"`
				}
				removal.decode(t, &document)
				if document.NextAction == "" && document.NextDetail == "" {
					t.Errorf("the refusal names no step:\n%s", removal.output)
				}
				if !strings.Contains(document.Error, "config.toml") {
					t.Errorf("the refusal does not name the file that keeps the choice from being recorded: %q", document.Error)
				}
			})
		}
	}
}
