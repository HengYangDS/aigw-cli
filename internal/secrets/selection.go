package secrets

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

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

type automaticStore struct {
	selection          Selection
	choice             backendChoice
	mutex              sync.Mutex
	selected           credentialBackend
	selectedBackend    string
	selectionWrites    uint64
	selectionPostimage credentialFileSnapshot
}

// prepareBackendSelectionRollback captures the automatic backend selection
// before a larger transaction first observes credentials. The returned
// compensation removes only a selection written by that transaction and
// refuses to overwrite a newer external choice. After successful completion,
// repeated calls are inert. Explicit stores have no automatic selection to compensate.
func prepareBackendSelectionRollback(store Store) (func() error, error) {
	automatic, ok := backendStore(store).(*automaticStore)
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
	completed := false
	return func() error {
		automatic.mutex.Lock()
		defer automatic.mutex.Unlock()
		if completed || automatic.selectionWrites == preparedWrites {
			completed = true
			return nil
		}
		if err := automatic.choice.Rollback(automatic.selectionPostimage); err != nil {
			return fmt.Errorf("restore automatic credential backend selection: %w", err)
		}
		automatic.selectionWrites = preparedWrites
		automatic.selectionPostimage = credentialFileSnapshot{}
		completed = true
		return nil
	}, nil
}

// Select chooses one credential backend from explicit policy and observed platform capabilities.
func Select(selection Selection) (Store, error) {
	backend, err := selectBackend(selection)
	if err != nil {
		return nil, err
	}
	return scopedView{store: backend}, nil
}

