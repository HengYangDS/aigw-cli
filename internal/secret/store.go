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
// provider is not one of providers, ignoring case, because a file system that
// ignores case keeps both spellings in one slot. It reads names only. It
// fails, without claiming there are none, when the backend cannot name its
// Tokens.
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
	unused := make([]string, 0, len(listed))
	for _, id := range listed {
		if !slices.ContainsFunc(providers, func(provider string) bool { return domain.SameIdentifier(slot(provider), slot(id)) }) {
			unused = append(unused, id)
		}
	}
	slices.Sort(unused)
	return slices.Compact(unused), nil
}

// TokenState is the one judgement of a provider's Token that every command
// decides by.
type TokenState uint8

const (
	// TokenAbsent says that the store holds no Token of the provider that
	// AIGW can use: it has no slot, an empty one, or a Keychain item that
	// macOS refuses AIGW, whose refusal names its fix.
	TokenAbsent TokenState = iota
	// TokenUsable says that the slot holds a Token that AIGW can use.
	TokenUsable
	// TokenUnobservable says that the store cannot tell whether it has the
	// slot, as a locked keychain or a Token file that the file store refuses
	// cannot. For each client that uses the Token it counts as an absent one:
	// the client waits for it, and the command goes on for the others.
	TokenUnobservable
)

// TokenObservation is what AIGW learns of a provider's Token slot without the
// value reaching this process.
type TokenObservation struct {
	// Present says that the store has a slot for the provider.
	Present bool
	// Empty says that the slot holds no value, which is no Token.
	Empty bool
	// Refusal is the refusal of the slot's Keychain item when macOS refuses
	// AIGW that item while its keychain is unlocked. The item is there, and it
	// is no Token that AIGW can use; the refusal owns its title and its fix.
	Refusal *KeychainItemRefusedError
	// Unobserved is the store's failure to tell whether it has the slot, as
	// Exists returns it, kept for the report that names the failure and its
	// fix. The other fields are then zero.
	Unobserved error
}

// State judges the observation: a Token that AIGW can use, none, or one that
// the store cannot observe.
func (observation TokenObservation) State() TokenState {
	switch {
	case observation.Unobserved != nil:
		return TokenUnobservable
	case observation.Present && !observation.Empty && observation.Refusal == nil:
		return TokenUsable
	}
	return TokenAbsent
}

// Usable reports whether the slot holds a Token that AIGW can use: it is
// present, holds a value, and is no Keychain item that macOS refuses AIGW.
func (observation TokenObservation) Usable() bool {
	return observation.State() == TokenUsable
}

// ObserveToken observes account's Token slot. It is the one owner of that
// observation and of its judgement (State), so that every command decides
// from the same facts: whether the store has the slot and, for a slot that it
// has, whether the slot is empty or is a Keychain item that macOS refuses
// AIGW. A store that cannot tell presence leaves its failure in Unobserved. A
// backend that cannot tell whether a slot is empty reports one that is not,
// and its Get reports ErrEmpty. One availability scope asks the backend once
// about each slot. A store that stands in front of another, as a preview does
// that presumes the Token that its command would store, answers the
// observation itself. Without a store there is no Token.
func ObserveToken(store Store, account string) TokenObservation {
	if store == nil {
		return TokenObservation{}
	}
	if observer, ok := store.(interface {
		ObserveToken(account string) TokenObservation
	}); ok {
		return observer.ObserveToken(account)
	}
	present, err := store.Exists(account)
	switch {
	case err != nil:
		return TokenObservation{Unobserved: err}
	case !present:
		return TokenObservation{}
	}
	empty, err := tokenSlotEmpty(store, account)
	refusal, _ := errors.AsType[*KeychainItemRefusedError](err)
	return TokenObservation{Present: true, Empty: empty, Refusal: refusal}
}

// slotEmptiness is implemented by a backend that tells whether a present slot
// holds nothing, from its metadata or from a read that keeps only emptiness.
type slotEmptiness interface {
	holdsNoValue(kind Kind, account string) (bool, error)
}

// tokenSlotEmpty reports that the backend's metadata shows account's Token
// slot present and empty, which holds no Token. It returns no value. A backend
// whose metadata and reader cannot tell reports false, and its Get reports
// ErrEmpty. The system keyring reads in its worker and keeps only emptiness.
// The one observation that fails is of a Keychain item that macOS refuses
// AIGW while its keychain is unlocked, whose refusal it returns: that item is
// there, and it is no Token that AIGW can use. Every other observation that
// fails reports a slot that is not empty.
func tokenSlotEmpty(store Store, account string) (bool, error) {
	backend := backendStore(store)
	if automatic, ok := backend.(*automaticStore); ok {
		selected, err := automatic.resolve()
		if err != nil {
			return false, nil
		}
		backend = selected
	}
	observer, ok := backend.(slotEmptiness)
	if !ok {
		return false, nil
	}
	ask := func() (bool, error) { return emptinessObserved(observer.holdsNoValue(APIToken, account)) }
	if view, scoped := store.(scopedView); scoped && view.availability != nil {
		return view.availability.emptiness(slotName(APIToken, account), ask)
	}
	return ask()
}

// emptinessObserved keeps of a backend's answer what a caller acts on:
// whether the slot is empty, or the refusal of its Keychain item. Any other
// failure is a slot that is not empty.
func emptinessObserved(empty bool, err error) (bool, error) {
	if _, refused := errors.AsType[*KeychainItemRefusedError](err); refused {
		return false, err
	}
	return err == nil && empty, nil
}

// IsReadOnly reports whether a Store can persist credential mutations.
func IsReadOnly(store Store) bool {
	reporter, ok := store.(interface{ ReadOnly() bool })
	return ok && reporter.ReadOnly()
}

func validate(account, value string, requireValue bool) error {
	if !domain.ValidIdentifier(account) {
		return fmt.Errorf("invalid provider ID %q", account)
	}
	if requireValue && value == "" {
		return errors.New("empty secret refused")
	}
	return nil
}
