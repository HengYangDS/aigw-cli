package secret

import (
	"fmt"
	"sync"
)

// Kind identifies a credential purpose independently of the selected storage
// backend. One backend owns every kind for an AIGW installation.
type Kind uint8

const (
	// APIToken identifies the credential used for model inference.
	APIToken Kind = iota
	// ProviderDiagnostic identifies credentials used only by an optional account diagnostic.
	ProviderDiagnostic
)

type scopedView struct {
	store        credentialBackend
	kind         Kind
	availability *availabilityObservation
}

type availabilityObservation struct {
	mutex sync.Mutex
	slots map[string]availabilityResult
	empty map[string]emptinessResult
}

// emptiness runs ask once per slot in this scope and keeps its answer:
// whether the slot is empty, or the refusal of its Keychain item.
func (observation *availabilityObservation) emptiness(slot string, ask func() (bool, error)) (bool, error) {
	observation.mutex.Lock()
	defer observation.mutex.Unlock()
	result, observed := observation.empty[slot]
	if !observed {
		result.empty, result.refusal = ask()
		if observation.empty == nil {
			observation.empty = map[string]emptinessResult{}
		}
		observation.empty[slot] = result
	}
	return result.empty, result.refusal
}

// forget drops what the scope observed of slot, presence and emptiness alike,
// before the operation changes the slot. The caller holds the mutex.
func (observation *availabilityObservation) forget(slot string) {
	delete(observation.slots, slot)
	delete(observation.empty, slot)
}

type availabilityResult struct {
	present bool
	err     error
}

type emptinessResult struct {
	empty   bool
	refusal error
}

// over returns store observing through the scope, or store itself when it is
// not AIGW's typed view.
func (observation *availabilityObservation) over(store Store) Store {
	view, ok := store.(scopedView)
	if !ok {
		return store
	}
	view.availability = observation
	return view
}

// ObserveAvailability starts one operation's presence-metadata scope over one
// credential view, as ObserveCredentials does over an installation's two.
// Purpose views that are derived from the result share its scope.
func ObserveAvailability(store Store) Store {
	observed, _ := ObserveCredentials(store, nil)
	return observed
}

// ObserveCredentials starts one operation's presence-metadata scope over the
// two views of an installation's credentials: tokens, its providers' Tokens,
// and balance, their balance credentials. In the scope each fact of a slot's
// availability is asked of the backend once, through whichever view, and a
// write or a removal through either view makes the scope forget that slot. It
// changes no backend, caches no credential read and leaves its arguments as
// they are; each new operation gets fresh observations, including prior
// errors. A store without AIGW's typed backend, such as a double that a test
// supplies, returns unchanged, and the other one still observes through the
// scope.
func ObserveCredentials(tokens Store, balance DiagnosticCredentialStore) (Store, DiagnosticCredentialStore) {
	scope := &availabilityObservation{slots: map[string]availabilityResult{}}
	if typed, ok := balance.(diagnosticCredentialStore); ok {
		balance = diagnosticCredentialStore{backend: scope.over(typed.backend)}
	}
	return scope.over(tokens), balance
}

// ForKind returns a narrow string-store view over one credential kind without
// selecting, probing, or falling back to another backend.
func ForKind(store Store, kind Kind) (Store, error) {
	if kind != APIToken && kind != ProviderDiagnostic {
		return nil, fmt.Errorf("unsupported credential kind %d", kind)
	}
	view, ok := store.(scopedView)
	if !ok {
		return nil, fmt.Errorf("credential store %T does not expose purpose selection", store)
	}
	view.kind = kind
	return view, nil
}

func (view scopedView) Get(account string) (string, error) {
	if err := validate(account, "", false); err != nil {
		return "", err
	}
	return view.store.get(view.kind, account)
}

func (view scopedView) Set(account, value string) error {
	if err := validate(account, value, true); err != nil {
		return err
	}
	if view.availability != nil {
		view.availability.mutex.Lock()
		defer view.availability.mutex.Unlock()
		view.availability.forget(slotName(view.kind, account))
	}
	return view.store.set(view.kind, account, value)
}

func (view scopedView) Delete(account string) error {
	if err := validate(account, "", false); err != nil {
		return err
	}
	if view.availability != nil {
		view.availability.mutex.Lock()
		defer view.availability.mutex.Unlock()
		view.availability.forget(slotName(view.kind, account))
	}
	return view.store.delete(view.kind, account)
}

func (view scopedView) Exists(account string) (bool, error) {
	if err := validate(account, "", false); err != nil {
		return false, err
	}
	if view.availability == nil {
		return view.store.exists(view.kind, account)
	}
	view.availability.mutex.Lock()
	defer view.availability.mutex.Unlock()
	slot := slotName(view.kind, account)
	result, observed := view.availability.slots[slot]
	if !observed {
		result.present, result.err = view.store.exists(view.kind, account)
		view.availability.slots[slot] = result
	}
	return result.present, result.err
}

func (view scopedView) ReadOnly() bool {
	_, environment := view.store.(environmentStore)
	return environment
}

func backendStore(store Store) credentialBackend {
	if view, ok := store.(scopedView); ok {
		return view.store
	}
	return nil
}

func slotName(kind Kind, account string) string {
	if kind == ProviderDiagnostic {
		return "diagnostic@" + account
	}
	return account
}
