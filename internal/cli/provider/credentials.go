package provider

import (
	"errors"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"
)

// deleteCredentials deletes the provider's Token and balance credentials from
// a writable backend, reading each one first so that it can be put back if
// the configuration is not saved. A slot that holds nothing needs nothing. A
// credential that cannot be read is still deleted, because the removal means
// to delete it, but nothing can put it back. A deletion that fails undoes the
// ones before it and refuses the removal.
func deleteCredentials(runtime invocation.Context, id string, document removeDocument) ([]credentialChange, error) {
	var changes []credentialChange
	if document.Token != credentialAbsent {
		change, deleted, err := deleteSlot(runtime.Secrets.Get, runtime.Secrets.Set, runtime.Secrets.Delete, id,
			"the Token was put back", "the Token stays deleted")
		if err != nil {
			return nil, deletionFailure(id, "the Token", err, changes)
		}
		if deleted {
			changes = append(changes, change)
		}
	}
	if document.BalanceCredential != credentialAbsent {
		change, deleted, err := deleteSlot(runtime.Providers.Get, runtime.Providers.Set, runtime.Providers.Delete, id,
			"the balance credentials were put back", "the balance credentials stay deleted")
		if err != nil {
			return nil, deletionFailure(id, "the balance credentials", err, changes)
		}
		if deleted {
			changes = append(changes, change)
		}
	}
	return changes, nil
}

// deleteSlot reads one credential slot and deletes it, and returns the change
// that puts the value back, without an undo when the value could not be read.
// It reports whether it deleted anything: a slot that holds nothing is left
// alone.
func deleteSlot[T any](get func(string) (T, error), set func(string, T) error, remove func(string) error, id, undone, kept string) (credentialChange, bool, error) {
	value, readErr := get(id)
	if errors.Is(readErr, secret.ErrNotFound) {
		return credentialChange{}, false, nil
	}
	if err := remove(id); err != nil && !errors.Is(err, secret.ErrNotFound) {
		return credentialChange{}, false, err
	}
	change := credentialChange{undone: undone, kept: kept}
	if readErr == nil {
		change.undo = func() error { return set(id, value) }
	}
	return change, true, nil
}

// deletionFailure refuses a removal whose deletion of subject failed with err,
// after undoing the deletions before it. When every one is undone nothing was
// removed, and the fix shows the provider's credentials as the backend reports
// them, which tells whether it answers again. When an earlier deletion stays,
// the problem names it, and rerunning the removal converges.
func deletionFailure(id, subject string, err error, changes []credentialChange) error {
	outcome := undoChanges(changes)
	if len(outcome.kept) == 0 {
		return terminal.ProblemError("provider "+id+" was not removed: "+subject+" could not be deleted", "",
			"Nothing was removed"+clauses("; ", outcome.undone)+".", "aigw provider show "+id, err)
	}
	return keptFailure(sentence(subject)+" could not be deleted: "+terminal.ProblemFor(err).Title+".", err, outcome,
		"provider "+id+" was not removed", "aigw provider remove "+id)
}
