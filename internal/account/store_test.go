package account_test

import (
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/account"
)

func TestMemoryStoreKeepsAccountProbeCredentialSeparate(t *testing.T) {
	store := account.NewMemoryStore()
	want := account.Credential{SystemToken: "system-secret", UserID: "10000"}
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
