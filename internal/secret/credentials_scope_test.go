package secret

import (
	"errors"
	"path/filepath"
	"runtime"
	"testing"

	keyring "github.com/zalando/go-keyring"
)

// countingKeyring is a system keyring that counts each question of whether an
// item is there, by item, and holds what is written to it.
func countingKeyring(asked map[string]int) Store {
	held := map[string]string{}
	return scopedView{store: keyringStore{
		observe: func(_, slot string) (bool, error) {
			asked[slot]++
			_, present := held[slot]
			return present, nil
		},
		read: func(_, slot string) (string, error) {
			value, present := held[slot]
			if !present {
				return "", keyring.ErrNotFound
			}
			return value, nil
		},
		write:  func(_, slot, value string) error { held[slot] = value; return nil },
		remove: func(_, slot string) error { delete(held, slot); return nil },
	}}
}

// One operation observes a provider's Token and its balance credentials
// through one scope: each is asked of the backend once, by whichever view. A
// write or a removal of balance credentials ends the scope's reading of their
// slot, as a Token's does, and of no other slot. The stores that the scope was
// opened over stay as they were, and the next operation asks again.
func TestObserveCredentialsSharesOneScopeBetweenTokensAndBalanceCredentials(t *testing.T) {
	asked := map[string]int{}
	backend := countingKeyring(asked)
	diagnostics, err := NewDiagnosticCredentialStore(backend)
	if err != nil {
		t.Fatal(err)
	}
	tokens, balance := ObserveCredentials(backend, diagnostics)
	observe := func(view interface{ Exists(string) (bool, error) }, want bool) {
		t.Helper()
		for range 2 {
			if present, err := view.Exists("team"); err != nil || present != want {
				t.Fatalf("Exists(team) = %t, %v; want %t", present, err, want)
			}
		}
	}
	requireAsked := func(token, credentials int) {
		t.Helper()
		if asked["team"] != token || asked["diagnostic@team"] != credentials {
			t.Fatalf("the backend was asked %d times about the Token and %d times about the balance credentials; want %d and %d",
				asked["team"], asked["diagnostic@team"], token, credentials)
		}
	}
	observe(tokens, false)
	observe(balance, false)
	requireAsked(1, 1)

	if err := balance.Set("team", DiagnosticCredential{SystemToken: "system-token", UserID: "42"}); err != nil {
		t.Fatal(err)
	}
	observe(balance, true)
	observe(tokens, false)
	requireAsked(1, 2)

	if err := balance.Delete("team"); err != nil {
		t.Fatal(err)
	}
	observe(balance, false)
	requireAsked(1, 3)

	if err := tokens.Set("team", "synthetic-token"); err != nil {
		t.Fatal(err)
	}
	observe(tokens, true)
	observe(balance, false)
	requireAsked(2, 3)

	// The stores of the installation carry no scope.
	observe(backend, true)
	observe(diagnostics, false)
	requireAsked(4, 5)

	tokens, balance = ObserveCredentials(backend, diagnostics)
	observe(tokens, true)
	observe(balance, false)
	requireAsked(5, 6)
}

// The scope forgets balance credentials that the operation stores or removes
// on a backend that was named and on the one that AIGW selects itself, whose
// writes go on to the selected backend: no write of the balance view leaves
// the scope, as the undo of a stored Token once did.
func TestObserveCredentialsForgetsBalanceCredentialsOnEveryBackend(t *testing.T) {
	for _, backend := range []string{"file", ""} {
		t.Run("backend="+backend, func(t *testing.T) {
			selected, err := Select(Selection{
				Backend: backend, GOOS: runtime.GOOS, Root: filepath.Join(t.TempDir(), "secrets"),
				KeyringProbe: func(Store) error { return errors.New("isolated file backend") },
			})
			if err != nil {
				t.Fatal(err)
			}
			diagnostics, err := NewDiagnosticCredentialStore(selected)
			if err != nil {
				t.Fatal(err)
			}
			_, balance := ObserveCredentials(selected, diagnostics)
			requirePresent := func(want bool, after string) {
				t.Helper()
				if present, err := balance.Exists("team"); err != nil || present != want {
					t.Fatalf("balance credentials %s: present %t, %v; want %t", after, present, err, want)
				}
			}
			requirePresent(false, "of an empty store")
			if err := balance.Set("team", DiagnosticCredential{SystemToken: "system-token", UserID: "42"}); err != nil {
				t.Fatal(err)
			}
			requirePresent(true, "that the operation stored")
			if err := balance.Delete("team"); err != nil {
				t.Fatal(err)
			}
			requirePresent(false, "that the operation removed")
		})
	}
}

// balanceDouble is a balance credential store that is not AIGW's own.
type balanceDouble struct{ DiagnosticCredentialStore }

// A store that is not AIGW's typed one returns unchanged, whichever of the
// two it is, and the other one still gets the scope.
func TestObserveCredentialsRetainsUntypedAndAbsentStores(t *testing.T) {
	if tokens, balance := ObserveCredentials(nil, nil); tokens != nil || balance != nil {
		t.Fatal("absent stores gained a scope")
	}
	asked := map[string]int{}
	backend := countingKeyring(asked)
	diagnostics, err := NewDiagnosticCredentialStore(backend)
	if err != nil {
		t.Fatal(err)
	}
	double, doubleBalance := &faultStore{Store: backend}, &balanceDouble{DiagnosticCredentialStore: diagnostics}
	if tokens, balance := ObserveCredentials(double, doubleBalance); tokens != double || balance != doubleBalance {
		t.Fatal("an untyped store was wrapped or replaced")
	}

	tokens, balance := ObserveCredentials(backend, doubleBalance)
	if balance != doubleBalance {
		t.Fatal("an untyped balance credential store was wrapped or replaced")
	}
	for range 2 {
		if present, err := tokens.Exists("team"); err != nil || present {
			t.Fatalf("Exists(team) = %t, %v", present, err)
		}
	}
	if asked["team"] != 1 {
		t.Fatalf("a typed Token store beside an untyped one asked %d times; want once", asked["team"])
	}

	tokens, balance = ObserveCredentials(double, diagnostics)
	if tokens != double {
		t.Fatal("an untyped Token store was wrapped or replaced")
	}
	for range 2 {
		if present, err := balance.Exists("team"); err != nil || present {
			t.Fatalf("Exists(team) = %t, %v", present, err)
		}
	}
	if asked["diagnostic@team"] != 1 {
		t.Fatalf("a typed balance credential store beside an untyped one asked %d times; want once", asked["diagnostic@team"])
	}
	// Its failure to tell is kept for the operation too, and is no absence.
	failing := scopedView{store: keyringStore{observe: func(string, string) (bool, error) {
		asked["failing"]++
		return false, errors.New("the keychain is locked")
	}}}
	failingDiagnostics, err := NewDiagnosticCredentialStore(failing)
	if err != nil {
		t.Fatal(err)
	}
	_, balance = ObserveCredentials(failing, failingDiagnostics)
	for range 2 {
		if present, err := balance.Exists("team"); err == nil || present {
			t.Fatalf("Exists(team) over a backend that cannot tell = %t, %v", present, err)
		}
	}
	if asked["failing"] != 1 {
		t.Fatalf("a backend that cannot tell was asked %d times; want once", asked["failing"])
	}
}
