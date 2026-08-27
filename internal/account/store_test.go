package account_test

import (
	"errors"
	"testing"

	"aigw-cli/internal/account"
)

func TestMemoryStoreKeepsAccountProbeCredentialSeparate(t *testing.T) {
	store := account.NewMemoryStore()
	want := account.Credential{SystemToken: "system-secret", UserID: "10000"}
	if store.Has("dmx") {
		t.Fatal("new store unexpectedly has credential")
	}
	if err := store.Set("dmx", want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get("dmx")
	if err != nil || got != want {
		t.Fatalf("Get = %#v, %v", got, err)
	}
	if err := store.Delete("dmx"); err != nil || store.Has("dmx") {
		t.Fatalf("delete = %v, has=%v", err, store.Has("dmx"))
	}
}

type stringMemory struct {
	value     string
	err       error
	setErr    error
	deleteErr error
}

func (store *stringMemory) Get(string) (string, error) { return store.value, store.err }
func (store *stringMemory) Set(_ string, value string) error {
	if store.setErr != nil {
		return store.setErr
	}
	store.value = value
	store.err = nil
	return nil
}
func (store *stringMemory) Delete(string) error {
	if store.deleteErr != nil {
		return store.deleteErr
	}
	store.value = ""
	store.err = errors.New("missing")
	return nil
}

func TestBackendStoreMapsTypedBackendAndDomainErrors(t *testing.T) {
	backendMissing := errors.New("backend missing")
	backend := &stringMemory{err: backendMissing}
	store := account.NewBackendStore(backend, func(err error) bool { return errors.Is(err, backendMissing) })
	if _, err := store.Get("dmx"); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("missing error = %v", err)
	}
	want := account.Credential{SystemToken: "system", UserID: "42"}
	if err := store.Set("dmx", want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get("dmx")
	if err != nil || got != want || !store.Has("dmx") {
		t.Fatalf("credential = %#v, %v, has=%v", got, err, store.Has("dmx"))
	}
	backend.value = "not-json"
	if _, err := store.Get("dmx"); err == nil {
		t.Fatal("malformed credential accepted")
	}
	backend.value = `{"system_token":"","user_id":""}`
	if _, err := store.Get("dmx"); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("incomplete credential error = %v", err)
	}
	if err := store.Set("bad name", want); err == nil {
		t.Fatal("invalid profile accepted")
	}
	for _, incomplete := range []account.Credential{{SystemToken: "system"}, {UserID: "42"}} {
		if err := store.Set("dmx", incomplete); err == nil {
			t.Fatalf("incomplete credential accepted: %#v", incomplete)
		}
	}
	wantBackendErr := errors.New("backend failed")
	backend.err = wantBackendErr
	if _, err := store.Get("dmx"); !errors.Is(err, wantBackendErr) {
		t.Fatalf("backend Get error = %v", err)
	}
	backend.err = nil
	backend.setErr = wantBackendErr
	if err := store.Set("dmx", want); !errors.Is(err, wantBackendErr) {
		t.Fatalf("backend Set error = %v", err)
	}
	backend.setErr = nil
	backend.deleteErr = wantBackendErr
	if err := store.Delete("dmx"); !errors.Is(err, wantBackendErr) {
		t.Fatalf("backend Delete error = %v", err)
	}
}

func TestMemoryStoreGetMissingProfileReturnsNotFound(t *testing.T) {
	store := account.NewMemoryStore()
	if _, err := store.Get("missing"); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("Get error = %v", err)
	}
}

func TestMemoryStoreRejectsInvalidProfileOrIncompleteCredential(t *testing.T) {
	store := account.NewMemoryStore()
	for _, name := range []string{"bad name", "", "../escape"} {
		if err := store.Set(name, account.Credential{SystemToken: "s", UserID: "1"}); err == nil {
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
