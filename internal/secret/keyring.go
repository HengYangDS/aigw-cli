package secret

import (
	"errors"
	"fmt"

	"aigw-cli/internal/secret/native"

	keyring "github.com/zalando/go-keyring"
)

// keyringStore persists credentials in the operating system's native credential service.
type keyringStore struct {
	observe   func(service, slot string) (bool, error)
	read      func(service, slot string) (string, error)
	write     func(service, slot, value string) error
	remove    func(service, slot string) error
	enumerate func(service string) ([]string, error)
}

// ErrNativeReaderUnverified marks a failed noninteractive access check for a
// versioned reader. The underlying cause remains internal to the operation.
var ErrNativeReaderUnverified = errors.New("native credential reader access was not verified")

func newKeyringStore(executable string) keyringStore {
	return keyringStore{
		observe: func(service, account string) (bool, error) {
			return native.Exists(executable, service, account)
		},
		read: func(service, account string) (string, error) {
			return native.Read(executable, service, account)
		},
		write: func(service, account, value string) error {
			return native.Write(executable, service, account, value)
		},
		remove:    func(service, account string) error { return native.Delete(executable, service, account) },
		enumerate: func(service string) ([]string, error) { return native.List(executable, service) },
	}
}

// VerifyNativeReaderAccess checks that a copied executable can read every
// preflight-confirmed Account Token from the selected native store before client
// files point at it. Other backends have no per-executable native authorization.
func VerifyNativeReaderAccess(store Store, executable string, accounts []string) error {
	keyring, err := selectedKeyring(store)
	if err != nil || !keyring {
		return err
	}
	for _, account := range accounts {
		if err := validate(account, "", false); err != nil {
			return fmt.Errorf("%w: inspect selected Account Token: %w", ErrNativeReaderUnverified, err)
		}
		value, err := native.Read(executable, Service, account)
		if err != nil {
			return fmt.Errorf("%w: copied credential reader cannot read the selected Account Token: %w", ErrNativeReaderUnverified, err)
		}
		if value == "" {
			return fmt.Errorf("%w: copied credential reader returned an empty Account Token", ErrNativeReaderUnverified)
		}
	}
	return nil
}

// selectedKeyring reports whether the store currently selects the native
// keyring, rechecking an automatic selection.
func selectedKeyring(store Store) (bool, error) {
	if store == nil {
		return false, errors.New("credential backend is unavailable")
	}
	backend := backendStore(store)
	if automatic, ok := backend.(*automaticStore); ok {
		automatic.mutex.Lock()
		selected, _, err := automatic.observeSelectionLocked()
		automatic.mutex.Unlock()
		if err != nil {
			return false, err
		}
		backend = selected
	}
	return backendKind(backend) == "keyring", nil
}

func (store keyringStore) get(kind Kind, account string) (string, error) {
	slot := slotName(kind, account)
	value, err := store.read(Service, slot)
	if errors.Is(err, keyring.ErrNotFound) || errors.Is(err, native.ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read %s/%s from system keyring: %w", Service, slot, err)
	}
	if value == "" {
		return "", ErrEmpty
	}
	return value, nil
}

func (store keyringStore) set(kind Kind, account, value string) error {
	slot := slotName(kind, account)
	if err := store.write(Service, slot, value); err != nil {
		return fmt.Errorf("write %s/%s to system keyring: %w", Service, slot, err)
	}
	return nil
}

func (store keyringStore) delete(kind Kind, account string) error {
	slot := slotName(kind, account)
	err := store.remove(Service, slot)
	if errors.Is(err, keyring.ErrNotFound) || errors.Is(err, native.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete %s/%s from system keyring: %w", Service, slot, err)
	}
	return nil
}

func (store keyringStore) list() ([]string, error) {
	names, err := store.enumerate(Service)
	if err != nil {
		return nil, fmt.Errorf("list %s in system keyring: %w", Service, err)
	}
	return names, nil
}

func (store keyringStore) exists(kind Kind, account string) (bool, error) {
	present, err := store.observe(Service, slotName(kind, account))
	if err != nil {
		return false, fmt.Errorf("observe %s/%s in system keyring: %w", Service, slotName(kind, account), err)
	}
	return present, nil
}
