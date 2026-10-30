package synchronization

import (
	"fmt"
	"strings"
	"testing"

	"aigw-cli/internal/clients"
)

// builtinClients is the composition root's adapter set, which tests that
// exercise real adapters name explicitly.
var builtinClients = clients.Registry()

// Client behavior comes only from the registry that the composition root
// supplies; a synchronizer without one refuses instead of guessing clients.
func TestSynchronizerWithoutRegistryRefusesToGuessClients(t *testing.T) {
	defer func() {
		recovered := recover()
		if recovered == nil || !strings.Contains(fmt.Sprint(recovered), "client registry") {
			t.Fatalf("recovered = %v, want the missing client registry refusal", recovered)
		}
	}()
	Synchronizer{}.ClientIDs()
}
