package codex

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"aigw-cli/internal/client"
	"aigw-cli/internal/client/contract"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/discovery"
	surfaceidentity "aigw-cli/internal/surface"
)

func TestCodexAdapterKeepsTheSeamContract(t *testing.T) {
	contract.Run(t, func(t *testing.T) contract.Fixture {
		home := t.TempDir()
		target := filepath.Join(home, ".codex", "config.toml")
		executable := filepath.Join(home, "bin", "codex")
		if err := os.MkdirAll(filepath.Dir(executable), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
			t.Fatal(err)
		}
		after := codexConfiguration(target)
		binding := after.Clients[configuration.ClientCodex]
		binding.Executable = executable
		after.Clients[configuration.ClientCodex] = binding
		before := after.Clone()
		delete(before.Clients, configuration.ClientCodex)
		runtime, err := after.ResolveRuntime(configuration.ClientCodex, "")
		if err != nil {
			t.Fatal(err)
		}
		surface := discovery.Surface{ID: string(HomeDefault), Authority: string(surfaceidentity.AuthorityAIGW), ConfigPath: target, AutoManaged: true}
		return contract.Fixture{
			Adapter: New(), Home: home, Before: before, After: after, Runtime: runtime,
			Dependencies: client.Dependencies{
				Discovery:      fixedDiscoverer{result: discovery.Result{Surfaces: []discovery.Surface{surface}}},
				Runner:         &captureAdapterRunner{err: errors.New("contract fixtures run no client")},
				AIGWExecutable: filepath.Join(home, "bin", "aigw"),
			},
		}
	})
}
