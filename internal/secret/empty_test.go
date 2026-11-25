package secret

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"aigw-cli/internal/secret/native"
)

// An empty Token file holds no Token: Get reports ErrEmpty, which is not an
// absent slot, and tokenSlotEmpty says so from its size alone. Exists keeps
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
	if empty, err := tokenSlotEmpty(store, "team"); !empty || err != nil {
		t.Fatalf("tokenSlotEmpty() of the empty file = %t, %v", empty, err)
	}
	if err := store.Set("other", "sk-other-0001"); err != nil {
		t.Fatal(err)
	}
	for _, account := range []string{"other", "absent"} {
		if empty, err := tokenSlotEmpty(store, account); empty || err != nil {
			t.Fatalf("tokenSlotEmpty() of a stored or an absent Token = %t, %v", empty, err)
		}
	}
}

// A backend whose metadata cannot tell and whose slots are never empty, such as
// the in-memory one, reports no empty slot.
func TestTokenSlotEmptyOfABackendThatCannotTell(t *testing.T) {
	if empty, err := tokenSlotEmpty(NewMemoryStore(), "team"); empty || err != nil {
		t.Fatalf("a backend without that metadata reported an empty slot: %t, %v", empty, err)
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

// Every writable native backend, macOS Keychain and Secret Service alike,
// reaches the system keyring store, whose metadata
// query cannot tell an empty item: tokenSlotEmpty asks its reader, which
// returns only whether the item is empty. An absent item, a stored Token and
// a failed worker are not empty. An item that macOS refuses AIGW while its
// keychain is unlocked is the one observation that fails, with that item's
// refusal: the item is there, and it is no Token that AIGW can use. A locked
// keychain answers with the same status and stays a failed worker. Within one
// availability scope the worker runs once per Token slot.
func TestTokenSlotEmptyOfTheSystemKeyring(t *testing.T) {
	for _, test := range []struct {
		name    string
		value   string
		err     error
		want    bool
		refused bool
	}{
		{name: "an empty item", want: true},
		{name: "a stored Token", value: "sk-team-0001"},
		{name: "an absent item", err: native.ErrNotFound},
		{name: "a failed worker", err: errors.New("keychain locked")},
		{name: "a locked keychain", err: &native.KeychainError{Status: -25293}},
		{name: "an item that macOS refuses", err: &native.KeychainError{Status: -25293, Unlocked: true}, refused: true},
	} {
		store := scopedView{store: keyringStore{empty: func(string, string) (bool, error) { return test.value == "", test.err }}}
		got, err := tokenSlotEmpty(store, "team")
		refusal, refused := errors.AsType[*KeychainItemRefusedError](err)
		if got != test.want || refused != test.refused || (err != nil) != test.refused {
			t.Errorf("%s: tokenSlotEmpty() = %t, %v, want %t and a refusal %t", test.name, got, err, test.want, test.refused)
		}
		if refused && *refusal != (KeychainItemRefusedError{Kind: APIToken, Provider: "team", Status: -25293}) {
			t.Errorf("%s: refusal = %+v", test.name, *refusal)
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
	if empty, err := tokenSlotEmpty(store, "team"); empty || err != nil {
		t.Fatalf("an empty variable counts as an empty slot, not as unset: %t, %v", empty, err)
	}
}

// One operation forgets what it observed of a slot that it then writes or
// deletes: whether the slot is empty, as whether it is present. A command that
// stores a Token in a slot that it found empty would otherwise go on to report
// the provider as one without a Token.
func TestAScopeForgetsTheEmptinessOfASlotThatItChanges(t *testing.T) {
	root := filepath.Join(t.TempDir(), "secrets")
	store := newFileStore(root)
	empty := func() {
		t.Helper()
		if err := store.Set("team", "sk-team-0001"); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "team"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for name, change := range map[string]func(Store) error{
		"a write":    func(scoped Store) error { return scoped.Set("team", "sk-team-0002") },
		"a deletion": func(scoped Store) error { return scoped.Delete("team") },
	} {
		empty()
		scoped := ObserveAvailability(store)
		if observed, err := tokenSlotEmpty(scoped, "team"); !observed || err != nil {
			t.Fatalf("%s: tokenSlotEmpty() of the empty file = %t, %v", name, observed, err)
		}
		if err := change(scoped); err != nil {
			t.Fatal(err)
		}
		if observed, err := tokenSlotEmpty(scoped, "team"); observed || err != nil {
			t.Errorf("%s: tokenSlotEmpty() in the same scope afterwards = %t, %v, want a slot that is not empty", name, observed, err)
		}
	}
}

// One operation asks the worker once per slot, as it observes presence once,
// and keeps what the worker answered: empty or not, or the refusal of the
// slot's item.
func TestTokenSlotEmptyAsksTheWorkerOncePerScope(t *testing.T) {
	for _, test := range []struct {
		name    string
		err     error
		refused bool
	}{
		{name: "an empty item"},
		{name: "an item that macOS refuses", err: &native.KeychainError{Status: -25293, Unlocked: true}, refused: true},
	} {
		asked := 0
		view := scopedView{store: keyringStore{empty: func(string, string) (bool, error) {
			asked++
			return test.err == nil, test.err
		}}}
		scoped := ObserveAvailability(view)
		for range 3 {
			empty, err := tokenSlotEmpty(scoped, "team")
			if _, refused := errors.AsType[*KeychainItemRefusedError](err); empty == test.refused || refused != test.refused {
				t.Fatalf("%s: tokenSlotEmpty() in a scope = %t, %v", test.name, empty, err)
			}
		}
		if asked != 1 {
			t.Fatalf("%s: the worker ran %d times in one scope, want once", test.name, asked)
		}
		for range 2 {
			if _, err := tokenSlotEmpty(view, "team"); (err != nil) != test.refused {
				t.Fatalf("%s: tokenSlotEmpty() outside a scope = %v", test.name, err)
			}
		}
		if asked != 3 {
			t.Fatalf("%s: outside a scope the worker ran %d times in all, want 3", test.name, asked)
		}
	}
}
