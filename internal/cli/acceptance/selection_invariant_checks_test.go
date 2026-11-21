package cli_test

import (
	"bytes"
	"slices"
	"testing"
)

// holds checks the one sentence that every command must leave true for a
// client that keeps a record, and returns the client as the command left it.
//
// After a committed command, the record of each of the client's files names
// the model that the file selects, or the client's selection, files and
// records are exactly as before. A preview and a refused command change
// nothing. A model that a file chose is the stored selection afterwards,
// unless the command selected for the client itself or left the client
// untouched. A rollback that succeeds leaves the restored selection stored.
//
// "Exactly as before" is accepted only in a state whose own construction
// prevents the record (selectionWorld.blocked), and from a command that asks
// for what is stored already (selectionCommand.idle), which has nothing to
// save and must change nothing at all. In every other case a command that
// succeeds must leave every record naming what its file selects. A withdrawal
// is checked for what it removes (withdrew).
func (w selectionWorld) holds(t *testing.T, before selectionSnapshot, command selectionCommand, err error) selectionSnapshot {
	t.Helper()
	after := w.snapshot(t)
	switch {
	case command.preview || err != nil:
		if !bytes.Equal(after.config, before.config) || !after.clientUntouched(before) {
			t.Errorf("%s (err=%v) changed the configuration, a file or a record: stored %q -> %q, files show %q -> %q",
				command.name, err, before.stored, after.stored, before.shows, after.shows)
		}
	case command.withdraws:
		w.withdrew(t, before, after, command)
	default:
		w.committed(t, before, after, command)
	}
	return after
}

// committed checks a command that succeeded and is neither a preview nor a
// withdrawal.
func (w selectionWorld) committed(t *testing.T, before, after selectionSnapshot, command selectionCommand) {
	t.Helper()
	if command.restores && w.restored != "" && after.stored != w.restored {
		t.Errorf("%s succeeded and left %q stored, not the restored %q", command.name, after.stored, w.restored)
	}
	if command.leaves != nil {
		if missing := command.leaves(after); missing != "" {
			t.Errorf("%s succeeded and %s", command.name, missing)
		}
	}
	if command.idle != nil && command.idle(before) {
		if !bytes.Equal(after.config, before.config) || !after.clientUntouched(before) {
			t.Errorf("%s asked for what is stored already and changed the configuration, a file or a record: stored %q -> %q, files show %q -> %q",
				command.name, before.stored, after.stored, before.shows, after.shows)
		}
		return
	}
	if w.blocked != "" && after.clientUntouched(before) {
		// Something in the state prevents the record, and the command left
		// the choice in its file alone.
		return
	}
	for index := range w.files {
		if after.files[index] == nil {
			continue
		}
		if !after.follows[index] {
			t.Errorf("%s left the record of file %d behind: the file shows %q, stored %q -> %q, and nothing in the state prevents the record (blocked: %q)",
				command.name, index, after.shows[index], before.stored, after.stored, w.blocked)
		}
		if chose := before.shows[index] != "" && !before.follows[index]; chose && !command.explicit && after.stored != before.shows[index] {
			t.Errorf("%s neither followed nor left the model chosen in file %d: it showed %q, stored %q -> %q",
				command.name, index, before.shows[index], before.stored, after.stored)
		}
	}
}

// withdrew checks a withdrawal that succeeded. A file that had a record
// before has none now and no longer selects through AIGW: it holds nothing of
// what AIGW writes there, and it does not still select the model that it
// selected through AIGW's entries. These are the assertions of
// TestAClientIsWithdrawnAfterAModelWasChosenInIt (Codex) and of
// TestAWithdrawalOfAClientThatWaitsForItsTokenRemovesAIGWsPart (Hermes), with
// each world naming AIGW's part of its file (keepsAIGWsPart). A file that had
// no record before is not AIGW's to change, and keeps its bytes.
func (w selectionWorld) withdrew(t *testing.T, before, after selectionSnapshot, command selectionCommand) {
	t.Helper()
	for index := range w.files {
		if before.records[index] == nil {
			if !bytes.Equal(after.files[index], before.files[index]) {
				t.Errorf("%s changed file %d, which had no record of AIGW's before", command.name, index)
			}
			continue
		}
		if after.records[index] != nil {
			t.Errorf("%s succeeded and left AIGW's record of file %d", command.name, index)
		}
		if after.files[index] == nil {
			continue
		}
		if w.keepsAIGWsPart != nil && w.keepsAIGWsPart(after.files[index]) {
			t.Errorf("%s succeeded and left AIGW's part in file %d:\n%s", command.name, index, after.files[index])
		}
		if after.shows[index] != "" && after.shows[index] == before.shows[index] {
			t.Errorf("%s succeeded and file %d still selects %q, which it selected through AIGW", command.name, index, after.shows[index])
		}
	}
}

// twinned compares a run without a Token with its twin: the same state and
// command with every Token. What a command refuses, what it leaves stored as
// the client's selection, and whether the record of each file names what the
// file selects do not depend on a Token. Two differences are meant. They are
// written down here and nowhere else, and every other difference fails:
//
//  1. Without the Token of the selection that a command saves for a client,
//     the client's files are not rewritten; only AIGW's record of them moves.
//     So the twins are not compared by the bytes of the files or by the
//     models that the files show, only by whether each record follows its
//     file.
//  2. `aigw use` refuses when it would have to store a Token that it does not
//     have. A command that selects a model of the provider whose Token is
//     absent (selectionCommand.stores) may be refused where its twin
//     succeeds, and then nothing more is compared.
func (without selectionRun) twinned(t *testing.T, with selectionRun, command selectionCommand, absent string) {
	t.Helper()
	if (with.err == nil) != (without.err == nil) {
		if with.err == nil && command.stores == absent {
			return
		}
		t.Errorf("%s: the Token of %s decides the refusal: with it err=%v, without it err=%v", command.name, absent, with.err, without.err)
		return
	}
	if with.after.stored != without.after.stored {
		t.Errorf("%s: the Token of %s decides what is stored: %q with it, %q without it (before: %q)",
			command.name, absent, with.after.stored, without.after.stored, without.before.stored)
	}
	if !slices.Equal(with.after.follows, without.after.follows) {
		t.Errorf("%s: the Token of %s decides which records name what their files select: %v with it (files show %q), %v without it (files show %q)",
			command.name, absent, with.after.follows, with.after.shows, without.after.follows, without.after.shows)
	}
}
