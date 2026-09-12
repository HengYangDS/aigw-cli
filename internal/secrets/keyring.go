package secrets

import (
	"errors"
	"fmt"

	keyring "github.com/zalando/go-keyring"
)

// keyringStore persists credentials in the operating system's native credential service.
type keyringStore struct {
	observe func(service, slot string) (bool, error)
}

func (keyringStore) get(kind Kind, account string) (string, error) {
	slot := slotName(kind, account)
	value, err := keyring.Get(Service, slot)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read %s/%s from system keyring: %w", Service, slot, err)
	}
	if value == "" {
		return "", ErrNotFound
	}
	return value, nil
}

func (keyringStore) set(kind Kind, account, value string) error {
	slot := slotName(kind, account)
	if err := keyring.Set(Service, slot, value); err != nil {
		return fmt.Errorf("write %s/%s to system keyring: %w", Service, slot, err)
	}
	return nil
}

func (keyringStore) delete(kind Kind, account string) error {
	slot := slotName(kind, account)
	err := keyring.Delete(Service, slot)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete %s/%s from system keyring: %w", Service, slot, err)
	}
	return nil
}

func (store keyringStore) exists(kind Kind, account string) (bool, error) {
	present, err := store.observe(Service, slotName(kind, account))
	if err != nil {
		return false, fmt.Errorf("observe %s/%s in system keyring: %w", Service, slotName(kind, account), err)
	}
	return present, nil
}
