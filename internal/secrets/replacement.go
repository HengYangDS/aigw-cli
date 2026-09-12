package secrets

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"maps"
	"slices"
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
	compensate := func() error {
		var failure error
		for _, rollback := range slices.Backward(rollbacks) {
			failure = errors.Join(failure, rollback())
		}
		if failure != nil {
			return fmt.Errorf("credential rollback also failed: %w", failure)
		}
		if err := rollbackBackend(); err != nil {
			return fmt.Errorf("backend selection rollback also failed: %w", err)
		}
		return nil
	}
	for _, account := range slices.Sorted(maps.Keys(updates)) {
		rollback, err := replaceToken(store, account, updates[account])
		if err != nil {
			return nil, errors.Join(fmt.Errorf("store Token for Account %q: %w", account, err), compensate())
		}
		rollbacks = append(rollbacks, rollback)
	}
	return compensate, nil
}

func replaceToken(store Store, account, token string) (func() error, error) {
	previous, err := store.Get(account)
	if err != nil && !errors.Is(err, ErrNotFound) {
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
			restore = scopedView{store: selected, kind: view.kind}
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
			return fmt.Errorf("credential postimage changed for Account %q; refusing to overwrite newer state", account)
		}
		if err != nil {
			return fmt.Errorf("observe Token for Account %q before rollback: %w", account, err)
		}
		if hadPrevious {
			err = restore.Set(account, previous)
		} else {
			err = restore.Delete(account)
		}
		if err != nil {
			return fmt.Errorf("restore Token for Account %q: %w", account, err)
		}
		completed = true
		return nil
	}, nil
}
