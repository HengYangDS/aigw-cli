package adapter

import (
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/domain"
)

// specAdapter is a fixture adapter whose admission record a test chooses.
type specAdapter struct {
	failingProjectionAdapter
	spec domain.ClientSpec
}

func (adapter specAdapter) Spec() domain.ClientSpec { return adapter.spec }

func TestRegistryRejectsInvalidAdapterSets(t *testing.T) {
	tests := []struct {
		name     string
		adapters []Adapter
		want     string
	}{
		{name: "nil adapter", adapters: []Adapter{nil}, want: "adapter is nil"},
		{name: "empty adapter ID", adapters: []Adapter{failingProjectionAdapter{}}, want: "has no identifier"},
		{
			name: "duplicate adapter",
			adapters: []Adapter{
				failingProjectionAdapter{id: "duplicate"},
				failingProjectionAdapter{id: "duplicate"},
			},
			want: "registered more than once",
		},
		{
			// The registry returns the specification's defect instead of
			// letting the admission panic on it.
			name:     "native authentication without a model provider",
			adapters: []Adapter{specAdapter{spec: domain.ClientSpec{ID: "native", ClientNativeAuthentication: true}}},
			want:     `client "native" has client-native authentication but no model provider`,
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
