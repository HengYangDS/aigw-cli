package domain_test

import (
	"reflect"
	"testing"

	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
)

// Domain's own tests cannot import the adapters, which import domain, so they
// admit a copy of the built-in client records. This external test keeps the
// copy equal to the adapters' specifications in admission order.
func TestFixtureClientsMatchTheBuiltInAdmission(t *testing.T) {
	if got, want := domain.FixtureClientSpecs(), clients.Admission().Specs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("domain fixture clients = %#v, want the built-in admission %#v", got, want)
	}
}
