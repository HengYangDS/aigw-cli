//go:build linux

package native

import (
	"errors"
	"testing"

	"github.com/godbus/dbus/v5"
)

// A locked collection is refused before any search, even when it would return
// no items, so a listing never reports nothing stored when it cannot tell.
func TestListingRefusesALockedCollectionBeforeItSearches(t *testing.T) {
	searched := false
	_, err := listAccounts(secretServiceListing{
		requireUnlocked: func() error { return ErrUnavailable },
		search: func(string) ([]dbus.ObjectPath, error) {
			searched = true
			return nil, nil
		},
	}, "AIGW_TOKEN")
	if !errors.Is(err, ErrUnavailable) || searched {
		t.Fatalf("a locked collection = %v, searched %t", err, searched)
	}
}

// An unlocked collection names each item's account, and one without items
// names none.
func TestListingNamesTheAccountsOfAnUnlockedCollection(t *testing.T) {
	unlocked := func() error { return nil }
	output, err := listAccounts(secretServiceListing{
		requireUnlocked: unlocked,
		search:          func(string) ([]dbus.ObjectPath, error) { return []dbus.ObjectPath{"/one", "/two"}, nil },
		attributes: func(item dbus.ObjectPath) (dbus.Variant, error) {
			return dbus.MakeVariant(map[string]string{"username": string(item)[1:]}), nil
		},
	}, "AIGW_TOKEN")
	if err != nil || string(output) != "one\ntwo" {
		t.Fatalf("an unlocked collection = %q, %v", output, err)
	}
	output, err = listAccounts(secretServiceListing{
		requireUnlocked: unlocked,
		search:          func(string) ([]dbus.ObjectPath, error) { return nil, nil },
	}, "AIGW_TOKEN")
	if err != nil || output != nil {
		t.Fatalf("an empty collection = %q, %v", output, err)
	}
}
