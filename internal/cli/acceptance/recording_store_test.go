package cli_test

import (
	"errors"
	"testing"

	"aigw-cli/internal/secret"
)

type recordingCredentialStore[T string | secret.DiagnosticCredential] struct {
	backend interface {
		Get(accountID string) (T, error)
		Set(accountID string, value T) error
		Delete(accountID string) error
		Exists(accountID string) (bool, error)
	}
	getCalls    []string
	existsCalls []string
	setCalls    []string
	deleteCalls []string
	existsErr   error
	getErr      error
	setErr      error
	deleteErr   error
}

func (store *recordingCredentialStore[T]) Get(account string) (T, error) {
	store.getCalls = append(store.getCalls, account)
	if store.getErr != nil {
		var value T
		return value, store.getErr
	}
	return store.backend.Get(account)
}

func (store *recordingCredentialStore[T]) Set(account string, value T) error {
	store.setCalls = append(store.setCalls, account)
	if store.setErr != nil {
		return store.setErr
	}
	return store.backend.Set(account, value)
}

func (store *recordingCredentialStore[T]) Delete(account string) error {
	store.deleteCalls = append(store.deleteCalls, account)
	if store.deleteErr != nil {
		return store.deleteErr
	}
	return store.backend.Delete(account)
}

func (store *recordingCredentialStore[T]) Exists(account string) (bool, error) {
	store.existsCalls = append(store.existsCalls, account)
	if store.existsErr != nil {
		return false, store.existsErr
	}
	return store.backend.Exists(account)
}

func TestCredentialFixturePreservesRealStateAcrossInjectedFailures(t *testing.T) {
	backend := secret.NewMemoryStore()
	store := &recordingCredentialStore[string]{backend: backend}
	if err := store.Set("account", "original"); err != nil {
		t.Fatal(err)
	}
	want := errors.New("credential mutation failed")
	store.setErr, store.deleteErr = want, want
	if err := store.Set("account", "replacement"); !errors.Is(err, want) {
		t.Fatalf("failed write = %v", err)
	}
	if err := store.Delete("account"); !errors.Is(err, want) {
		t.Fatalf("failed deletion = %v", err)
	}
	if value, err := store.Get("account"); err != nil || value != "original" {
		t.Fatalf("failed mutation changed stored value: %q, %v", value, err)
	}
	store.setErr, store.deleteErr = nil, nil
	if err := store.Set("account", "replacement"); err != nil {
		t.Fatal(err)
	}
	if value, err := backend.Get("account"); err != nil || value != "replacement" {
		t.Fatalf("successful write did not reach the backend: %q, %v", value, err)
	}
	if err := store.Delete("account"); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.Get("account"); !errors.Is(err, secret.ErrNotFound) {
		t.Fatalf("successful deletion retained credential: %v", err)
	}
	if len(store.setCalls) != 3 || len(store.deleteCalls) != 2 || len(store.getCalls) != 1 {
		t.Fatal("credential observations lost an attempted operation")
	}
}
