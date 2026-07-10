package secrets

import (
	"errors"
	"fmt"

	keyring "github.com/zalando/go-keyring"
)

type KeyringStore struct{}

func NewKeyringStore() KeyringStore { return KeyringStore{} }

func (KeyringStore) Get(profile string) (string, error) {
	if err := validate(profile, "", false); err != nil {
		return "", err
	}
	value, err := keyring.Get(Service, profile)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read %s/%s from system keyring: %w", Service, profile, err)
	}
	if value == "" {
		return "", ErrNotFound
	}
	return value, nil
}

func (KeyringStore) Set(profile, value string) error {
	if err := validate(profile, value, true); err != nil {
		return err
	}
	if err := keyring.Set(Service, profile, value); err != nil {
		return fmt.Errorf("write %s/%s to system keyring: %w", Service, profile, err)
	}
	return nil
}

func (KeyringStore) Delete(profile string) error {
	if err := validate(profile, "", false); err != nil {
		return err
	}
	err := keyring.Delete(Service, profile)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete %s/%s from system keyring: %w", Service, profile, err)
	}
	return nil
}

func (s KeyringStore) Has(profile string) bool {
	_, err := s.Get(profile)
	return err == nil
}
