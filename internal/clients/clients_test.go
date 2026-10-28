package clients

import (
	"slices"
	"testing"
)

// The built-in set registers every admitted client's adapter in admission
// order; no package below the composition root holds a default registry.
func TestRegistryRegistersEveryAdmittedClientInOrder(t *testing.T) {
	want := []string{"claude", "claude-desktop", "codex", "hermes"}
	if got := Registry().IDs(); !slices.Equal(got, want) {
		t.Fatalf("registered clients = %q, want %q", got, want)
	}
}
