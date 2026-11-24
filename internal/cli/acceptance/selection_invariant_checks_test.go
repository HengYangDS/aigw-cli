package cli_test

import (
	"bytes"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/domain"
)

// holds checks the one sentence that every command must leave true for a
// client that keeps a record, and returns the client as the command left it.
//
// After a committed command, the record of each of the client's files names
// the model that the file selects, or the client's selection, files and
// records are exactly as before. A preview and a refused command change
// nothing anywhere. A model that a file chose is the stored selection
// afterwards, unless the command selected for the client itself or left the
// client untouched. A rollback that succeeds leaves the restored binding
// stored. A withdrawal disables the client and removes AIGW's part.
//
// "Exactly as before" is accepted only in a state whose own construction
// prevents the record (selectionWorld.blocked), and from a command that asks
// for what is stored already (selectionCommand.idle), which has nothing to
// save and must change nothing at all. In every other case a command that
// succeeds must leave every record naming what its file selects. A withdrawal
// is checked for what it removes (withdrewFaults).
func (w selectionWorld) holds(t *testing.T, before selectionSnapshot, command selectionCommand, err error) selectionSnapshot {
	t.Helper()
	after := w.snapshot(t)
	for _, fault := range w.faults(t, before, after, command, err) {
		t.Error(fault)
	}
	return after
}

// faults returns what the one sentence finds wrong with after, the client as
// a command left it, so that a probe can give the checker an outcome that no
// command produces and require a fault.
func (w selectionWorld) faults(t *testing.T, before, after selectionSnapshot, command selectionCommand, err error) []string {
	t.Helper()
	switch {
	case command.preview || err != nil:
		var faults []string
		if !bytes.Equal(after.config, before.config) || !after.clientUntouched(before) {
			faults = append(faults, fmt.Sprintf("%s (err=%v) changed the configuration, a file or a record: stored %q -> %q, files show %q -> %q",
				command.name, err, before.stored, after.stored, before.shows, after.shows))
		}
		// A command that writes nothing writes nothing anywhere: nothing else
		// of the world changed either, such as a backup beside the
		// configuration.
		if !reflect.DeepEqual(w.folder(t, filepath.Dir(w.app.Config.Path())), before.tree) {
			faults = append(faults, fmt.Sprintf("%s (err=%v) changed a path other than the configuration, a client file or a record", command.name, err))
		}
		return faults
	case command.withdraws:
		return w.withdrewFaults(t, before, after, command)
	default:
		return w.committedFaults(t, before, after, command)
	}
}

// committedFaults checks a command that succeeded and is neither a preview
// nor a withdrawal.
func (w selectionWorld) committedFaults(t *testing.T, before, after selectionSnapshot, command selectionCommand) []string {
	t.Helper()
	faults := w.declaredFaults(before, after, command)
	if command.idle != nil && command.idle(before) {
		if !bytes.Equal(after.config, before.config) || !after.clientUntouched(before) {
			faults = append(faults, fmt.Sprintf("%s asked for what is stored already and changed the configuration, a file or a record: stored %q -> %q, files show %q -> %q",
				command.name, before.stored, after.stored, before.shows, after.shows))
		}
		return faults
	}
	if w.blocked != "" && after.clientUntouched(before) {
		// Something in the state prevents the record, and the command left
		// the choice in its file alone.
		return faults
	}
	return append(faults, w.fileFaults(t, before, after, command)...)
}

// declaredFaults checks what a command itself declares: the binding and the
// selection that a rollback restores, and the setting that it says it saves.
func (w selectionWorld) declaredFaults(before, after selectionSnapshot, command selectionCommand) []string {
	var faults []string
	if command.restores && w.restoredBinding != nil && !reflect.DeepEqual(after.binding, *w.restoredBinding) {
		faults = append(faults, fmt.Sprintf("%s succeeded and left the client's binding %+v, not the restored %+v", command.name, after.binding, *w.restoredBinding))
	}
	if command.restores && w.restored != "" && after.stored != w.restored {
		faults = append(faults, fmt.Sprintf("%s succeeded and left %q stored, not the restored %q", command.name, after.stored, w.restored))
	}
	if command.leaves != nil {
		if missing := command.leaves(after); missing != "" {
			faults = append(faults, fmt.Sprintf("%s succeeded and %s", command.name, missing))
		}
	}
	_ = before
	return faults
}

// fileFaults checks each of the client's files: that the command did not drop
// one, that every record names what its file selects, that a choice is the
// stored selection unless the command selected for the client, and that a
// record belongs to the provider whose endpoint the file reaches.
func (w selectionWorld) fileFaults(t *testing.T, before, after selectionSnapshot, command selectionCommand) []string {
	t.Helper()
	var faults []string
	for index := range w.files {
		if after.files[index] == nil {
			if before.files[index] != nil {
				faults = append(faults, fmt.Sprintf("%s succeeded and removed file %d, which the command did not ask to drop", command.name, index))
			}
			continue
		}
		if !after.follows[index] {
			faults = append(faults, fmt.Sprintf("%s left the record of file %d behind: the file shows %q, stored %q -> %q, and nothing in the state prevents the record (blocked: %q)",
				command.name, index, after.shows[index], before.stored, after.stored, w.blocked))
		}
		if chose := before.shows[index] != "" && !before.follows[index]; chose && !command.explicit && after.stored != before.shows[index] {
			faults = append(faults, fmt.Sprintf("%s neither followed nor left the model chosen in file %d: it showed %q, stored %q -> %q",
				command.name, index, before.shows[index], before.stored, after.stored))
		}
		// The same model ID of two providers is two models. AIGW's record of a
		// file names the model that the file selects through the endpoint that
		// AIGW wrote into it; that endpoint must not be one that another
		// provider declares. An endpoint of the file's own provider, such as
		// the one an edit wrote while the Token that would rewrite it is
		// missing, reaches the same provider and is not a fault.
		if w.reaches != nil && after.follows[index] && after.shows[index] != "" && after.provider != "" {
			reached := w.reaches(t, w.files[index])
			if owner := w.endpointOwner(reached); owner != "" && owner != after.provider {
				faults = append(faults, fmt.Sprintf("%s left file %d selecting %q through an endpoint of another provider (%s), while AIGW records it as the selection of %q",
					command.name, index, after.shows[index], owner, after.provider))
			}
		}
	}
	return faults
}

