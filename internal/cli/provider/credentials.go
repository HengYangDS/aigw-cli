package provider

import (
	"errors"
	"slices"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"
)

// deleteCredentials deletes the provider's Token and balance credentials from
// a writable backend, reading each one first, and returns the restore that
// puts back every one that it deleted. It deletes nothing that it cannot read
// back: a credential that cannot be read refuses the removal before anything
// is written, and a deletion that fails restores the ones before it.
func deleteCredentials(runtime invocation.Context, id string, document removeDocument) (func() error, error) {
	var restores []func() error
	restore := func() error {
		var failure error
		for _, undo := range slices.Backward(restores) {
			failure = errors.Join(failure, undo())
		}
		return failure
	}
	if document.Token != credentialAbsent {
		undo, err := deleteSlot(runtime.Secrets.Get, runtime.Secrets.Set, runtime.Secrets.Delete, id)
		if err != nil {
			return nil, credentialProblem("the "+id+" Token", "aigw check", err, nil)
		}
		restores = append(restores, undo)
	}
	if document.BalanceCredential != credentialAbsent {
		undo, err := deleteSlot(runtime.Providers.Get, runtime.Providers.Set, runtime.Providers.Delete, id)
		if err != nil {
			return nil, credentialProblem("the "+id+" balance credentials", "aigw provider edit "+id+" --no-balance-token", err, restore)
		}
		restores = append(restores, undo)
	}
	return restore, nil
}

// deleteSlot reads one credential slot, deletes it and returns the restore
// that writes the value back. An empty slot needs no restore.
func deleteSlot[T any](get func(string) (T, error), set func(string, T) error, remove func(string) error, id string) (func() error, error) {
	value, err := get(id)
	if errors.Is(err, secret.ErrNotFound) {
		return func() error { return nil }, nil
	}
	if err != nil {
		return nil, err
	}
	if err := remove(id); err != nil && !errors.Is(err, secret.ErrNotFound) {
		return nil, err
	}
	return func() error { return set(id, value) }, nil
}

// credentialProblem explains a credential that a removal could not read or
// delete, after restoring the credentials that it deleted before, so that the
// removal changed nothing; fix clears the credential or diagnoses the backend.
func credentialProblem(subject, fix string, err error, restore func() error) error {
	impact := "Nothing was removed."
	if restore != nil {
		if restoreErr := restore(); restoreErr != nil {
			impact = "The provider was not removed, and a credential that it deleted first could not be restored."
			err = errors.Join(err, restoreErr)
		}
	}
	return terminal.ProblemError(subject+" could not be read and deleted, so the provider was not removed", "", impact, fix, err)
}
