package secrets

import (
	"errors"
	"fmt"

	keyring "github.com/zalando/go-keyring"
)

type KeyringStore struct {
	observe func(service, slot string) (bool, error)
}

func NewKeyringStore() KeyringStore { return KeyringStore{observe: observeKeyringItem} }

func (store KeyringStore) Get(profile string) (string, error) {
	return store.get(APIToken, profile)
}

func (KeyringStore) get(kind Kind, profile string) (string, error) {
	if err := validate(profile, "", false); err != nil {
		return "", err
	}
	slot := slotName(kind, profile)
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

func (store KeyringStore) Set(profile, value string) error {
	return store.set(APIToken, profile, value)
}

func (KeyringStore) set(kind Kind, profile, value string) error {
	if err := validate(profile, value, true); err != nil {
		return err
	}
	slot := slotName(kind, profile)
	if err := keyring.Set(Service, slot, value); err != nil {
		return fmt.Errorf("write %s/%s to system keyring: %w", Service, slot, err)
	}
	return nil
}

func (store KeyringStore) Delete(profile string) error {
	return store.delete(APIToken, profile)
}

func (KeyringStore) delete(kind Kind, profile string) error {
	if err := validate(profile, "", false); err != nil {
		return err
	}
	slot := slotName(kind, profile)
	err := keyring.Delete(Service, slot)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete %s/%s from system keyring: %w", Service, slot, err)
	}
	return nil
}

func (store KeyringStore) Exists(profile string) (bool, error) {
	return store.exists(APIToken, profile)
}

func (store KeyringStore) exists(kind Kind, profile string) (bool, error) {
	if err := validate(profile, "", false); err != nil {
		return false, err
	}
	present, err := store.observe(Service, slotName(kind, profile))
	if err != nil {
		return false, fmt.Errorf("observe %s/%s in system keyring: %w", Service, slotName(kind, profile), err)
	}
	return present, nil
}
