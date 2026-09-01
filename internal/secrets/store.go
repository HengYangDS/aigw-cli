// Package secrets owns Account Token storage backends. It never stores tokens
// in AIGW configuration or client projections.
package secrets

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	configuration "aigw-cli/internal/configuration"
)

const Service = "AIGW_TOKEN"

var (
	ErrNotFound = errors.New("secret not found")
	ErrReadOnly = errors.New("secret store is read-only")
)

type Store interface {
	Get(profile string) (string, error)
	Set(profile, value string) error
	Delete(profile string) error
	Exists(profile string) (bool, error)
}

type typedStore interface {
	Store
	get(kind Kind, account string) (string, error)
	set(kind Kind, account, value string) error
	delete(kind Kind, account string) error
	exists(kind Kind, account string) (bool, error)
}

// Selection is the complete host snapshot used to select one Token store.
type Selection struct {
	Backend      string
	GOOS         string
	Root         string
	Getenv       func(string) string
	KeyringProbe func(Store) error
}

type automaticStore struct {
	selection          Selection
	choice             backendChoice
	mutex              sync.Mutex
	selected           Store
	selectedBackend    string
	selectionPersisted bool
}

func IsReadOnly(store Store) bool {
	reporter, ok := store.(interface{ ReadOnly() bool })
	return ok && reporter.ReadOnly()
}

func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

func validate(profile, value string, requireValue bool) error {
	if !configuration.ValidProfileName(profile) {
		return fmt.Errorf("invalid profile name %q", profile)
	}
	if requireValue && value == "" {
		return errors.New("empty secret refused")
	}
	return nil
}

func Select(selection Selection) (Store, error) {
	getenv := selection.Getenv
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	switch selection.Backend {
	case "keyring":
		store := NewKeyringStore()
		if err := probeKeyring(store, selection.KeyringProbe); err != nil {
			return nil, fmt.Errorf("use keyring secret backend: %w", err)
		}
		return store, nil
	case "env":
		return NewEnvironmentStore(getenv), nil
	case "file":
		if selection.GOOS != "darwin" && selection.GOOS != "linux" && selection.GOOS != "windows" {
			return nil, fmt.Errorf("file secret backend is not supported on operating system %q", selection.GOOS)
		}
		if selection.Root == "" {
			return nil, errors.New("file secret backend requires an AIGW storage root")
		}
		return newFileStore(filepath.Join(selection.Root, "tokens")), nil
	case "":
		if selection.GOOS != "darwin" && selection.GOOS != "linux" && selection.GOOS != "windows" {
			return nil, fmt.Errorf("unsupported operating system %q", selection.GOOS)
		}
		if selection.Root == "" {
			return nil, errors.New("automatic secret backend requires an AIGW storage root")
		}
		return &automaticStore{
			selection: selection,
			choice:    newBackendChoice(selection.Root),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported secret backend %q; supported backends are keyring, file, and env", selection.Backend)
	}
}

func (store *automaticStore) Get(profile string) (string, error) {
	return store.get(APIToken, profile)
}

func (store *automaticStore) get(kind Kind, profile string) (string, error) {
	selected, err := store.resolve(false)
	if err != nil {
		return "", err
	}
	value, err := selected.get(kind, profile)
	if err != nil {
		return "", err
	}
	if err := store.persistSelection(); err != nil {
		return "", err
	}
	return value, nil
}

func (store *automaticStore) Set(profile, value string) error {
	return store.set(APIToken, profile, value)
}

func (store *automaticStore) set(kind Kind, profile, value string) error {
	if err := validate(profile, value, true); err != nil {
		return err
	}
	selected, err := store.resolve(true)
	if err != nil {
		return err
	}
	return selected.set(kind, profile, value)
}

func (store *automaticStore) Delete(profile string) error {
	return store.delete(APIToken, profile)
}

func (store *automaticStore) delete(kind Kind, profile string) error {
	if err := validate(profile, "", false); err != nil {
		return err
	}
	selected, err := store.resolve(true)
	if err != nil {
		return err
	}
	return selected.delete(kind, profile)
}

func (store *automaticStore) Exists(profile string) (bool, error) {
	return store.exists(APIToken, profile)
}

func (store *automaticStore) exists(kind Kind, profile string) (bool, error) {
	selected, err := store.resolve(false)
	if err != nil {
		return false, err
	}
	return selected.exists(kind, profile)
}

func (store *automaticStore) resolve(persist bool) (typedStore, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if store.selected != nil {
		if persist && !store.selectionPersisted {
			if err := store.choice.Write(store.selectedBackend); err != nil {
				return nil, err
			}
			store.selectionPersisted = true
		}
		return requireTypedStore(store.selected)
	}
	backend, err := store.choice.Read()
	persisted := err == nil
	if errors.Is(err, ErrNotFound) {
		backend = "keyring"
		if err := probeKeyring(NewKeyringStore(), store.selection.KeyringProbe); err != nil {
			backend = "file"
		}
	} else if err != nil {
		return nil, err
	}
	selected, err := Select(Selection{
		Backend:      backend,
		GOOS:         store.selection.GOOS,
		Root:         store.selection.Root,
		Getenv:       store.selection.Getenv,
		KeyringProbe: store.selection.KeyringProbe,
	})
	if err != nil {
		return nil, err
	}
	if persist {
		if err := store.choice.Write(backend); err != nil {
			return nil, err
		}
		persisted = true
	}
	store.selected = selected
	store.selectedBackend = backend
	store.selectionPersisted = persisted
	return requireTypedStore(selected)
}

func requireTypedStore(store Store) (typedStore, error) {
	typed, ok := store.(typedStore)
	if !ok {
		return nil, fmt.Errorf("credential backend %T does not support typed slots", store)
	}
	return typed, nil
}

func (store *automaticStore) persistSelection() error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if store.selectionPersisted {
		return nil
	}
	if err := store.choice.Write(store.selectedBackend); err != nil {
		return err
	}
	store.selectionPersisted = true
	return nil
}

func probeKeyring(store Store, probe func(Store) error) error {
	if probe != nil {
		return probe(store)
	}
	_, err := store.Exists("aigw-backend-probe")
	return err
}
