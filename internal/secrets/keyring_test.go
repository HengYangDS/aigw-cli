package secrets

import (
	"errors"
	"testing"

	keyring "github.com/zalando/go-keyring"
)

func TestKeyringCredentialKindsShareOneServiceWithoutSharingSlots(t *testing.T) {
	keyring.MockInit()
	store := NewKeyringStore()
	if err := store.set(APIToken, "dmx", "api-token"); err != nil {
		t.Fatal(err)
	}
	if err := store.set(ProviderDiagnostic, "dmx", diagnosticValue); err != nil {
		t.Fatal(err)
	}
	if got, err := keyring.Get(Service, "dmx"); err != nil || got != "api-token" {
		t.Fatalf("API token slot = %q, %v", got, err)
	}
	if got, err := keyring.Get(Service, "diagnostic@dmx"); err != nil || got != diagnosticValue {
		t.Fatalf("diagnostic slot = %q, %v", got, err)
	}
}

func TestKeyringStoreLifecycleAndValidation(t *testing.T) {
	keyring.MockInit()
	store := NewKeyringStore()
	store.observe = mockKeyringObserver
	if mustExist(t, store, "dmx") {
		t.Fatal("new mocked keyring unexpectedly has a token")
	}
	if err := store.Set("dmx", "api-token"); err != nil || !mustExist(t, store, "dmx") {
		t.Fatalf("set token = %v, exists=%v", err, mustExist(t, store, "dmx"))
	}
	if got, err := store.Get("dmx"); err != nil || got != "api-token" {
		t.Fatalf("token = %q, %v", got, err)
	}
	if err := store.Delete("dmx"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("dmx"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing token error = %v", err)
	}
	if err := store.Delete("dmx"); err != nil {
		t.Fatalf("delete absent token = %v", err)
	}
	for _, name := range []string{"bad name", "", "../escape"} {
		if _, err := store.Get(name); err == nil {
			t.Errorf("Get(%q) succeeded", name)
		}
		if err := store.Set(name, "value"); err == nil {
			t.Errorf("Set(%q) succeeded", name)
		}
		if err := store.Delete(name); err == nil {
			t.Errorf("Delete(%q) succeeded", name)
		}
	}
	if err := store.Set("dmx", ""); err == nil {
		t.Fatal("empty token accepted")
	}
}

func TestKeyringStoreMapsEmptyValuesAndProviderErrors(t *testing.T) {
	keyring.MockInit()
	if err := keyring.Set(Service, "dmx", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := NewKeyringStore().Get("dmx"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("empty provider value error = %v", err)
	}
	want := errors.New("keyring unavailable")
	keyring.MockInitWithError(want)
	t.Cleanup(keyring.MockInit)
	store := NewKeyringStore()
	if _, err := store.Get("dmx"); !errors.Is(err, want) {
		t.Fatalf("Get error = %v", err)
	}
	if err := store.Set("dmx", "value"); !errors.Is(err, want) {
		t.Fatalf("Set error = %v", err)
	}
	if err := store.Delete("dmx"); !errors.Is(err, want) {
		t.Fatalf("Delete error = %v", err)
	}
}

type untypedStore struct{}

func (untypedStore) Get(string) (string, error)  { return "", ErrNotFound }
func (untypedStore) Set(string, string) error    { return nil }
func (untypedStore) Delete(string) error         { return nil }
func (untypedStore) Exists(string) (bool, error) { return false, nil }

func mockKeyringObserver(service, slot string) (bool, error) {
	_, err := keyring.Get(service, slot)
	if errors.Is(err, keyring.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

func TestForKindRejectsUntypedStoreAndUnknownKind(t *testing.T) {
	if _, err := ForKind(untypedStore{}, APIToken); err == nil {
		t.Fatal("untyped store accepted")
	}
	if _, err := ForKind(NewMemoryStore(), Kind(255)); err == nil {
		t.Fatal("unknown credential kind accepted")
	}
}
