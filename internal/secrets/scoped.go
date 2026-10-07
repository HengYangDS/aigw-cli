package secrets

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
}

type availabilityResult struct {
	present bool
	err     error
}

// ObserveAvailability starts one operation's presence-metadata scope without
// changing its backend or caching credential reads. Purpose views share the
// scope; each new operation gets fresh observations, including prior errors.
// Stores without AIGW's typed backend remain unchanged.
func ObserveAvailability(store Store) Store {
	view, ok := store.(scopedView)
	if !ok {
		return store
	}
	view.availability = &availabilityObservation{slots: map[string]availabilityResult{}}
	return view
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
		delete(view.availability.slots, slotName(view.kind, account))
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
		delete(view.availability.slots, slotName(view.kind, account))
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
