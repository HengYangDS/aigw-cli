// Package secret owns Account credential storage and transactional replacement,
// and the private credential reader that native clients run for a Token. It
// never stores secrets in AIGW configuration or client projections.
package secret

import (
	"errors"
	"fmt"
	"slices"

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
	// list names the providers that hold a stored Token. It reads no value,
	// prompts for nothing, and fails when the store would need either.
	list() ([]string, error)
}

// UnusedTokens returns, sorted, the provider IDs of the stored Tokens whose
// provider is not one of providers. It reads names only. It fails, without
// claiming there are none, when the backend cannot name its Tokens.
func UnusedTokens(store Store, providers []string) ([]string, error) {
	backend := backendStore(store)
	if backend == nil {
		return nil, errors.New("this credential store cannot name its Tokens")
	}
	listed, err := backend.list()
	if err != nil {
		return nil, fmt.Errorf("name the stored Tokens: %w", err)
	}
	// The environment backend's IDs decode to lower case, so the variable
	// that an ID selects is what tells two IDs apart there.
	slot := func(id string) string { return id }
	if _, environment := backend.(environmentStore); environment {
		slot = EnvironmentKey
	}
	used := make(map[string]bool, len(providers))
	for _, provider := range providers {
		used[slot(provider)] = true
	}
	unused := make([]string, 0, len(listed))
	for _, id := range listed {
		if !used[slot(id)] {
			unused = append(unused, id)
		}
	}
	slices.Sort(unused)
	return slices.Compact(unused), nil
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
