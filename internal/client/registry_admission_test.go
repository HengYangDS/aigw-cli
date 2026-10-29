package client

import (
	"slices"
	"strings"
	"testing"
)

func TestRegistryRejectsInvalidAdapterSets(t *testing.T) {
	tests := []struct {
		name     string
		adapters []Adapter
		want     string
	}{
		{name: "nil adapter", adapters: []Adapter{nil}, want: "adapter is nil"},
		{name: "empty adapter ID", adapters: []Adapter{failingProjectionAdapter{}}, want: "ID is empty"},
		{
			name: "duplicate adapter",
			adapters: []Adapter{
				failingProjectionAdapter{id: "duplicate"},
				failingProjectionAdapter{id: "duplicate"},
			},
			want: "registered more than once",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewRegistry(test.adapters...); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("NewRegistry() error = %v, want %q", err, test.want)
			}
		})
	}
}

// The registration order is the admission order of every configuration that
// the registry serves.
func TestRegistryAdmitsItsAdaptersInRegistrationOrder(t *testing.T) {
	registry, err := NewRegistry(failingProjectionAdapter{id: "second"}, failingProjectionAdapter{id: "first"})
	if err != nil {
		t.Fatal(err)
	}
	if got := registry.Admission().IDs(); !slices.Equal(got, []string{"second", "first"}) || !slices.Equal(registry.IDs(), got) {
		t.Fatalf("admission = %v, registry = %v", got, registry.IDs())
	}
}
