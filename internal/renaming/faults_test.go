package renaming

import (
	"errors"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

type failingRenameConfigStore struct{ err error }

func (store failingRenameConfigStore) CaptureSnapshot() (configuration.Snapshot, error) {
	return configuration.Snapshot{}, store.err
}
func (failingRenameConfigStore) Save(configuration.Config) error { return nil }
func (store failingRenameConfigStore) Commit(before configuration.Snapshot, _ configuration.Config) (configuration.Snapshot, error) {
	return before, store.err
}
func (failingRenameConfigStore) RestoreSnapshot(configuration.Snapshot, configuration.Snapshot) error {
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
	return "", secrets.ErrNotFound
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
	if errors.Is(err, secrets.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

type faultProbeStore struct {
	values         map[string]secrets.DiagnosticCredential
	getErrors      map[string]error
	setErr         error
	deleteErr      error
	setReplacement secrets.DiagnosticCredential
	retainDelete   bool
	afterDeleteErr error
	deleted        bool
}

func (store *faultProbeStore) Get(id string) (secrets.DiagnosticCredential, error) {
	if err := store.getErrors[id]; err != nil {
		return secrets.DiagnosticCredential{}, err
	}
	if value, ok := store.values[id]; ok {
		return value, nil
	}
	return secrets.DiagnosticCredential{}, secrets.ErrNotFound
}

func (store *faultProbeStore) Set(id string, value secrets.DiagnosticCredential) error {
	if store.setErr != nil {
		return store.setErr
	}
	if store.setReplacement != (secrets.DiagnosticCredential{}) {
		value = store.setReplacement
	}
	if store.values == nil {
		store.values = map[string]secrets.DiagnosticCredential{}
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
	if errors.Is(err, secrets.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}
