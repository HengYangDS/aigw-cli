package account_test

import (
	"errors"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/account"
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
