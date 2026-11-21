package cli_test

import (
	"testing"

	"aigw-cli/internal/adapter/codex"
)

// A second symptom of the record that stays behind (F1), Codex with two
// files and no Token: file A's choice is recorded by the record-only write,
// the person sets the same model in file B, then picks a third model in A.
func TestRecordCodexAStaleRecordMakesTheNextPickUnfollowable(t *testing.T) {
	app, output, targets := recCodexFiles(t, 2)
	if err := app.Secrets.Delete("gateway"); err != nil {
		t.Fatal(err)
	}
	recPick(t, targets[0], "gpt-terra")
	if err := recRun(t, app, output, "sync", "--json"); err != nil {
		t.Fatal(err)
	}
	recPick(t, targets[1], "gpt-terra")
	if err := recRun(t, app, output, "sync", "--json"); err != nil {
		t.Fatal(err)
	}
	recCodexState(t, app, targets, "both files show the stored selection")
	recPick(t, targets[0], "gpt-luna")
	_ = recRun(t, app, output, "status")
	if got := followedIn(t, app, output, "status", "--json"); len(got) != 1 || got[0].Model != "gateway/gpt-luna" {
		t.Errorf("status follows %+v; file A selects gpt-luna and file B the stored gpt-terra, each a model that the person chose", got)
	}
	if err := app.Secrets.Set("gateway", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	_ = recRun(t, app, output, "status")
	if err := recRun(t, app, output, "sync", "--json"); err != nil {
		t.Errorf("after the Token, synchronization refuses the person's own picks: %v", err)
	}
	recCodexState(t, app, targets, "after the Token and sync")
	t.Logf("S=%s", selection(t, app, codex.ID).Model)
}
