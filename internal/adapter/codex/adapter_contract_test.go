package codex

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/contract"
	"aigw-cli/internal/adapter/discovery"
	surfaceidentity "aigw-cli/internal/adapter/surface"
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
		binding := after.Clients[ID]
		binding.Executable = executable
		after.Clients[ID] = binding
		before := after.Clone()
		delete(before.Clients, ID)
		runtime, err := after.ResolveRuntime(ID, "")
		if err != nil {
			t.Fatal(err)
		}
		surface := discovery.Surface{ID: string(HomeDefault), Authority: string(surfaceidentity.AuthorityAIGW), ConfigPath: target, AutoManaged: true}
		return contract.Fixture{
			Adapter: New(), Home: home, Before: before, After: after, Runtime: runtime,
			Dependencies: adapter.Dependencies{
				Discovery:      fixedDiscoverer{result: discovery.Result{Surfaces: []discovery.Surface{surface}}},
				Runner:         &captureAdapterRunner{err: errors.New("contract fixtures run no client")},
				AIGWExecutable: filepath.Join(home, "bin", "aigw"),
			},
		}
	})
}
