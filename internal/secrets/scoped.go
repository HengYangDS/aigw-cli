package secrets

import "fmt"

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
	store credentialBackend
	kind  Kind
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
	return view.store.set(view.kind, account, value)
}

func (view scopedView) Delete(account string) error {
	if err := validate(account, "", false); err != nil {
		return err
	}
	return view.store.delete(view.kind, account)
}

func (view scopedView) Exists(account string) (bool, error) {
	if err := validate(account, "", false); err != nil {
		return false, err
	}
	return view.store.exists(view.kind, account)
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
