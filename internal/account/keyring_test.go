package account_test

import (
	"errors"
	"testing"

	"aigw-cli/internal/account"
	keyring "github.com/zalando/go-keyring"
)

func TestKeyringStoreLifecycleUsesMockedSystemKeyring(t *testing.T) {
	keyring.MockInit()
	store := account.NewKeyringStore()
	want := account.Credential{SystemToken: "system-secret", UserID: "10000"}
	if store.Has("dmx") {
		t.Fatal("new mocked keyring unexpectedly has credential")
	}
	if err := store.Set("dmx", want); err != nil {
		t.Fatal(err)
	}
	if !store.Has("dmx") {
		t.Fatal("stored credential not found")
	}
	got, err := store.Get("dmx")
	if err != nil || got != want {
		t.Fatalf("Get = %#v, %v", got, err)
	}
	if err := store.Delete("dmx"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("dmx"); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("missing error = %v", err)
	}
	if err := store.Delete("dmx"); err != nil {
		t.Fatalf("deleting an absent credential must not error: %v", err)
	}
}

func TestKeyringStoreRejectsInvalidProfileAndIncompleteCredential(t *testing.T) {
	keyring.MockInit()
	store := account.NewKeyringStore()
	want := account.Credential{SystemToken: "system-secret", UserID: "10000"}
	for _, name := range []string{"bad name", "", "../escape"} {
		if _, err := store.Get(name); err == nil {
			t.Errorf("Get(%q) succeeded", name)
		}
		if err := store.Set(name, want); err == nil {
			t.Errorf("Set(%q) succeeded", name)
		}
	}
	if err := store.Set("dmx", account.Credential{UserID: "10000"}); err == nil {
		t.Fatal("credential without system token accepted")
	}
	if err := store.Set("dmx", account.Credential{SystemToken: "system-secret"}); err == nil {
		t.Fatal("credential without user ID accepted")
	}
}

func TestKeyringStoreTreatsIncompleteStoredCredentialAsNotFound(t *testing.T) {
	keyring.MockInit()
	if err := keyring.Set(account.Service, "dmx", `{"system_token":"","user_id":""}`); err != nil {
		t.Fatal(err)
	}
	store := account.NewKeyringStore()
	if _, err := store.Get("dmx"); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("incomplete credential error = %v", err)
	}
}

func TestKeyringStoreSurfacesMalformedStoredCredential(t *testing.T) {
	keyring.MockInit()
	if err := keyring.Set(account.Service, "dmx", "not-json"); err != nil {
		t.Fatal(err)
	}
	store := account.NewKeyringStore()
	if _, err := store.Get("dmx"); err == nil {
		t.Fatal("malformed credential accepted")
	}
}

func TestKeyringStoreSurfacesUnderlyingKeyringErrors(t *testing.T) {
	keyring.MockInitWithError(errors.New("keyring unavailable"))
	t.Cleanup(keyring.MockInit)
	store := account.NewKeyringStore()
	if _, err := store.Get("dmx"); err == nil {
		t.Fatal("Get succeeded despite keyring failure")
	}
	if err := store.Set("dmx", account.Credential{SystemToken: "s", UserID: "1"}); err == nil {
		t.Fatal("Set succeeded despite keyring failure")
	}
}
