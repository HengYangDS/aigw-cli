package secret

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"aigw-cli/internal/secret/native"
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

// A backend whose metadata cannot tell and whose slots are never empty, such as
// the in-memory one, reports no empty slot.
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

// Every writable native backend, macOS Keychain, Secret Service and
// Credential Manager alike, reaches the system keyring store, whose metadata
// query cannot tell an empty item: TokenSlotEmpty asks its reader, which
// returns only whether the item is empty. An absent item, a stored Token and
// a failed worker are not empty. Within one availability scope the worker runs
// once per Token slot.
func TestTokenSlotEmptyOfTheSystemKeyring(t *testing.T) {
	for _, test := range []struct {
		name  string
		value string
		err   error
		want  bool
	}{
		{"an empty item", "", nil, true},
		{"a stored Token", "sk-team-0001", nil, false},
		{"an absent item", "", native.ErrNotFound, false},
		{"a failed worker", "", errors.New("keychain locked"), false},
	} {
		store := scopedView{store: keyringStore{empty: func(string, string) (bool, error) { return test.value == "", test.err }}}
		if got := TokenSlotEmpty(store, "team"); got != test.want {
			t.Errorf("%s: TokenSlotEmpty() = %t, want %t", test.name, got, test.want)
		}
	}
}

// The read-only environment backend treats an empty variable as unset: its
// slot is neither present nor empty, so no Token is reported absent twice.
func TestTokenSlotEmptyOfTheEnvironmentBackend(t *testing.T) {
	store := scopedView{store: environmentStore{getenv: func(key string) string {
		if key == EnvironmentKey("team") {
			return ""
		}
		return "set"
	}}}
	if present, err := store.Exists("team"); err != nil || present {
		t.Fatalf("Exists() of an empty variable = %t, %v, want unset", present, err)
	}
	if TokenSlotEmpty(store, "team") {
		t.Fatal("an empty variable counts as an empty slot, not as unset")
	}
}

// One operation asks the worker once per slot, as it observes presence once.
func TestTokenSlotEmptyAsksTheWorkerOncePerScope(t *testing.T) {
	asked := 0
	view := scopedView{store: keyringStore{empty: func(string, string) (bool, error) {
		asked++
		return true, nil
	}}}
	scoped := ObserveAvailability(view)
	for range 3 {
		if !TokenSlotEmpty(scoped, "team") {
			t.Fatal("the empty item was not reported")
		}
	}
	if asked != 1 {
		t.Fatalf("the worker ran %d times in one scope, want once", asked)
	}
	TokenSlotEmpty(view, "team")
	TokenSlotEmpty(view, "team")
	if asked != 3 {
		t.Fatalf("outside a scope the worker ran %d times in all, want 3", asked)
	}
}
