package claudedesktop

import (
	"path/filepath"
	"testing"

	"aigw-cli/internal/client/contract"
	"aigw-cli/internal/configuration"
)

func TestClaudeDesktopAdapterKeepsTheSeamContract(t *testing.T) {
	contract.Run(t, func(t *testing.T) contract.Fixture {
		fixture := newClaudeDesktopFixture(t)
		before := fixture.cfg.Clone()
		delete(before.Clients, configuration.ClientClaudeDesktop)
		// The fixture keeps the library at <home>/Claude-3p/configLibrary.
		home := filepath.Dir(filepath.Dir(fixture.cfg.Clients[configuration.ClientClaudeDesktop].Targets[0]))
		return contract.Fixture{
			Adapter: New(), Home: home, Dependencies: fixture.deps,
			Before: before, After: fixture.cfg, Runtime: fixture.runtime,
		}
	})
}
