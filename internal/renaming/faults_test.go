package renaming

import (
	"errors"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
)

type failingRenameConfigStore struct{ err error }

func (configStore failingRenameConfigStore) CaptureSnapshot() (store.Snapshot, error) {
	return store.Snapshot{}, configStore.err
}
func (failingRenameConfigStore) Save(domain.Config) error { return nil }
func (configStore failingRenameConfigStore) Commit(before store.Snapshot, _ domain.Config) (store.Snapshot, error) {
	return before, configStore.err
}
func (failingRenameConfigStore) RestoreSnapshot(store.Snapshot, store.Snapshot) error {
	return nil
}

type faultTokenStore struct {
	values         map[string]string
	getErrors      map[string]error
	setErr         error
	deleteErr      error
	setReplacement string
	retainDelete   bool
	afterDeleteErr error
	deleted        bool
}

func (store *faultTokenStore) Get(id string) (string, error) {
	if err := store.getErrors[id]; err != nil {
		return "", err
	}
	if value, ok := store.values[id]; ok {
		return value, nil
	}
	return "", secret.ErrNotFound
}

func (store *faultTokenStore) Set(id, value string) error {
	if store.setErr != nil {
		return store.setErr
	}
	if store.setReplacement != "" {
		value = store.setReplacement
	}
	if store.values == nil {
		store.values = map[string]string{}
	}
	store.values[id] = value
	return nil
}

func (store *faultTokenStore) Delete(id string) error {
	if store.deleteErr != nil {
		return store.deleteErr
	}
	store.deleted = true
	if !store.retainDelete {
		delete(store.values, id)
	}
	return nil
}

func (store *faultTokenStore) Exists(id string) (bool, error) {
	if store.deleted {
		_, present := store.values[id]
		return present, store.afterDeleteErr
	}
	_, err := store.Get(id)
	if errors.Is(err, secret.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

type faultProbeStore struct {
	values         map[string]secret.DiagnosticCredential
	getErrors      map[string]error
	setErr         error
	deleteErr      error
	setReplacement secret.DiagnosticCredential
	retainDelete   bool
	afterDeleteErr error
	deleted        bool
}

func (store *faultProbeStore) Get(id string) (secret.DiagnosticCredential, error) {
	if err := store.getErrors[id]; err != nil {
		return secret.DiagnosticCredential{}, err
	}
	if value, ok := store.values[id]; ok {
		return value, nil
	}
	return secret.DiagnosticCredential{}, secret.ErrNotFound
}

func (store *faultProbeStore) Set(id string, value secret.DiagnosticCredential) error {
	if store.setErr != nil {
		return store.setErr
	}
	if store.setReplacement != (secret.DiagnosticCredential{}) {
		value = store.setReplacement
	}
	if store.values == nil {
		store.values = map[string]secret.DiagnosticCredential{}
	}
	store.values[id] = value
	return nil
}

func (store *faultProbeStore) Delete(id string) error {
	if store.deleteErr != nil {
		return store.deleteErr
	}
	store.deleted = true
	if !store.retainDelete {
		delete(store.values, id)
	}
	return nil
}

func (store *faultProbeStore) Exists(id string) (bool, error) {
	if store.deleted {
		_, present := store.values[id]
		return present, store.afterDeleteErr
	}
	_, err := store.Get(id)
	if errors.Is(err, secret.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}
