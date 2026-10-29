package hermes

import (
	"path/filepath"
	"testing"

	"aigw-cli/internal/client/contract"
)

func TestHermesAdapterKeepsTheSeamContract(t *testing.T) {
	contract.Run(t, func(t *testing.T) contract.Fixture {
		after, runtime, deps := hermesVerificationFixture(t)
		before := after.Clone()
		delete(before.Clients, ID)
		return contract.Fixture{
			Adapter: New(), Home: filepath.Dir(after.Clients[ID].Targets[0]), Dependencies: deps,
			Before: before, After: after, Runtime: runtime,
		}
	})
}
