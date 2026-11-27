package cli_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/terminal"
)

// Ninth review, probe 6b. The Change's scenario "An explicit selection
// returns to the stored model": the stored model serves two protocols, the
// operator picked a model on the other protocol in Hermes, and returns with
// `aigw use` without --protocol. An explicit selection starts from the stored
// binding (fromStoredBinding); the protocol of a selection that names none is
// resolved before that, from the working configuration, which follows the
// choice. The control is the same command without a choice in between.
func TestSelectionAnExplicitReturnWithoutAProtocolAfterAChoiceOnAnotherProtocol(t *testing.T) {
	t.Parallel()
	for _, choice := range []bool{false, true} {
		app, output, _, target := shippedHermesAppWith(t, true)
		for _, args := range [][]string{
			{"use", "hermes", "ucloud/gpt-6.2-sol", "--protocol", "chat", "--unlisted"},
			{"use", "hermes", "ucloud/gpt-6.2-sol", "--protocol", "responses"},
			{"client", "edit", "hermes", "--via", "http://127.0.0.1:9000"},
		} {
			if err := selRun(t, app, output, args...); err != nil {
				t.Fatalf("%v: %v\n%s", args, err, output)
			}
		}
		if choice {
			hermesNativeSelection(t, target, "aigw-ucloud-chat", "gemini-4-argon", nil)
			if got := followedIn(t, app, output, "status", "--json"); len(got) != 1 {
				t.Fatalf("the fixture's choice is not followed: %+v", got)
			}
		}
		before := selection(t, app, hermes.ID)
		document, err := selDocument(t, app, output, "use", "hermes", "ucloud/gpt-6.2-sol", "--json")
		if err != nil {
			t.Fatalf("choice=%t: the return: %v\n%s", choice, err, output)
		}
		after := selection(t, app, hermes.ID)
		t.Logf("choice=%t: stored %s on %s via %q; after the return %s on %s via %q; document protocol=%v forwarding_cleared=%v", choice,
			before.Model, before.Protocol.Name(), before.ForwardingRoot, after.Model, after.Protocol.Name(), after.ForwardingRoot, document["protocol"], document["forwarding_cleared"])
		if after.Protocol != before.Protocol || after.ForwardingRoot != before.ForwardingRoot {
			t.Errorf("choice=%t: `aigw use hermes ucloud/gpt-6.2-sol` returned to the stored model on %s via %q; the stored selection was on %s via %q (forwarding_cleared=%v)", choice,
				after.Protocol.Name(), after.ForwardingRoot, before.Protocol.Name(), before.ForwardingRoot, document["forwarding_cleared"])
		}
	}
}

// Ninth review, probe 1c: a command that names Hermes while its file shows a
// choice whose provider has no Token: the stored selection and the record
// move to the file together, and the file is left. The choice is an
// Anthropic model, which a forwarding cannot carry, so an edit that forwards
// Hermes is refused and changes nothing, naming the direct connection.
func TestSelectionANamedCommandRecordsAHermesChoiceWithoutAToken(t *testing.T) {
	t.Parallel()
	for _, command := range [][]string{
		{"sync", "--json"},
		{"client", "edit", "hermes", "--direct", "--json"},
	} {
		app, output, target, chosen := waitingHermes(t)
		err := selRun(t, app, output, command...)
		stored, behind := selection(t, app, hermes.ID), selHermesBehind(t, target)
		t.Logf("%v: err=%v S=%s via %q behind=%q fileKept=%t", command, err, stored.Model, stored.ForwardingRoot, behind, bytes.Equal(readFile(t, target), chosen))
		if err != nil || stored.Model.String() != "ucloud/claude-opus-5-5" || behind != "" || !bytes.Equal(readFile(t, target), chosen) {
			t.Errorf("%v: err=%v S=%s, %s, file kept=%t", command, err, stored.Model, behind, bytes.Equal(readFile(t, target), chosen))
		}
	}
	app, output, target, _ := waitingHermes(t)
	before := ownedFileSnapshot(t, filepath.Dir(target), app.Config.Path())
	err := selRun(t, app, output, "client", "edit", "hermes", "--via", "http://127.0.0.1:8792", "--json")
	if problem := terminal.ProblemFor(err); err == nil || problem.Fix != "aigw client edit hermes --direct" {
		t.Errorf("forwarding an Anthropic choice: err=%v, fix %q", err, problem.Fix)
	}
	if after := ownedFileSnapshot(t, filepath.Dir(target), app.Config.Path()); !equalSnapshots(after, before) {
		t.Error("the refused forwarding changed Hermes's file, its record or the stored configuration")
	}
}

// Ninth review, a question for the specification: every committing command
// now writes the record of every enabled client whose record is behind. While
// Hermes's folder cannot be written, which commands about something else
// still work? (Logged, and asserted only that a failed command changes
// nothing.)
func TestSelectionAnUnwritableRecordAndCommandsAboutSomethingElse(t *testing.T) {
	t.Parallel()
	for _, command := range [][]string{
		{"client", "edit", "codex", "--request-retries", "2", "--json"},
		{"use", "claude", "ucloud/claude-opus-5-5", "--json"},
		{"provider", "edit", "ucloud", "--label", "Other", "--json"},
		{"client", "disable", "hermes", "--json"},
	} {
		app, output, target := pendingHermes(t)
		hermesNativeSelection(t, target, "aigw-dmxapi-responses", "gpt-6.2-sol", nil)
		folder := filepath.Dir(target)
		before := ownedFileSnapshot(t, folder, app.Config.Path())
		release := blockFolder(t, folder)
		err := selRun(t, app, output, command...)
		release()
		t.Logf("%v with Hermes's folder read-only: err=%v", command, err)
		if after := ownedFileSnapshot(t, folder, app.Config.Path()); err != nil && !equalSnapshots(after, before) {
			t.Errorf("%v failed (%v) and left the record, the file or the stored configuration changed", command, err)
		}
	}
}
