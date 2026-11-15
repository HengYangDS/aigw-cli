package secret

import (
	"errors"
	"testing"

	"aigw-cli/internal/secret/native"
)

// ObserveToken tells every state of a Token slot from the backend's two
// answers, presence and emptiness, and never asks about the emptiness of a
// slot that is not there. A slot is usable only when it is present, holds a
// value and is no Keychain item that macOS refuses AIGW. A failed emptiness
// answer that is no such refusal, a locked keychain's among them, leaves the
// slot as presence reported it. A store that cannot tell presence fails as
// its Exists fails, with the same error.
func TestObserveTokenTellsEachStateOfASlot(t *testing.T) {
	unavailable := errors.New("the store is locked")
	for _, test := range []struct {
		name      string
		present   bool
		presence  error
		empty     bool
		emptiness error
		want      TokenObservation
		refused   bool
		usable    bool
		// asked is how often the backend is asked whether the slot is empty.
		asked int
	}{
		{name: "an absent slot"},
		{name: "an empty slot", present: true, empty: true, want: TokenObservation{Present: true, Empty: true}, asked: 1},
		{name: "an item that macOS refuses", present: true, emptiness: &native.KeychainError{Status: -25293, Unlocked: true},
			want: TokenObservation{Present: true}, refused: true, asked: 1},
		{name: "a stored Token", present: true, want: TokenObservation{Present: true}, usable: true, asked: 1},
		{name: "a locked keychain", present: true, emptiness: &native.KeychainError{Status: -25293},
			want: TokenObservation{Present: true}, usable: true, asked: 1},
		{name: "a store that cannot tell presence", presence: unavailable},
	} {
		asked := 0
		store := scopedView{store: keyringStore{
			observe: func(string, string) (bool, error) { return test.present, test.presence },
			empty: func(string, string) (bool, error) {
				asked++
				return test.empty, test.emptiness
			},
		}}
		observed, err := ObserveToken(store, "team")
		_, wantErr := store.Exists("team")
		if (err == nil) != (wantErr == nil) || (err != nil && (err.Error() != wantErr.Error() || !errors.Is(err, unavailable))) {
			t.Errorf("%s: ObserveToken() failed with %v; Exists() fails with %v", test.name, err, wantErr)
		}
		if (observed.Refusal != nil) != test.refused || observed.Present != test.want.Present || observed.Empty != test.want.Empty ||
			observed.Usable() != test.usable || asked != test.asked {
			t.Errorf("%s: ObserveToken() = %+v, usable %t, after %d emptiness answers; want %+v, a refusal %t, usable %t, after %d",
				test.name, observed, observed.Usable(), asked, test.want, test.refused, test.usable, test.asked)
		}
		if test.refused && *observed.Refusal != (KeychainItemRefusedError{Kind: APIToken, Provider: "team", Status: -25293}) {
			t.Errorf("%s: refusal = %+v", test.name, *observed.Refusal)
		}
	}
}

// One operation asks the backend once about each slot, presence and
// emptiness alike, however many of its parts observe the Token.
func TestObserveTokenAsksTheBackendOncePerScope(t *testing.T) {
	asked := map[string]int{}
	view := scopedView{store: keyringStore{
		observe: func(_, slot string) (bool, error) {
			asked["exists "+slot]++
			return true, nil
		},
		empty: func(_, slot string) (bool, error) {
			asked["empty "+slot]++
			return slot == "empty", nil
		},
	}}
	scoped := ObserveAvailability(view)
	for range 3 {
		for account, usable := range map[string]bool{"team": true, "empty": false} {
			if observed, err := ObserveToken(scoped, account); err != nil || observed.Usable() != usable {
				t.Fatalf("ObserveToken(%s) in a scope = %+v, %v", account, observed, err)
			}
		}
	}
	for operation, count := range asked {
		if count != 1 {
			t.Errorf("the backend answered %q %d times in one scope, want once", operation, count)
		}
	}
	if len(asked) != 4 {
		t.Fatalf("the backend answered %v; want presence and emptiness of two slots", asked)
	}
}

// answeringStore stands in front of a store and answers the observation of a
// Token itself, as a preview does for the Token that its command would store.
type answeringStore struct {
	Store
	observation TokenObservation
}

func (store answeringStore) ObserveToken(string) (TokenObservation, error) {
	return store.observation, nil
}

// A store that answers the observation itself is asked nothing else: neither
// its presence nor the backend behind it.
func TestObserveTokenTakesTheAnswerOfAStoreThatObservesItself(t *testing.T) {
	want := TokenObservation{Present: true, Empty: true}
	if observed, err := ObserveToken(answeringStore{observation: want}, "team"); err != nil || observed != want {
		t.Fatalf("ObserveToken() = %+v, %v; want the store's own answer %+v", observed, err, want)
	}
}
