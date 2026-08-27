package secrets

import (
	"errors"
	"fmt"

	keyring "github.com/zalando/go-keyring"
)

type KeyringStore struct{}

func NewKeyringStore() KeyringStore { return KeyringStore{} }

func (KeyringStore) Get(profile string) (string, error) {
	return KeyringStore{}.get(APIToken, profile)
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

func (KeyringStore) Set(profile, value string) error {
	return KeyringStore{}.set(APIToken, profile, value)
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

func (KeyringStore) Delete(profile string) error {
	return KeyringStore{}.delete(APIToken, profile)
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

func (s KeyringStore) Has(profile string) bool {
	_, err := s.Get(profile)
	return err == nil
}
