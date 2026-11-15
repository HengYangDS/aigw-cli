package secret

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"maps"
	"slices"

	"aigw-cli/internal/platform"
)

// Replace applies Account Tokens in stable order and owns their compensation,
// including any automatic backend selection created by these writes. Partial
// failure compensates earlier writes; newer credentials remain untouched.
// Compensation is retryable and becomes inert after success. Store operations
// still require the caller's mutation lock for cross-process serialization.
func Replace(store Store, updates map[string]string) (func() error, error) {
	if len(updates) == 0 {
		return func() error { return nil }, nil
	}
	rollbackBackend, err := prepareBackendSelectionRollback(store)
	if err != nil {
		return nil, err
	}
	rollbacks := make([]func() error, 0, len(updates))
	// restoreTokens puts back, newest first, the Tokens that this call stored.
	restoreTokens := func() error {
		var failure error
		for _, rollback := range slices.Backward(rollbacks) {
			failure = errors.Join(failure, rollback())
		}
		return failure
	}
	compensate := func() error {
		if err := restoreTokens(); err != nil {
			return fmt.Errorf("credential rollback also failed: %w", err)
		}
		if err := rollbackBackend(); err != nil {
			return fmt.Errorf("backend selection rollback also failed: %w", err)
		}
		return nil
	}
	for _, account := range slices.Sorted(maps.Keys(updates)) {
		rollback, err := replaceToken(store, account, updates[account])
		if err != nil {
			// An undo that fails is a failed undo under its own name, which
			// every presentation of the batch's failure then states once. As
			// in the compensation, the backend choice is kept while a Token
			// that this call stored could not be put back.
			failure := fmt.Errorf("store Token for provider %q: %w", account, err)
			if err := restoreTokens(); err != nil {
				return nil, errors.Join(failure, platform.FailedUndo("restoring the Tokens that this command had stored", err))
			}
			return nil, errors.Join(failure, platform.FailedUndo("removing the credential backend choice that this command recorded", rollbackBackend()))
		}
		rollbacks = append(rollbacks, rollback)
	}
	return compensate, nil
}

// replaceToken stores token for account and returns the compensation that
// restores the previous Token. An empty previous Token is no Token: no
// provider accepts it and Set refuses it, so the compensation removes the
// slot, as for an absent one.
func replaceToken(store Store, account, token string) (func() error, error) {
	previous, err := store.Get(account)
	if err != nil && !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrEmpty) {
		return nil, err
	}
	hadPrevious := err == nil
	restore := store
	if view, ok := store.(scopedView); ok {
		if automatic, ok := view.store.(*automaticStore); ok {
			selected, err := automatic.resolve()
			if err != nil {
				return nil, err
			}
			// The compensation changes the slot too, so it shares the scope
			// of the store, which then forgets what it observed of the slot.
			restore = scopedView{store: selected, kind: view.kind, availability: view.availability}
		}
	}
	if err := store.Set(account, token); err != nil {
		return nil, err
	}
	completed := false
	return func() error {
		if completed {
			return nil
		}
		current, err := restore.Get(account)
		if errors.Is(err, ErrNotFound) || err == nil && subtle.ConstantTimeCompare([]byte(current), []byte(token)) != 1 {
			return fmt.Errorf("credential postimage changed for provider %q; refusing to overwrite newer state", account)
		}
		if err != nil {
			return fmt.Errorf("observe Token for provider %q before rollback: %w", account, err)
		}
		if hadPrevious {
			err = restore.Set(account, previous)
		} else {
			err = restore.Delete(account)
		}
		if err != nil {
			return fmt.Errorf("restore Token for provider %q: %w", account, err)
		}
		completed = true
		return nil
	}, nil
}