func selectBackend(selection Selection) (credentialBackend, error) {
	getenv := selection.Getenv
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	switch selection.Backend {
	case "keyring":
		store := keyringStore{observe: observeKeyringItem}
		if err := probeKeyring(scopedView{store: store}, selection.KeyringProbe); err != nil {
			return nil, fmt.Errorf("use keyring secret backend: %w", err)
		}
		return store, nil
	case "env":
		return environmentStore{getenv: getenv}, nil
	case "file":
		if selection.GOOS != "darwin" && selection.GOOS != "linux" && selection.GOOS != "windows" {
			return nil, fmt.Errorf("file secret backend is not supported on operating system %q", selection.GOOS)
		}
		if selection.Root == "" {
			return nil, errors.New("file secret backend requires an AIGW storage root")
		}
		return &fileStore{root: filepath.Join(selection.Root, "tokens")}, nil
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

func (store *automaticStore) get(kind Kind, account string) (string, error) {
	selected, err := store.resolve()
	if err != nil {
		return "", err
	}
	return selected.get(kind, account)
}

func (store *automaticStore) set(kind Kind, account, value string) error {
	return store.mutate(func(selected credentialBackend) error {
		return selected.set(kind, account, value)
	})
}

func (store *automaticStore) delete(kind Kind, account string) error {
	return store.mutate(func(selected credentialBackend) error {
		return selected.delete(kind, account)
	})
}

func (store *automaticStore) exists(kind Kind, account string) (bool, error) {
	selected, err := store.resolve()
	if err != nil {
		return false, err
	}
	return selected.exists(kind, account)
}

func (store *automaticStore) resolve() (credentialBackend, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	return store.resolveLocked()
}

func (store *automaticStore) resolveLocked() (credentialBackend, error) {
	if store.selected != nil {
		return store.selected, nil
	}
	backend, err := store.choice.Read()
	persisted := err == nil
	if errors.Is(err, ErrNotFound) {
		backend = "keyring"
	} else if err != nil {
		return nil, err
	}
	selection := store.selection
	selection.Backend = backend
	selected, err := selectBackend(selection)
	if err != nil && !persisted {
		selection.Backend = "file"
		selected, err = selectBackend(selection)
	}
	if err != nil {
		return nil, err
	}
	store.selected = selected
	store.selectedBackend = selection.Backend
	return selected, nil
}

func (store *automaticStore) mutate(operation func(credentialBackend) error) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	selected, err := store.resolveLocked()
	if err != nil {
		return err
	}
	wroteSelection, err := store.persistSelectionLocked()
	if err != nil {
		return err
	}
	if err := operation(selected); err != nil {
		if !wroteSelection {
			return err
		}
		if rollbackErr := store.choice.Rollback(store.selectionPostimage); rollbackErr != nil {
			return errors.Join(err, fmt.Errorf("restore automatic credential backend selection: %w", rollbackErr))
		}
		store.selectionWrites--
		store.selectionPostimage = credentialFileSnapshot{}
		return err
	}
	return nil
}

func (store *automaticStore) inspectBackend() (BackendSelection, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	selected, err := store.resolveLocked()
	if err != nil {
		return unavailableBackendSelection(), err
	}
	backend, err := store.choice.Read()
	if err != nil && !errors.Is(err, ErrNotFound) {
		return unavailableBackendSelection(), err
	}
	persistence := "deferred"
	if err == nil {
		if backend != store.selectedBackend {
			return unavailableBackendSelection(), errors.New("secret backend selection changed; retry the credential operation")
		}
		persistence = "persisted"
	}
	return availableBackendSelection(store.selectedBackend, IsReadOnly(scopedView{store: selected}), persistence), nil
}

func backendKind(store credentialBackend) string {
	switch store.(type) {
	case keyringStore:
		return "keyring"
	case environmentStore:
		return "env"
	case *fileStore:
		return "file"
	case *memoryStore:
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

// Inspect reports backend capability without reading credential values or
// persisting an automatic selection.
func Inspect(store Store) (BackendSelection, error) {
	if store == nil {
		err := errors.New("credential backend is unavailable")
		return unavailableBackendSelection(), err
	}
	backend := backendStore(store)
	if automatic, ok := backend.(*automaticStore); ok {
		return automatic.inspectBackend()
	}
	persistence := "explicit"
	if _, ok := backend.(*memoryStore); ok {
		persistence = "ephemeral"
	}
	return availableBackendSelection(backendKind(backend), IsReadOnly(store), persistence), nil
}

func (store *automaticStore) persistSelectionLocked() (bool, error) {
	postimage, written, err := store.choice.Persist(store.selectedBackend)
	if err != nil {
		return false, err
	}
	if written {
		store.selectionWrites++
		store.selectionPostimage = postimage
	}
	return written, nil
}

func probeKeyring(store Store, probe func(Store) error) error {
	if probe != nil {
		return probe(store)
	}
	_, err := store.Exists("aigw-backend-probe")
	return err
}

const backendChoiceName = "backend"

type backendChoice struct{ root string }

func newBackendChoice(root string) backendChoice { return backendChoice{root: root} }

func (choice backendChoice) Read() (string, error) {
	root, err := openSecureRoot(choice.root, false)
	if err != nil {
		return "", err
	}
	if root == nil {
		return "", ErrNotFound
	}
	defer func() { _ = root.Close() }()
	value, err := readSecureFile(root, backendChoiceName)
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read secret backend selection: %w", err)
	}
	backend := strings.TrimSpace(string(value))
	if !validPersistedBackend(backend) {
		return "", fmt.Errorf("invalid persisted secret backend %q", backend)
	}
	return backend, nil
}

func (choice backendChoice) Persist(backend string) (credentialFileSnapshot, bool, error) {
	if !validPersistedBackend(backend) {
		return credentialFileSnapshot{}, false, fmt.Errorf("invalid secret backend %q", backend)
	}
	root, err := openSecureRoot(choice.root, true)
	if err != nil {
		return credentialFileSnapshot{}, false, err
	}
	defer func() { _ = root.Close() }()
	preimage, err := captureOptionalSecureFile(root, backendChoiceName)
	if err != nil {
		return credentialFileSnapshot{}, false, err
	}
	if preimage.exists {
		defer clear(preimage.value)
		selected := strings.TrimSpace(string(preimage.value))
		if !validPersistedBackend(selected) {
			return credentialFileSnapshot{}, false, fmt.Errorf("invalid persisted secret backend %q", selected)
		}
		if selected != backend {
			return credentialFileSnapshot{}, false, errors.New("secret backend selection changed; retry the credential operation")
		}
		return credentialFileSnapshot{}, false, nil
	}
	postimage, err := writeSecureFileFromPreimage(root, backendChoiceName, preimage, []byte(backend+"\n"))
	if err != nil {
		return credentialFileSnapshot{}, false, fmt.Errorf("write secret backend selection: %w", err)
	}
	return postimage, true, nil
}

func (choice backendChoice) Rollback(postimage credentialFileSnapshot) error {
	if !postimage.exists || postimage.identity.info == nil {
		return errors.New("secret backend selection rollback requires an owned postimage")
	}
	root, err := openSecureRoot(choice.root, false)
	if err != nil {
		return err
	}
	if root == nil {
		return fmt.Errorf("secret backend selection changed; refusing to remove newer state")
	}
	defer func() { _ = root.Close() }()
	if err := deleteSecureFileIf(root, backendChoiceName, postimage); err != nil {
		return fmt.Errorf("remove secret backend selection: %w", err)
	}
	return nil
}

func validPersistedBackend(backend string) bool {
	return backend == "keyring" || backend == "file"
}
