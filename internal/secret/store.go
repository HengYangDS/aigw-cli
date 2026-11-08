// Package secret owns Account credential storage and transactional replacement,
// and the private credential reader that native clients run for a Token. It
// never stores secrets in AIGW configuration or client projections.
package secret

import (
	"errors"
	"fmt"

	"aigw-cli/internal/domain"
)

// Service is the stable native-credential service identity owned by AIGW.
const Service = "AIGW_TOKEN"

var (
	// ErrNotFound reports that a requested credential slot is absent.
	ErrNotFound = errors.New("secret not found")
	// ErrEmpty reports a credential slot that holds an empty value, which no
	// provider accepts: like an absent one, it needs a Token stored again.
	ErrEmpty = errors.New("stored credential is empty")
	// errReadOnly reports that the selected credential backend cannot persist mutations.
	errReadOnly = errors.New("secret store is read-only")
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

// slotEmptiness is implemented by a backend whose metadata tells, without
// reading the value, that a present slot holds nothing.
type slotEmptiness interface {
	holdsNoValue(kind Kind, account string) (bool, error)
}

// TokenSlotEmpty reports that the backend's metadata shows account's Token
// slot present and empty, which holds no Token. It reads no value. A backend
// whose metadata cannot tell reports false, and its Get reports ErrEmpty.
func TokenSlotEmpty(store Store, account string) bool {
	backend := backendStore(store)
	if automatic, ok := backend.(*automaticStore); ok {
		selected, err := automatic.resolve()
		if err != nil {
			return false
		}
		backend = selected
	}
	observer, ok := backend.(slotEmptiness)
	if !ok {
		return false
	}
	empty, err := observer.holdsNoValue(APIToken, account)
	return err == nil && empty
}

// IsReadOnly reports whether a Store can persist credential mutations.
func IsReadOnly(store Store) bool {
	reporter, ok := store.(interface{ ReadOnly() bool })
	return ok && reporter.ReadOnly()
}

func validate(account, value string, requireValue bool) error {
	if !domain.ValidIdentifier(account) {
		return fmt.Errorf("invalid Account identifier %q", account)
	}
	if requireValue && value == "" {
		return errors.New("empty secret refused")
	}
	return nil
}
