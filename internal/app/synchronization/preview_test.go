package synchronization

import (
	"errors"
	"slices"
	"testing"
)

// presenceStore answers whether an Account's Token exists, records each
// Account that it was asked about, and fails every Token read and write.
type presenceStore struct {
	present map[string]bool
	asked   []string
}

func (s *presenceStore) Exists(account string) (bool, error) {
	s.asked = append(s.asked, account)
	return s.present[account], nil
}

func (*presenceStore) Get(string) (string, error) { return "", errors.New("read a Token") }

func (*presenceStore) Set(string, string) error { return errors.New("wrote a Token") }

func (*presenceStore) Delete(string) error { return errors.New("deleted a Token") }

// TestPresumedTokensAskOnlyOtherAccountsPresence shows that a preview presumes
// the new Accounts' Tokens without the store, asks the store only whether
// other Accounts' Tokens exist, and refuses every Token read and write.
func TestPresumedTokensAskOnlyOtherAccountsPresence(t *testing.T) {
	store := &presenceStore{present: map[string]bool{"stored": true}}
	presumed := presumedTokens{accounts: []string{"new"}, store: store}
	for account, want := range map[string]bool{"new": true, "stored": true, "missing": false} {
		if got, err := presumed.Exists(account); err != nil || got != want {
			t.Fatalf("Exists(%s) = %v, %v; want %v", account, got, err, want)
		}
	}
	slices.Sort(store.asked)
	if !slices.Equal(store.asked, []string{"missing", "stored"}) {
		t.Fatalf("the store was asked about %q", store.asked)
	}
	if _, err := presumed.Get("stored"); !errors.Is(err, errPresumedToken) {
		t.Fatalf("Get = %v", err)
	}
	if err := presumed.Set("new", "token"); !errors.Is(err, errPresumedToken) {
		t.Fatalf("Set = %v", err)
	}
	if err := presumed.Delete("stored"); !errors.Is(err, errPresumedToken) {
		t.Fatalf("Delete = %v", err)
	}
	if got, err := (presumedTokens{accounts: []string{"new"}}).Exists("other"); err != nil || got {
		t.Fatalf("without a store, Exists(other) = %v, %v", got, err)
	}
}
