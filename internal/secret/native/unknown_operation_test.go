package native

import (
	"errors"
	"testing"
)

// Ported from the maintenance branch (fdae584b): an unknown credential
// operation is refused on every platform before it reaches a native store,
// and it returns no output.
func TestProviderRejectsUnknownCredentialOperation(t *testing.T) {
	value, err := queryCredential("unknown", "aigw-private-test", "synthetic", nil, InstalledProgram)
	if value != nil || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unknown credential operation returned output: %q, %v", value, err)
	}
}
