package secrets

import "fmt"

// Kind identifies a credential purpose independently of the selected storage
// backend. One backend owns every kind for an AIGW installation.
type Kind uint8

const (
	APIToken Kind = iota
	ProviderDiagnostic
)

type scopedView struct {
	store typedStore
	kind  Kind
}

// ForKind returns a narrow string-store view over one credential kind without
// selecting, probing, or falling back to another backend.
func ForKind(store Store, kind Kind) (Store, error) {
	scoped, err := requireTypedStore(store)
	if err != nil {
		return nil, err
	}
	if kind != APIToken && kind != ProviderDiagnostic {
		return nil, fmt.Errorf("unsupported credential kind %d", kind)
	}
	return scopedView{store: scoped, kind: kind}, nil
}

func (view scopedView) Get(account string) (string, error) {
	return view.store.get(view.kind, account)
}

func (view scopedView) Set(account, value string) error {
	return view.store.set(view.kind, account, value)
}

func (view scopedView) Delete(account string) error {
	return view.store.delete(view.kind, account)
}

func (view scopedView) Has(account string) bool {
	_, err := view.Get(account)
	return err == nil
}

func (view scopedView) ReadOnly() bool { return IsReadOnly(view.store) }

func slotName(kind Kind, account string) string {
	if kind == ProviderDiagnostic {
		return "diagnostic@" + account
	}
	return account
}
