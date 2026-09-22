// Package secrets owns Account credential storage and transactional replacement.
// It never stores secrets in AIGW configuration or client projections.
package secrets

import (
	"errors"
	"fmt"

	configuration "aigw-cli/internal/configuration"
)

// Service is the stable native-credential service identity owned by AIGW.
const Service = "AIGW_TOKEN"

var (
	// ErrNotFound reports that a requested credential slot is absent.
	ErrNotFound = errors.New("secret not found")
	// ErrReadOnly reports that the selected credential backend cannot persist mutations.
	ErrReadOnly = errors.New("secret store is read-only")
)

// Store is the credential boundary used by Account-scoped AIGW operations.
type Store interface {
	Get(account string) (string, error)
	Set(account, value string) error
	Delete(account string) error
	Exists(account string) (bool, error)
}

type credentialBackend interface {
	get(kind Kind, account string) (string, error)
	set(kind Kind, account, value string) error
	delete(kind Kind, account string) error
	exists(kind Kind, account string) (bool, error)
}

// IsReadOnly reports whether a Store can persist credential mutations.
func IsReadOnly(store Store) bool {
	reporter, ok := store.(interface{ ReadOnly() bool })
	return ok && reporter.ReadOnly()
}

func validate(account, value string, requireValue bool) error {
	if !configuration.ValidIdentifier(account) {
		return fmt.Errorf("invalid Account identifier %q", account)
	}
	if requireValue && value == "" {
		return errors.New("empty secret refused")
	}
	return nil
}
