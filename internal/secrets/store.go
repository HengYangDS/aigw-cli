// Package secrets owns Account Token storage backends. It never stores tokens
// in AIGW configuration or client projections.
package secrets

import (
	"errors"
	"fmt"

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
	Has(profile string) bool
}

func IsReadOnly(store Store) bool {
	reporter, ok := store.(interface{ ReadOnly() bool })
	return ok && reporter.ReadOnly()
}

func validate(profile, value string, requireValue bool) error {
	if !configuration.ValidProfileName(profile) {
		return fmt.Errorf("invalid profile name %q", profile)
	}
	if requireValue && value == "" {
		return errors.New("empty secret refused")
	}
	return nil
}

func Select(backend string, getenv func(string) string) (Store, error) {
	switch backend {
	case "", "keyring":
		return NewKeyringStore(), nil
	case "env":
		return NewEnvironmentStore(getenv), nil
	default:
		return nil, fmt.Errorf("unsupported secret backend %q; supported backends are keyring and env", backend)
	}
}