// endpointOwner names the provider whose declared endpoints include endpoint,
// or "" when no provider declares it or more than one does, as the world's
// stored configuration declares them.
func (w selectionWorld) endpointOwner(endpoint string) string {
	cfg, err := w.app.Config.Load()
	if err != nil {
		return ""
	}
	endpoint = strings.TrimSuffix(endpoint, "/")
	owner := ""
	for id, provider := range cfg.Providers {
		for _, candidate := range []string{provider.Endpoints.Anthropic, provider.Endpoints.Responses, provider.Endpoints.Chat} {
			if candidate == "" || strings.TrimSuffix(candidate, "/") != endpoint {
				continue
			}
			if owner != "" && owner != id {
				return ""
			}
			owner = id
		}
	}
	return owner
}

// withdrewFaults checks a withdrawal that succeeded. A file that had a record
// before has none now and no longer selects through AIGW: it holds nothing of
// what AIGW writes there, and it does not still select the model that it
// selected through AIGW's entries. These are the assertions of
// TestAClientIsWithdrawnAfterAModelWasChosenInIt (Codex) and of
// TestAWithdrawalOfAClientThatWaitsForItsTokenRemovesAIGWsPart (Hermes), with
// each world naming AIGW's part of its file (keepsAIGWsPart). A file that had
// no record before is not AIGW's to change, and keeps its bytes. The client
// itself is disabled in the stored configuration, or the next synchronization
// would write its files again.
func (w selectionWorld) withdrewFaults(t *testing.T, before, after selectionSnapshot, command selectionCommand) []string {
	t.Helper()
	var faults []string
	if after.binding.Enabled {
		faults = append(faults, fmt.Sprintf("%s succeeded and left the client enabled in the stored configuration", command.name))
	}
	for index := range w.files {
		if before.records[index] == nil {
			if !bytes.Equal(after.files[index], before.files[index]) {
				faults = append(faults, fmt.Sprintf("%s changed file %d, which had no record of AIGW's before", command.name, index))
			}
			continue
		}
		if after.records[index] != nil {
			faults = append(faults, fmt.Sprintf("%s succeeded and left AIGW's record of file %d", command.name, index))
		}
		if after.files[index] == nil {
			continue
		}
		if w.keepsAIGWsPart != nil && w.keepsAIGWsPart(after.files[index]) {
			faults = append(faults, fmt.Sprintf("%s succeeded and left AIGW's part in file %d:\n%s", command.name, index, after.files[index]))
		}
		if after.shows[index] != "" && after.shows[index] == before.shows[index] {
			faults = append(faults, fmt.Sprintf("%s succeeded and file %d still selects %q, which it selected through AIGW", command.name, index, after.shows[index]))
		}
	}
	return faults
}

// twinned compares a run without a Token with its twin: the same state and
// command with every Token. What a command refuses, what it leaves stored as
// the client's selection, whether the record of each file names what the file
// selects, and the rest of the client's binding do not depend on a Token. Two
// differences are meant. They are written down here and nowhere else, and
// every other difference fails:
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
	for _, fault := range without.twinnedFaults(with, command, absent) {
		t.Error(fault)
	}
}

// twinnedFaults returns the differences between the twins that a Token must
// not decide.
func (without selectionRun) twinnedFaults(with selectionRun, command selectionCommand, absent string) []string {
	var faults []string
	if (with.err == nil) != (without.err == nil) {
		if with.err == nil && command.stores == absent {
			return nil
		}
		return []string{fmt.Sprintf("%s: the Token of %s decides the refusal: with it err=%v, without it err=%v", command.name, absent, with.err, without.err)}
	}
	if with.after.stored != without.after.stored {
		faults = append(faults, fmt.Sprintf("%s: the Token of %s decides what is stored: %q with it, %q without it (before: %q)",
			command.name, absent, with.after.stored, without.after.stored, without.before.stored))
	}
	if !slices.Equal(with.after.follows, without.after.follows) {
		faults = append(faults, fmt.Sprintf("%s: the Token of %s decides which records name what their files select: %v with it (files show %q), %v without it (files show %q)",
			command.name, absent, with.after.follows, with.after.shows, without.after.follows, without.after.shows))
	}
	// Everything else of the binding is the configuration's own answer and
	// does not depend on a Token: what forwards, what retries, which provider
	// and protocol the selection names. The program and the files are each
	// world's own paths, so they are not compared.
	if with, without := tokenedBinding(with.after.binding), tokenedBinding(without.after.binding); !reflect.DeepEqual(with, without) {
		faults = append(faults, fmt.Sprintf("%s: the Token of %s decides the stored binding: %+v with it, %+v without it",
			command.name, absent, with, without))
	}
	return faults
}

// tokenedBinding is the part of a binding that a Token must not change: what
// the client enables, what it selects, what forwards and what retries. The
// program and the configuration files are each world's own paths, so they
// are not compared.
func tokenedBinding(binding domain.ClientBinding) domain.ClientBinding {
	binding.Executable, binding.Targets = "", nil
	return binding
}
