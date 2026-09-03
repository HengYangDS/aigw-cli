// Package secrets owns Account Token storage backends. It never stores tokens
// in AIGW configuration or client projections.
package secrets

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/transaction"
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

// BackendSelection is a secret-free observation of the one credential
// backend selected for this invocation. Persistence describes how the
// selection itself is retained, not whether credential values are durable.
type BackendSelection struct {
	Kind           string `json:"kind"`
	Availability   string `json:"availability"`
	Mutability     string `json:"mutability"`
	Persistence    string `json:"persistence"`
	RecoveryAction string `json:"recovery_action"`
}

type backendInspector interface {
	inspectBackend() (BackendSelection, error)
}

type automaticStore struct {
	selection          Selection
	choice             backendChoice
	mutex              sync.Mutex
	selected           Store
	selectedBackend    string
	selectionPersisted bool
	selectionWrites    uint64
	selectionPostimage transaction.FileSnapshot
}

// PrepareBackendSelectionRollback captures the automatic backend selection
// before a larger transaction first observes credentials. The returned
// compensation removes only a selection written by that transaction and
// refuses to overwrite a newer external choice. Explicit stores have no
// automatic selection to compensate.
func PrepareBackendSelectionRollback(store Store) (func() error, error) {
	automatic, ok := store.(*automaticStore)
	if !ok {
		return func() error { return nil }, nil
	}
	automatic.mutex.Lock()
	defer automatic.mutex.Unlock()
	_, err := automatic.choice.Read()
	if err == nil {
		return func() error { return nil }, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("observe automatic credential backend selection: %w", err)
	}
	preparedWrites := automatic.selectionWrites
	return func() error {
		automatic.mutex.Lock()
		defer automatic.mutex.Unlock()
		if automatic.selectionWrites == preparedWrites {
			return nil
		}
		selectionPath := filepath.Join(automatic.selection.Root, "backend")
		if err := transaction.RestoreFileAtomicIfPostimage(selectionPath, transaction.FileSnapshot{}, automatic.selectionPostimage); err != nil {
			return fmt.Errorf("restore automatic credential backend selection: %w", err)
		}
		automatic.selectionPersisted = false
		automatic.selectionWrites = preparedWrites
		return nil
	}, nil
}

func IsReadOnly(store Store) bool {
	reporter, ok := store.(interface{ ReadOnly() bool })
	return ok && reporter.ReadOnly()
}

// Inspect reports backend capability without reading credential values or
// persisting an automatic selection.
func Inspect(store Store) (BackendSelection, error) {
	if store == nil {
		err := errors.New("credential backend is unavailable")
		return unavailableBackendSelection(), err
	}
	if inspector, ok := store.(backendInspector); ok {
		return inspector.inspectBackend()
	}
	if view, ok := store.(scopedView); ok {
		return Inspect(view.store)
	}
	persistence := "explicit"
	if _, ok := store.(*MemoryStore); ok {
		persistence = "ephemeral"
	}
	return availableBackendSelection(backendKind(store), IsReadOnly(store), persistence), nil
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
			store.selectionWrites++
			store.selectionPostimage = transaction.ExactModeWritePostimage([]byte(store.selectedBackend+"\n"), 0o600)
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
	if persist && !persisted {
		if err := store.choice.Write(backend); err != nil {
			return nil, err
		}
		persisted = true
		store.selectionWrites++
		store.selectionPostimage = transaction.ExactModeWritePostimage([]byte(backend+"\n"), 0o600)
	}
	store.selected = selected
	store.selectedBackend = backend
	store.selectionPersisted = persisted
	return requireTypedStore(selected)
}

func (store *automaticStore) inspectBackend() (BackendSelection, error) {
	selected, err := store.resolve(false)
	if err != nil {
		return unavailableBackendSelection(), err
	}
	store.mutex.Lock()
	backend := store.selectedBackend
	persistence := "deferred"
	if store.selectionPersisted {
		persistence = "persisted"
	}
	store.mutex.Unlock()
	return availableBackendSelection(backend, IsReadOnly(selected), persistence), nil
}

func backendKind(store Store) string {
	switch store.(type) {
	case KeyringStore:
		return "keyring"
	case EnvironmentStore:
		return "env"
	case *fileStore:
		return "file"
	case *MemoryStore:
		return "memory"
	default:
		return "unknown"
	}
}

func availableBackendSelection(kind string, readOnly bool, persistence string) BackendSelection {
	mutability := "read_write"
	if readOnly {
		mutability = "read_only"
	}
	return BackendSelection{
		Kind:         kind,
		Availability: "available",
		Mutability:   mutability,
		Persistence:  persistence,
	}
}

func unavailableBackendSelection() BackendSelection {
	return BackendSelection{
		Kind:           "unknown",
		Availability:   "unavailable",
		Mutability:     "unknown",
		Persistence:    "unknown",
		RecoveryAction: "aigw doctor",
	}
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
	store.selectionWrites++
	store.selectionPostimage = transaction.ExactModeWritePostimage([]byte(store.selectedBackend+"\n"), 0o600)
	return nil
}

func probeKeyring(store Store, probe func(Store) error) error {
	if probe != nil {
		return probe(store)
	}
	_, err := store.Exists("aigw-backend-probe")
	return err
}
