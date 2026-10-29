package store_test

import (
	"reflect"
	"testing"

	"aigw-cli/internal/clients"
	"aigw-cli/internal/store"
)

// The store's own tests cannot import the adapters, which import the store, so
// they admit a copy of the built-in client records. This external test keeps
// the copy equal to the adapters' specifications in admission order.
func TestFixtureClientsMatchTheBuiltInAdmission(t *testing.T) {
	if got, want := store.FixtureClientSpecs(), clients.Admission().Specs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("store fixture clients = %#v, want the built-in admission %#v", got, want)
	}
}
