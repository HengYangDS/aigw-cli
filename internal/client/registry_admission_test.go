package client

import (
	"strings"
	"testing"

	"aigw-cli/internal/configuration"
)

func TestRegistryRejectsInvalidAdmissionSets(t *testing.T) {
	tests := []struct {
		name     string
		specs    []configuration.ClientSpec
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
		{
			name: "duplicate admission",
			specs: []configuration.ClientSpec{
				failingProjectionAdapter{id: "duplicate"}.Spec(),
				failingProjectionAdapter{id: "duplicate"}.Spec(),
			},
			adapters: []Adapter{
				failingProjectionAdapter{id: "duplicate"},
			},
			want: "declared more than once",
		},
		{name: "missing adapter", specs: []configuration.ClientSpec{{ID: "missing"}}, want: "has no operational adapter"},
		{
			name:     "mismatched admission",
			specs:    []configuration.ClientSpec{{ID: "client", Label: "Expected"}},
			adapters: []Adapter{failingProjectionAdapter{id: "client"}},
			want:     "does not match",
		},
		{
			name:     "unadmitted adapter",
			adapters: []Adapter{failingProjectionAdapter{id: "extra"}},
			want:     "unadmitted adapter",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewRegistry(test.specs, test.adapters...); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("NewRegistry() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestBuiltInRegistryInvariantsPanicOnInvalidDefinitions(t *testing.T) {
	tests := []struct {
		name string
		run  func()
	}{
		{name: "invalid registry", run: func() { mustRegistry(nil, failingProjectionAdapter{id: "unadmitted"}) }},
		{name: "missing client", run: func() { mustClientSpec("missing") }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("invariant violation did not panic")
				}
			}()
			test.run()
		})
	}
}
