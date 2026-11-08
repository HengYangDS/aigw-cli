package secret

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// An empty Token file holds no Token: Get reports ErrEmpty, which is not an
// absent slot, and TokenSlotEmpty says so from its size alone. Exists keeps
// reporting the slot, which a removal still deletes.
func TestFileStoreReportsAnEmptyToken(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	store := newFileStore(root)
	if err := store.Set("team", "sk-team-0001"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "team"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("team"); !errors.Is(err, ErrEmpty) || errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() of an empty Token = %v, want ErrEmpty", err)
	}
	if present, err := store.Exists("team"); err != nil || !present {
		t.Fatalf("Exists() of an empty Token = %t, %v, want its slot present", present, err)
	}
	if !TokenSlotEmpty(store, "team") {
		t.Fatal("TokenSlotEmpty() did not see the empty file")
	}
	if err := store.Set("other", "sk-other-0001"); err != nil {
		t.Fatal(err)
	}
	if TokenSlotEmpty(store, "other") || TokenSlotEmpty(store, "absent") {
		t.Fatal("TokenSlotEmpty() reported a stored or an absent Token")
	}
}

// A backend whose metadata cannot tell, such as a system keyring, reports no
// empty slot, and its Get reports one when a command reads it.
func TestTokenSlotEmptyOfABackendThatCannotTell(t *testing.T) {
	if TokenSlotEmpty(NewMemoryStore(), "team") {
		t.Fatal("a backend without that metadata reported an empty slot")
	}
}

// The system keyring reports an empty item as ErrEmpty too; its metadata
// query cannot tell an empty item, so its Get does.
func TestKeyringReportsAnEmptyTokenAsEmpty(t *testing.T) {
	store := scopedView{store: keyringStore{read: func(string, string) (string, error) { return "", nil }}}
	if _, err := store.Get("team"); !errors.Is(err, ErrEmpty) {
		t.Fatalf("Get() of an empty keyring item = %v, want ErrEmpty", err)
	}
}

// A Token replaces an empty one, as it replaces an absent one, and undoing
// the replacement removes the new Token rather than writing an empty one,
// which Set refuses.
func TestReplaceTakesTheSlotOfAnEmptyToken(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	store := newFileStore(root)
	if err := store.Set("team", "sk-team-0001"); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(filepath.Join(root, "team"), 0); err != nil {
		t.Fatal(err)
	}
	undo, err := Replace(store, map[string]string{"team": "sk-team-0002"})
	if err != nil {
		t.Fatalf("Replace() over an empty Token = %v", err)
	}
	if token, err := store.Get("team"); err != nil || token != "sk-team-0002" {
		t.Fatalf("the new Token = %q, %v", token, err)
	}
	if err := undo(); err != nil {
		t.Fatalf("undo = %v", err)
	}
	if _, err := store.Get("team"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after undo the slot = %v, want it absent", err)
	}
}
