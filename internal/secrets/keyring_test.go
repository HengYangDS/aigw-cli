package secrets

import (
	"errors"
	"testing"

	keyring "github.com/zalando/go-keyring"
)

func TestKeyringStoreLifecycleUsesMockedSystemKeyring(t *testing.T) {
	keyring.MockInit()
	store := NewKeyringStore()
	if store.Has("dmx") {
		t.Fatal("new mocked keyring unexpectedly has secret")
	}
	if err := store.Set("dmx", "top-secret-value"); err != nil {
		t.Fatal(err)
	}
	if !store.Has("dmx") {
		t.Fatal("stored secret not found")
	}
	got, err := store.Get("dmx")
	if err != nil || got != "top-secret-value" {
		t.Fatalf("Get = %q, %v", got, err)
	}
	if err := store.Delete("dmx"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("dmx"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing error = %v", err)
	}
	if err := store.Delete("dmx"); err != nil {
		t.Fatalf("deleting an absent secret must not error: %v", err)
	}
}

func TestKeyringStoreRejectsInvalidProfileAndEmptyValue(t *testing.T) {
	keyring.MockInit()
	store := NewKeyringStore()
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
		t.Fatal("empty secret accepted")
	}
}

func TestKeyringStoreTreatsEmptyProviderValueAsNotFound(t *testing.T) {
	keyring.MockInit()
	if err := keyring.Set(Service, "dmx", ""); err != nil {
		t.Fatal(err)
	}
	store := NewKeyringStore()
	if _, err := store.Get("dmx"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("empty provider value error = %v", err)
	}
}

func TestKeyringStoreSurfacesUnderlyingKeyringErrors(t *testing.T) {
	keyring.MockInitWithError(errors.New("keyring unavailable"))
	t.Cleanup(keyring.MockInit)
	store := NewKeyringStore()
	if _, err := store.Get("dmx"); err == nil {
		t.Fatal("Get succeeded despite keyring failure")
	}
	if err := store.Set("dmx", "value"); err == nil {
		t.Fatal("Set succeeded despite keyring failure")
	}
	if err := store.Delete("dmx"); err == nil {
		t.Fatal("Delete succeeded despite keyring failure")
	}
}
